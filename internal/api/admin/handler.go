package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/audit"
	"github.com/peizhengma/biography-v2/internal/domain/era"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	"github.com/peizhengma/biography-v2/internal/domain/preset"
	"github.com/peizhengma/biography-v2/internal/domain/quote"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	"github.com/peizhengma/biography-v2/internal/domain/user"
	"github.com/peizhengma/biography-v2/internal/domain/welcome"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
	auditService "github.com/peizhengma/biography-v2/internal/service/audit"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	eraService "github.com/peizhengma/biography-v2/internal/service/era"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	presetService "github.com/peizhengma/biography-v2/internal/service/preset"
	quoteService "github.com/peizhengma/biography-v2/internal/service/quote"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
	welcomeService "github.com/peizhengma/biography-v2/internal/service/welcome"
)

// Handler Admin API 处理器
type Handler struct {
	llmManager     *llm.Manager
	asrProvider    asr.Provider
	ttsProvider    tts.Provider
	userService    *userService.Service
	convService    *convService.Service
	memoirService  *memoirService.Service
	topicService   *topicService.Service
	quoteService   *quoteService.Service
	llmService     *llmService.Service
	eraService     *eraService.Service
	presetService  *presetService.Service
	welcomeService *welcomeService.Service
	auditService   *auditService.Service
}

// NewHandler 创建 Admin Handler
func NewHandler(
	llmManager *llm.Manager,
	asrProvider asr.Provider,
	ttsProvider tts.Provider,
	userSvc *userService.Service,
	convSvc *convService.Service,
	memoirSvc *memoirService.Service,
	topicSvc *topicService.Service,
	quoteSvc *quoteService.Service,
	llmSvc *llmService.Service,
	eraSvc *eraService.Service,
	presetSvc *presetService.Service,
	welcomeSvc *welcomeService.Service,
	auditSvc *auditService.Service,
) *Handler {
	return &Handler{
		llmManager:     llmManager,
		asrProvider:    asrProvider,
		ttsProvider:    ttsProvider,
		userService:    userSvc,
		convService:    convSvc,
		memoirService:  memoirSvc,
		topicService:   topicSvc,
		quoteService:   quoteSvc,
		llmService:     llmSvc,
		eraService:     eraSvc,
		presetService:  presetSvc,
		welcomeService: welcomeSvc,
		auditService:   auditSvc,
	}
}

// --- 用户管理 ---

// CreateUser 创建用户
func (h *Handler) CreateUser(c *gin.Context) {
	var input user.AdminCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fmt.Printf("[CreateUser] JSON bind error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON绑定失败: " + err.Error()})
		return
	}

	fmt.Printf("[CreateUser] Input: phone=%s, nickname=%v, gender=%v, birth_year=%v\n",
		input.Phone, input.Nickname, input.Gender, input.BirthYear)

	u, err := h.userService.AdminCreate(c.Request.Context(), &input)
	if err != nil {
		fmt.Printf("[CreateUser] Service error: %v\n", err)
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrInvalidPhone) ||
			errors.Is(err, userService.ErrInvalidPassword) ||
			errors.Is(err, userService.ErrPhoneAlreadyExists) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("[CreateUser] Success: user_id=%s\n", u.ID)

	// 记录审计日志
	if err := h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionCreateUser,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &u.ID,
		TargetLabel: u.Phone,
		Detail:      map[string]any{"nickname": input.Nickname},
		IPAddress:   c.ClientIP(),
	}); err != nil {
		fmt.Printf("[CreateUser] Audit log error: %v\n", err)
	}

	c.JSON(http.StatusCreated, u)
}

// ListUsers 获取用户列表
func (h *Handler) ListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	users, total, err := h.userService.AdminList(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUser 获取用户详情
func (h *Handler) GetUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUser 更新用户
func (h *Handler) UpdateUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var input struct {
		Nickname         *string `json:"nickname"`
		PreferredName    *string `json:"preferred_name"`
		Gender           *string `json:"gender"`
		BirthYear        *int    `json:"birth_year"`
		Hometown         *string `json:"hometown"`
		MainCity         *string `json:"main_city"`
		ProfileCompleted *bool   `json:"profile_completed"`
		IsActive         *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userService.AdminUpdate(c.Request.Context(), userID, &user.AdminUpdateInput{
		Nickname:         input.Nickname,
		PreferredName:    input.PreferredName,
		Gender:           input.Gender,
		BirthYear:        input.BirthYear,
		Hometown:         input.Hometown,
		MainCity:         input.MainCity,
		ProfileCompleted: input.ProfileCompleted,
		IsActive:         input.IsActive,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionEditUser,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &userID,
		TargetLabel: user.Phone,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, user)
}

// DeleteUser 删除用户（软删除）
func (h *Handler) DeleteUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// 先获取用户信息用于审计日志
	u, _ := h.userService.GetByID(c.Request.Context(), userID)
	targetLabel := ""
	if u != nil {
		targetLabel = u.Phone
	}

	if err := h.userService.AdminDelete(c.Request.Context(), userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	if err := h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionDeleteUser,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &userID,
		TargetLabel: targetLabel,
		IPAddress:   c.ClientIP(),
	}); err != nil {
		fmt.Printf("[DeleteUser] Audit log error: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// ResetPassword 重置用户密码
func (h *Handler) ResetPassword(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var input struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息用于审计日志
	u, _ := h.userService.GetByID(c.Request.Context(), userID)
	targetLabel := ""
	if u != nil {
		targetLabel = u.Phone
	}

	if err := h.userService.AdminResetPassword(c.Request.Context(), userID, input.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionResetPassword,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &userID,
		TargetLabel: targetLabel,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

// ToggleUserActive 切换用户激活状态
func (h *Handler) ToggleUserActive(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// 获取当前用户状态
	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 切换状态
	newActive := !u.IsActive
	u, err = h.userService.AdminUpdate(c.Request.Context(), userID, &user.AdminUpdateInput{
		IsActive: &newActive,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionToggleActive,
		TargetType:  audit.TargetTypeUser,
		TargetID:    &userID,
		TargetLabel: u.Phone,
		Detail:      map[string]any{"is_active": u.IsActive},
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{
		"message":   "user status updated",
		"is_active": u.IsActive,
	})
}

// GetUserStats 获取用户统计信息（包含详细数据）
func (h *Handler) GetUserStats(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	ctx := c.Request.Context()

	// 获取用户信息
	u, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 获取对话列表（包含消息）
	conversations, _, _ := h.convService.AdminList(ctx, &userID, nil, 1000, 0)

	// 获取回忆录列表
	memoirs, _ := h.memoirService.ListByUserID(ctx, userID)

	// 获取话题池
	topicPool, _ := h.topicService.GetTopicOptions(ctx, userID, 100)

	// 计算统计数据
	totalMessages := 0
	totalDurationMins := 0.0
	for _, conv := range conversations {
		totalMessages += conv.MessageCount
		if conv.UpdatedAt.After(conv.CreatedAt) {
			totalDurationMins += conv.UpdatedAt.Sub(conv.CreatedAt).Minutes()
		}
	}

	totalMemoirChars := 0
	lifeStages := make(map[string]int)
	for _, m := range memoirs {
		totalMemoirChars += len(m.Content)
		if m.TimePeriod != nil && *m.TimePeriod != "" {
			lifeStages[*m.TimePeriod]++
		}
	}

	avgDuration := 0.0
	avgMessages := 0.0
	convToMemoirRate := 0.0
	avgMemoirLen := 0
	if len(conversations) > 0 {
		avgDuration = totalDurationMins / float64(len(conversations))
		avgMessages = float64(totalMessages) / float64(len(conversations))
		convToMemoirRate = float64(len(memoirs)) / float64(len(conversations))
	}
	if len(memoirs) > 0 {
		avgMemoirLen = totalMemoirChars / len(memoirs)
	}

	// 返回扁平化的结构（前端期望的格式）
	result := gin.H{
		"id":                u.ID,
		"phone":             u.Phone,
		"nickname":          u.Nickname,
		"preferred_name":    u.PreferredName,
		"gender":            u.Gender,
		"birth_year":        u.BirthYear,
		"hometown":          u.Hometown,
		"main_city":         u.MainCity,
		"profile_completed": u.ProfileCompleted,
		"is_active":         u.IsActive,
		"era_memories":      u.EraMemories,
		"created_at":        u.CreatedAt,
		"updated_at":        u.UpdatedAt,
		"conversations":     conversations,
		"memoirs":           memoirs,
		"topic_pool":        topicPool,
		"stats": gin.H{
			"total_conversations":            len(conversations),
			"total_memoirs":                  len(memoirs),
			"total_messages":                 totalMessages,
			"total_duration_mins":            totalDurationMins,
			"total_memoir_chars":             totalMemoirChars,
			"avg_conversation_duration_mins": avgDuration,
			"avg_messages_per_conversation":  avgMessages,
			"conversation_to_memoir_rate":    convToMemoirRate,
			"avg_memoir_length":              avgMemoirLen,
			"life_stages_coverage":           lifeStages,
		},
	}

	c.JSON(http.StatusOK, result)
}

// --- 对话管理 ---

// ListConversations 获取对话列表
func (h *Handler) ListConversations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// 可选的用户过滤
	var userID *uuid.UUID
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		userID = &id
	}

	conversations, total, err := h.convService.AdminList(c.Request.Context(), userID, nil, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetConversation 获取对话详情
func (h *Handler) GetConversation(c *gin.Context) {
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	conv, err := h.convService.AdminGetWithMessages(c.Request.Context(), convID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, convService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// --- 回忆录管理 ---

// ListMemoirs 获取回忆录列表
func (h *Handler) ListMemoirs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	includeDeleted := c.Query("include_deleted") == "true"

	memoirs, total, err := h.memoirService.AdminList(c.Request.Context(), limit, offset, includeDeleted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"memoirs": memoirs,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// UpdateMemoir 更新回忆录
func (h *Handler) UpdateMemoir(c *gin.Context) {
	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	var input memoir.UpdateMemoirInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.memoirService.AdminUpdate(c.Request.Context(), memoirID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, m)
}

// DeleteMemoir 删除回忆录
func (h *Handler) DeleteMemoir(c *gin.Context) {
	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	if err := h.memoirService.AdminDelete(c.Request.Context(), memoirID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "memoir deleted"})
}

// RegenerateMemoir 重新生成回忆录
func (h *Handler) RegenerateMemoir(c *gin.Context) {
	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	ctx := c.Request.Context()

	// 获取回忆录
	m, err := h.memoirService.AdminGetByID(ctx, memoirID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 必须有关联的对话
	if m.ConversationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memoir has no associated conversation"})
		return
	}

	// 获取对话及消息
	conv, err := h.convService.AdminGetWithMessages(ctx, *m.ConversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get conversation: " + err.Error()})
		return
	}

	// 获取用户信息
	user, err := h.userService.GetByID(ctx, m.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user: " + err.Error()})
		return
	}

	// 构建对话文本
	var sb strings.Builder
	for _, msg := range conv.Messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		sb.WriteString(fmt.Sprintf("%s：%s\n\n", role, msg.Content))
	}
	conversationText := sb.String()

	// 调用 LLM 重新生成
	result, err := h.llmService.GenerateMemoir(ctx, &llmService.GenerateMemoirInput{
		UserName:     user.DisplayName(),
		BirthYear:    user.BirthYear,
		Hometown:     derefString(user.Hometown),
		Topic:        derefString(conv.Topic),
		Conversation: conversationText,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate memoir: " + err.Error()})
		return
	}

	// 更新回忆录
	updatedMemoir, err := h.memoirService.AdminUpdate(ctx, memoirID, &memoir.UpdateMemoirInput{
		Title:      &result.Title,
		Content:    &result.Content,
		TimePeriod: &result.TimePeriod,
		StartYear:  result.StartYear,
		EndYear:    result.EndYear,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update memoir: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedMemoir)
}

// derefString 安全解引用字符串指针
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- 话题管理 ---

// ListTopics 获取话题列表
func (h *Handler) ListTopics(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// 可选的用户过滤
	var userID *uuid.UUID
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		userID = &id
	}

	// 可选的状态过滤
	var status *topic.Status
	if s := c.Query("status"); s != "" {
		st := topic.Status(s)
		status = &st
	}

	topics, total, err := h.topicService.AdminList(c.Request.Context(), userID, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topics": topics,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// CreateTopic 创建话题
func (h *Handler) CreateTopic(c *gin.Context) {
	var input struct {
		UserID   string  `json:"user_id" binding:"required"`
		Title    string  `json:"title" binding:"required"`
		Greeting string  `json:"greeting"`
		Context  string  `json:"context"`
		Status   *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := uuid.Parse(input.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	topicInput := &topic.CreateTopicInput{
		Title:    input.Title,
		Greeting: input.Greeting,
		Context:  input.Context,
		Source:   topic.SourceManual,
	}
	if input.Status != nil {
		st := topic.Status(*input.Status)
		topicInput.Status = &st
	}

	t, err := h.topicService.AdminCreate(c.Request.Context(), userID, topicInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, t)
}

// UpdateTopic 更新话题
func (h *Handler) UpdateTopic(c *gin.Context) {
	topicID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic id"})
		return
	}

	var input topic.UpdateTopicInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.topicService.AdminUpdate(c.Request.Context(), topicID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, topicService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, t)
}

// DeleteTopic 删除话题
func (h *Handler) DeleteTopic(c *gin.Context) {
	topicID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic id"})
		return
	}

	if err := h.topicService.AdminDelete(c.Request.Context(), topicID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, topicService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "topic deleted"})
}

// --- 激励语管理 ---

// ListQuotes 获取激励语列表
func (h *Handler) ListQuotes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 可选的类型过滤
	var quoteType *quote.Type
	if t := c.Query("type"); t != "" {
		qt := quote.Type(t)
		quoteType = &qt
	}

	// 是否只返回激活的
	activeOnly := c.Query("active_only") == "true"

	quotes, total, err := h.quoteService.List(c.Request.Context(), quoteType, activeOnly, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"quotes":    quotes,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateQuote 创建激励语
func (h *Handler) CreateQuote(c *gin.Context) {
	var input quote.CreateQuoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q, err := h.quoteService.Create(c.Request.Context(), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, q)
}

// UpdateQuote 更新激励语
func (h *Handler) UpdateQuote(c *gin.Context) {
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	var input quote.UpdateQuoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q, err := h.quoteService.Update(c.Request.Context(), quoteID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, quoteService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, q)
}

// DeleteQuote 删除激励语
func (h *Handler) DeleteQuote(c *gin.Context) {
	quoteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	if err := h.quoteService.Delete(c.Request.Context(), quoteID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, quoteService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "quote deleted"})
}

// --- 系统监控 ---

// ProviderStatus 单个 Provider 状态
type ProviderStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok, error, unavailable
	Latency int64  `json:"latency_ms,omitempty"`
	Error   string `json:"error,omitempty"`
}

// APIItem 外部 API 项
type APIItem struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Category         string `json:"category"` // llm, asr, tts
	Provider         string `json:"provider"`
	ModelName        string `json:"model_name,omitempty"`
	IsPrimary        bool   `json:"is_primary,omitempty"`
	Status           string `json:"status"` // ok, error, unavailable
	InternalEndpoint string `json:"internal_endpoint"`
	UpstreamEndpoint string `json:"upstream_endpoint,omitempty"`
	LatencyMS        int64  `json:"latency_ms"`
	Error            string `json:"error,omitempty"`
	RawRequestBody   string `json:"raw_request_body,omitempty"`
	RawResponseBody  string `json:"raw_response_body,omitempty"`
	RawStatusCode    int    `json:"raw_status_code,omitempty"`
}

type upstreamEndpointProvider interface {
	UpstreamEndpoint() string
}

type modelNameProvider interface {
	ModelName() string
}

type rawLLMRequestProvider interface {
	RawGenerate(ctx context.Context, prompt string) (string, string, int, error)
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Status    string                    `json:"status"` // healthy, degraded, unhealthy
	Timestamp string                    `json:"timestamp"`
	Providers map[string]ProviderStatus `json:"providers"`
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	response := HealthCheckResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Providers: make(map[string]ProviderStatus),
	}

	// 检查 LLM Providers
	if h.llmManager != nil {
		llmResults := h.llmManager.HealthCheck(ctx)
		for name, err := range llmResults {
			status := ProviderStatus{
				Name:   name,
				Status: "ok",
			}
			if err != nil {
				status.Status = "error"
				status.Error = err.Error()
				response.Status = "degraded"
			}
			response.Providers["llm_"+name] = status
		}
	}

	// 检查 ASR Provider
	if h.asrProvider != nil {
		start := time.Now()
		err := h.asrProvider.HealthCheck(ctx)
		latency := time.Since(start).Milliseconds()
		status := ProviderStatus{
			Name:    h.asrProvider.Name(),
			Status:  "ok",
			Latency: latency,
		}
		if err != nil {
			status.Status = "error"
			status.Error = err.Error()
			response.Status = "degraded"
		}
		response.Providers["asr"] = status
	} else {
		response.Providers["asr"] = ProviderStatus{
			Name:   "aliyun",
			Status: "unavailable",
			Error:  "not configured",
		}
	}

	// 检查 TTS Provider
	if h.ttsProvider != nil {
		start := time.Now()
		err := h.ttsProvider.HealthCheck(ctx)
		latency := time.Since(start).Milliseconds()
		status := ProviderStatus{
			Name:    h.ttsProvider.Name(),
			Status:  "ok",
			Latency: latency,
		}
		if err != nil {
			status.Status = "error"
			status.Error = err.Error()
			response.Status = "degraded"
		}
		response.Providers["tts"] = status
	} else {
		response.Providers["tts"] = ProviderStatus{
			Name:   "doubao",
			Status: "unavailable",
			Error:  "not configured",
		}
	}

	// TODO: 检查数据库
	response.Providers["database"] = ProviderStatus{
		Name:   "postgres",
		Status: "unavailable",
		Error:  "not configured",
	}

	// 设置整体状态
	healthyCount := 0
	totalCount := 0
	for _, status := range response.Providers {
		totalCount++
		if status.Status == "ok" {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		response.Status = "unhealthy"
	} else if healthyCount < totalCount {
		response.Status = "degraded"
	}

	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// ListAPIs 列出当前系统接入的外部 API 及可用性
func (h *Handler) ListAPIs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	items := make([]APIItem, 0, 4)

	llmTargets := []struct {
		Name         string
		DefaultModel string
	}{
		{Name: "gemini", DefaultModel: "gemini-2.5-flash"},
		{Name: "dashscope", DefaultModel: "qwen-plus"},
	}

	primaryProvider := ""
	if h.llmManager != nil {
		if p, err := h.llmManager.Primary(); err == nil && p != nil {
			primaryProvider = p.Name()
		}
	}

	for _, target := range llmTargets {
		item := APIItem{
			ID:               "llm:" + target.Name,
			Name:             "大模型文本生成",
			Category:         "llm",
			Provider:         target.Name,
			ModelName:        target.DefaultModel,
			IsPrimary:        target.Name == primaryProvider,
			Status:           "unavailable",
			InternalEndpoint: "/api/admin/apis/llm:" + target.Name + "/test",
			Error:            "not configured",
		}

		if h.llmManager != nil {
			if provider, err := h.llmManager.Get(target.Name); err == nil {
				item.Status = "error"
				item.Error = ""
				if ep, ok := provider.(upstreamEndpointProvider); ok {
					item.UpstreamEndpoint = ep.UpstreamEndpoint()
				}
				if mp, ok := provider.(modelNameProvider); ok {
					item.ModelName = mp.ModelName()
				}

				// 外部可用性实时探测：发一个最小请求。
				testCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
				start := time.Now()
				rawProvider, ok := provider.(rawLLMRequestProvider)
				if ok {
					rawReq, rawResp, statusCode, probeErr := rawProvider.RawGenerate(testCtx, "hello")
					item.LatencyMS = time.Since(start).Milliseconds()
					item.RawRequestBody = rawReq
					item.RawResponseBody = rawResp
					item.RawStatusCode = statusCode
					if probeErr != nil {
						item.Status = "error"
						item.Error = probeErr.Error()
					} else if statusCode >= 200 && statusCode < 300 {
						item.Status = "ok"
					} else {
						item.Status = "error"
						item.Error = fmt.Sprintf("upstream status code: %d", statusCode)
					}
				} else {
					// 兜底：如果 provider 没有 raw 方法，至少走一次普通 chat。
					_, probeErr := provider.Chat(testCtx, []llm.Message{{Role: "user", Content: "hello"}})
					item.LatencyMS = time.Since(start).Milliseconds()
					item.RawRequestBody = fmt.Sprintf(`{"provider":"%s","prompt":"hello"}`, target.Name)
					item.RawResponseBody = ""
					item.RawStatusCode = http.StatusOK
					if probeErr != nil {
						item.Status = "error"
						item.Error = probeErr.Error()
					} else {
						item.Status = "ok"
					}
				}
				cancel()
			}
		}

		items = append(items, item)
	}

	// ASR provider
	if h.asrProvider != nil {
		item := APIItem{
			ID:               "asr",
			Name:             "语音识别",
			Category:         "asr",
			Provider:         h.asrProvider.Name(),
			InternalEndpoint: "/api/admin/apis/asr/test",
			Status:           "ok",
		}
		if ep, ok := h.asrProvider.(upstreamEndpointProvider); ok {
			item.UpstreamEndpoint = ep.UpstreamEndpoint()
		}

		// 外部可用性实时探测：ASR health check（token/网关连通）。
		testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		start := time.Now()
		probeErr := h.asrProvider.HealthCheck(testCtx)
		item.LatencyMS = time.Since(start).Milliseconds()
		item.RawRequestBody = fmt.Sprintf(`{"provider":"%s","check":"HealthCheck"}`, h.asrProvider.Name())
		if probeErr == nil {
			item.RawResponseBody = `{"message":"health check passed"}`
			item.RawStatusCode = http.StatusOK
		}
		cancel()
		if probeErr != nil {
			item.Status = "error"
			item.Error = probeErr.Error()
		}
		items = append(items, item)
	} else {
		items = append(items, APIItem{
			ID:               "asr",
			Name:             "语音识别",
			Category:         "asr",
			Provider:         "aliyun",
			InternalEndpoint: "/api/admin/apis/asr/test",
			UpstreamEndpoint: "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1 (token: https://nls-meta.cn-shanghai.aliyuncs.com/)",
			Status:           "unavailable",
			Error:            "not configured",
		})
	}

	// TTS provider
	if h.ttsProvider != nil {
		item := APIItem{
			ID:               "tts",
			Name:             "语音合成",
			Category:         "tts",
			Provider:         h.ttsProvider.Name(),
			InternalEndpoint: "/api/admin/apis/tts/test",
			Status:           "ok",
		}
		if ep, ok := h.ttsProvider.(upstreamEndpointProvider); ok {
			item.UpstreamEndpoint = ep.UpstreamEndpoint()
		}

		// 外部可用性实时探测：发最短文本合成请求。
		testCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		start := time.Now()
		_, probeErr := h.ttsProvider.Synthesize(testCtx, "你好", tts.SynthesisConfig{
			Format:     "mp3",
			SampleRate: 24000,
		})
		item.LatencyMS = time.Since(start).Milliseconds()
		item.RawRequestBody = fmt.Sprintf(`{"provider":"%s","text":"你好","format":"mp3","sample_rate":24000}`, h.ttsProvider.Name())
		if probeErr == nil {
			item.RawResponseBody = `{"message":"tts synthesize success"}`
			item.RawStatusCode = http.StatusOK
		}
		cancel()
		if probeErr != nil {
			item.Status = "error"
			item.Error = probeErr.Error()
		}
		items = append(items, item)
	} else {
		items = append(items, APIItem{
			ID:               "tts",
			Name:             "语音合成",
			Category:         "tts",
			Provider:         "doubao",
			InternalEndpoint: "/api/admin/apis/tts/test",
			UpstreamEndpoint: "https://openspeech.bytedance.com/api/v3/tts/unidirectional",
			Status:           "unavailable",
			Error:            "not configured",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"apis":       items,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// TestAPI 手动触发单个 API 可用性测试
func (h *Handler) TestAPI(c *gin.Context) {
	apiID := c.Param("api_id")

	switch {
	case strings.HasPrefix(apiID, "llm:"):
		providerName := strings.TrimPrefix(apiID, "llm:")
		if providerName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid llm provider"})
			return
		}
		if h.llmManager == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM manager not initialized"})
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
		}
		_ = c.ShouldBindJSON(&req)
		promptText := strings.TrimSpace(req.Prompt)
		if promptText == "" {
			promptText = "请仅回复：ok"
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		provider, err := h.llmManager.Get(providerName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"id":     apiID,
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		start := time.Now()
		rawReq := ""
		rawResp := ""
		rawStatus := 0

		if rawProvider, ok := provider.(rawLLMRequestProvider); ok {
			rawReq, rawResp, rawStatus, err = rawProvider.RawGenerate(ctx, promptText)
		} else {
			chatResp, chatErr := provider.Chat(ctx, []llm.Message{{Role: "user", Content: promptText}})
			if chatErr != nil {
				err = chatErr
			} else {
				rawReq = fmt.Sprintf(`{"provider":"%s","prompt":%q}`, providerName, promptText)
				rawResp = chatResp.Content
				rawStatus = http.StatusOK
			}
		}
		latency := time.Since(start).Milliseconds()
		if err != nil {
			errType, errHint := classifyProviderError(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"id":                apiID,
				"status":            "error",
				"error":             err.Error(),
				"error_type":        errType,
				"error_hint":        errHint,
				"latency_ms":        latency,
				"raw_request_body":  rawReq,
				"raw_response_body": rawResp,
				"raw_status_code":   rawStatus,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":                apiID,
			"status":            "ok",
			"provider":          providerName,
			"latency_ms":        latency,
			"raw_request_body":  rawReq,
			"raw_response_body": rawResp,
			"raw_status_code":   rawStatus,
		})
		return

	case apiID == "asr":
		if h.asrProvider == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"id":     apiID,
				"status": "unavailable",
				"error":  "ASR provider not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		requestBody := gin.H{
			"provider": h.asrProvider.Name(),
			"check":    "HealthCheck",
		}
		start := time.Now()
		err := h.asrProvider.HealthCheck(ctx)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			errType, errHint := classifyProviderError(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"id":           apiID,
				"status":       "error",
				"provider":     h.asrProvider.Name(),
				"error":        err.Error(),
				"error_type":   errType,
				"error_hint":   errHint,
				"latency_ms":   latency,
				"request_body": requestBody,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":           apiID,
			"status":       "ok",
			"provider":     h.asrProvider.Name(),
			"latency_ms":   latency,
			"request_body": requestBody,
			"response_body": gin.H{
				"message": "health check passed",
			},
		})
		return

	case apiID == "tts":
		if h.ttsProvider == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"id":     apiID,
				"status": "unavailable",
				"error":  "TTS provider not configured",
			})
			return
		}

		var req struct {
			Text       string `json:"text"`
			Voice      string `json:"voice"`
			SampleRate int    `json:"sample_rate"`
			Format     string `json:"format"`
		}
		_ = c.ShouldBindJSON(&req)

		text := strings.TrimSpace(req.Text)
		if text == "" {
			text = "你好，这是系统连通性测试。"
		}
		format := req.Format
		if format == "" {
			format = "mp3"
		}
		sampleRate := req.SampleRate
		if sampleRate <= 0 {
			sampleRate = 24000
		}
		requestBody := gin.H{
			"provider":    h.ttsProvider.Name(),
			"text":        text,
			"voice":       req.Voice,
			"format":      format,
			"sample_rate": sampleRate,
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		start := time.Now()
		audio, err := h.ttsProvider.Synthesize(ctx, text, tts.SynthesisConfig{
			Voice:      req.Voice,
			Format:     format,
			SampleRate: sampleRate,
		})
		latency := time.Since(start).Milliseconds()
		if err != nil {
			errType, errHint := classifyProviderError(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"id":           apiID,
				"status":       "error",
				"provider":     h.ttsProvider.Name(),
				"error":        err.Error(),
				"error_type":   errType,
				"error_hint":   errHint,
				"latency_ms":   latency,
				"request_body": requestBody,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":           apiID,
			"status":       "ok",
			"provider":     h.ttsProvider.Name(),
			"audio_bytes":  len(audio),
			"latency_ms":   latency,
			"request_body": requestBody,
			"response_body": gin.H{
				"audio_bytes": len(audio),
			},
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported api id"})
}

func classifyProviderError(errMsg string) (string, string) {
	msg := strings.ToLower(errMsg)

	switch {
	case strings.Contains(msg, "quota"),
		strings.Contains(msg, "insufficient"),
		strings.Contains(msg, "arrear"),
		strings.Contains(msg, "balance"),
		strings.Contains(msg, "欠费"),
		strings.Contains(msg, "余额"):
		return "billing_or_quota", "可能是额度不足、账户欠费或配额超限，请检查供应商账单与配额。"
	case strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "forbidden"),
		strings.Contains(msg, "invalid api key"),
		strings.Contains(msg, "signature"),
		strings.Contains(msg, "permission"),
		strings.Contains(msg, "access denied"),
		strings.Contains(msg, "401"),
		strings.Contains(msg, "403"):
		return "auth_or_permission", "可能是 AK/SK、Token、签名或权限配置问题，请核对密钥与权限。"
	case strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "timeout"),
		strings.Contains(msg, "i/o timeout"):
		return "timeout", "请求超时，可能是网络波动或上游服务响应慢。"
	case strings.Contains(msg, "no such host"),
		strings.Contains(msg, "dial tcp"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "tls"):
		return "network", "网络或 DNS/TLS 连接异常，请检查服务器网络与上游域名连通性。"
	case strings.Contains(msg, "429"),
		strings.Contains(msg, "503"),
		strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "service unavailable"),
		strings.Contains(msg, "too many requests"):
		return "provider_unstable", "上游服务限流或不可用，建议重试并关注供应商状态页。"
	default:
		return "unknown", "未知错误，请结合完整报错和供应商日志继续排查。"
	}
}

// GetLLMProviders 获取 LLM Provider 列表
func (h *Handler) GetLLMProviders(c *gin.Context) {
	if h.llmManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"providers": []string{},
			"primary":   "",
		})
		return
	}

	primary, _ := h.llmManager.Primary()
	primaryName := ""
	if primary != nil {
		primaryName = primary.Name()
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": h.llmManager.Available(),
		"primary":   primaryName,
	})
}

// SetLLMProvider 设置主要 LLM Provider
func (h *Handler) SetLLMProvider(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.llmManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM manager not initialized"})
		return
	}

	if err := h.llmManager.SetPrimary(req.Provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "primary provider updated",
		"provider": req.Provider,
	})
}

// TestLLMProvider 测试 LLM Provider
func (h *Handler) TestLLMProvider(c *gin.Context) {
	providerName := c.Param("provider")

	var req struct {
		Prompt string `json:"prompt" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.llmManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM manager not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := h.llmManager.ChatWith(ctx, providerName, []llm.Message{
		{Role: "user", Content: req.Prompt},
	})
	latency := time.Since(start).Milliseconds()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      err.Error(),
			"latency_ms": latency,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":      providerName,
		"response":      resp.Content,
		"finish_reason": resp.FinishReason,
		"tokens_used":   resp.TokensUsed,
		"latency_ms":    latency,
	})
}

// GetStats 获取统计数据
func (h *Handler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取用户统计
	users, totalUsers, _ := h.userService.AdminList(ctx, 10000, 0)

	// 计算资料完成率
	profileCompleted := 0
	birthDecadeMap := make(map[string]int)
	hometownMap := make(map[string]int)

	for _, u := range users {
		if u.ProfileCompleted {
			profileCompleted++
		}
		// 出生年代分布
		if u.BirthYear != nil && *u.BirthYear > 0 {
			decade := (*u.BirthYear / 10) * 10
			label := fmt.Sprintf("%d年代", decade)
			birthDecadeMap[label]++
		}
		// 家乡分布（简化处理，取省份）
		if u.Hometown != nil && *u.Hometown != "" {
			province := extractProvince(*u.Hometown)
			if province != "" {
				hometownMap[province]++
			}
		}
	}

	profileRate := 0.0
	if totalUsers > 0 {
		profileRate = float64(profileCompleted) / float64(totalUsers)
	}

	// 获取对话统计
	conversations, totalConversations, _ := h.convService.AdminList(ctx, nil, nil, 10000, 0)

	// 获取回忆录统计
	memoirs, totalMemoirs, _ := h.memoirService.AdminList(ctx, 10000, 0, false)

	// 计算今日和本周活跃
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -7)

	todayActiveUsers := make(map[uuid.UUID]bool)
	weekActiveUsers := make(map[uuid.UUID]bool)
	todayNewConversations := 0
	todayNewMemoirs := 0

	// 用户对话次数分布
	userConvCount := make(map[uuid.UUID]int)
	// 对话消息数分布
	msgCountBuckets := map[string]int{
		"0-5轮":   0,
		"6-10轮":  0,
		"11-20轮": 0,
		"21-50轮": 0,
		"50轮以上":  0,
	}

	for _, conv := range conversations {
		userConvCount[conv.UserID]++

		if conv.CreatedAt.After(todayStart) {
			todayActiveUsers[conv.UserID] = true
			todayNewConversations++
		}
		if conv.CreatedAt.After(weekStart) {
			weekActiveUsers[conv.UserID] = true
		}

		// 消息数分布
		msgCount := conv.MessageCount
		switch {
		case msgCount <= 5:
			msgCountBuckets["0-5轮"]++
		case msgCount <= 10:
			msgCountBuckets["6-10轮"]++
		case msgCount <= 20:
			msgCountBuckets["11-20轮"]++
		case msgCount <= 50:
			msgCountBuckets["21-50轮"]++
		default:
			msgCountBuckets["50轮以上"]++
		}
	}

	// 用户回忆录数分布
	userMemoirCount := make(map[uuid.UUID]int)
	for _, m := range memoirs {
		userMemoirCount[m.UserID]++
		if m.CreatedAt.After(todayStart) {
			todayNewMemoirs++
		}
	}

	// 构建分布数据
	convPerUserBuckets := map[string]int{
		"0次":    0,
		"1-2次":  0,
		"3-5次":  0,
		"6-10次": 0,
		"10次以上": 0,
	}
	memoirPerUserBuckets := map[string]int{
		"0篇":   0,
		"1-2篇": 0,
		"3-5篇": 0,
		"6篇以上": 0,
	}

	for _, u := range users {
		convCount := userConvCount[u.ID]
		switch {
		case convCount == 0:
			convPerUserBuckets["0次"]++
		case convCount <= 2:
			convPerUserBuckets["1-2次"]++
		case convCount <= 5:
			convPerUserBuckets["3-5次"]++
		case convCount <= 10:
			convPerUserBuckets["6-10次"]++
		default:
			convPerUserBuckets["10次以上"]++
		}

		memoirCount := userMemoirCount[u.ID]
		switch {
		case memoirCount == 0:
			memoirPerUserBuckets["0篇"]++
		case memoirCount <= 2:
			memoirPerUserBuckets["1-2篇"]++
		case memoirCount <= 5:
			memoirPerUserBuckets["3-5篇"]++
		default:
			memoirPerUserBuckets["6篇以上"]++
		}
	}

	// 简单的留存率计算（基于有多次对话的用户比例）
	retention1 := 0.0
	retention7 := 0.0
	retention30 := 0.0
	if totalUsers > 0 {
		usersWithMultipleConv := 0
		for _, count := range userConvCount {
			if count > 1 {
				usersWithMultipleConv++
			}
		}
		retention1 = float64(len(todayActiveUsers)) / float64(totalUsers)
		retention7 = float64(len(weekActiveUsers)) / float64(totalUsers)
		retention30 = float64(usersWithMultipleConv) / float64(totalUsers)
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"total_users":             totalUsers,
			"profile_completion_rate": profileRate,
			"total_conversations":     totalConversations,
			"total_memoirs":           totalMemoirs,
		},
		"activity": gin.H{
			"today_active_users":      len(todayActiveUsers),
			"week_active_users":       len(weekActiveUsers),
			"today_new_conversations": todayNewConversations,
			"today_new_memoirs":       todayNewMemoirs,
		},
		"retention": gin.H{
			"day1":  retention1,
			"day7":  retention7,
			"day30": retention30,
		},
		"distributions": gin.H{
			"birth_decade":              mapToDistribution(birthDecadeMap),
			"hometown_province":         mapToDistribution(hometownMap),
			"conversations_per_user":    bucketToDistribution(convPerUserBuckets, []string{"0次", "1-2次", "3-5次", "6-10次", "10次以上"}),
			"memoirs_per_user":          bucketToDistribution(memoirPerUserBuckets, []string{"0篇", "1-2篇", "3-5篇", "6篇以上"}),
			"messages_per_conversation": bucketToDistribution(msgCountBuckets, []string{"0-5轮", "6-10轮", "11-20轮", "21-50轮", "50轮以上"}),
		},
	})
}

// extractProvince 从地址中提取省份
func extractProvince(address string) string {
	provinces := []string{"北京", "上海", "天津", "重庆", "河北", "山西", "辽宁", "吉林", "黑龙江",
		"江苏", "浙江", "安徽", "福建", "江西", "山东", "河南", "湖北", "湖南", "广东",
		"海南", "四川", "贵州", "云南", "陕西", "甘肃", "青海", "台湾", "内蒙古", "广西",
		"西藏", "宁夏", "新疆", "香港", "澳门"}
	for _, p := range provinces {
		if strings.Contains(address, p) {
			return p
		}
	}
	return ""
}

// mapToDistribution 将 map 转换为分布数组
func mapToDistribution(m map[string]int) []gin.H {
	var result []gin.H
	for label, count := range m {
		result = append(result, gin.H{"label": label, "count": count})
	}
	// 按 count 降序排序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i]["count"].(int) < result[j]["count"].(int) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// bucketToDistribution 将桶转换为有序分布数组
func bucketToDistribution(buckets map[string]int, order []string) []gin.H {
	var result []gin.H
	for _, label := range order {
		result = append(result, gin.H{"label": label, "count": buckets[label]})
	}
	return result
}

// GetTTSVoices 获取 TTS 音色列表
func (h *Handler) GetTTSVoices(c *gin.Context) {
	if h.ttsProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TTS provider not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	voices, err := h.ttsProvider.ListVoices(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider": h.ttsProvider.Name(),
		"voices":   voices,
	})
}

// TestTTS 测试 TTS 合成
func (h *Handler) TestTTS(c *gin.Context) {
	if h.ttsProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TTS provider not configured"})
		return
	}

	var req struct {
		Text       string `json:"text" binding:"required"`
		Voice      string `json:"voice"`
		Format     string `json:"format"`
		SampleRate int    `json:"sample_rate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	config := tts.SynthesisConfig{
		Voice:      req.Voice,
		Format:     req.Format,
		SampleRate: req.SampleRate,
	}

	start := time.Now()
	audio, err := h.ttsProvider.Synthesize(ctx, req.Text, config)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      err.Error(),
			"latency_ms": latency,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":    h.ttsProvider.Name(),
		"audio_bytes": len(audio),
		"latency_ms":  latency,
	})
}

// --- 时代记忆预设管理 ---

// ListEraMemories 获取时代记忆预设列表
func (h *Handler) ListEraMemories(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var category *string
	if cat := c.Query("category"); cat != "" {
		category = &cat
	}

	memories, total, err := h.eraService.List(c.Request.Context(), category, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"era_memories": memories,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// CreateEraMemory 创建时代记忆预设
func (h *Handler) CreateEraMemory(c *gin.Context) {
	var input era.CreateMemoryPresetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.eraService.Create(c.Request.Context(), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionCreateEraMemory,
		TargetType:  audit.TargetTypeEraMemory,
		TargetID:    &m.ID,
		TargetLabel: fmt.Sprintf("%d-%d", m.StartYear, m.EndYear),
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusCreated, m)
}

// UpdateEraMemory 更新时代记忆预设
func (h *Handler) UpdateEraMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory id"})
		return
	}

	var input era.UpdateMemoryPresetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.eraService.Update(c.Request.Context(), memoryID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, eraService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionUpdateEraMemory,
		TargetType:  audit.TargetTypeEraMemory,
		TargetID:    &memoryID,
		TargetLabel: fmt.Sprintf("%d-%d", m.StartYear, m.EndYear),
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, m)
}

// DeleteEraMemory 删除时代记忆预设
func (h *Handler) DeleteEraMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory id"})
		return
	}

	if err := h.eraService.Delete(c.Request.Context(), memoryID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, eraService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:     audit.ActionDeleteEraMemory,
		TargetType: audit.TargetTypeEraMemory,
		TargetID:   &memoryID,
		IPAddress:  c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "era memory deleted"})
}

// --- 预设话题管理 ---

// ListPresetTopics 获取预设话题列表
func (h *Handler) ListPresetTopics(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	activeOnly := c.Query("active_only") == "true"

	topics, total, err := h.presetService.List(c.Request.Context(), activeOnly, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"preset_topics": topics,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

// CreatePresetTopic 创建预设话题
func (h *Handler) CreatePresetTopic(c *gin.Context) {
	var input preset.CreateTopicInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.presetService.Create(c.Request.Context(), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionCreatePresetTopic,
		TargetType:  audit.TargetTypePresetTopic,
		TargetID:    &t.ID,
		TargetLabel: t.Topic,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusCreated, t)
}

// UpdatePresetTopic 更新预设话题
func (h *Handler) UpdatePresetTopic(c *gin.Context) {
	topicID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic id"})
		return
	}

	var input preset.UpdateTopicInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.presetService.Update(c.Request.Context(), topicID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, presetService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionUpdatePresetTopic,
		TargetType:  audit.TargetTypePresetTopic,
		TargetID:    &topicID,
		TargetLabel: t.Topic,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, t)
}

// DeletePresetTopic 删除预设话题
func (h *Handler) DeletePresetTopic(c *gin.Context) {
	topicID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic id"})
		return
	}

	if err := h.presetService.Delete(c.Request.Context(), topicID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, presetService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:     audit.ActionDeletePresetTopic,
		TargetType: audit.TargetTypePresetTopic,
		TargetID:   &topicID,
		IPAddress:  c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "preset topic deleted"})
}

// --- 欢迎语管理 ---

// ListWelcomeMessages 获取欢迎语列表
func (h *Handler) ListWelcomeMessages(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, total, err := h.welcomeService.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"welcome_messages": messages,
		"total":            total,
		"limit":            limit,
		"offset":           offset,
	})
}

// CreateWelcomeMessage 创建欢迎语
func (h *Handler) CreateWelcomeMessage(c *gin.Context) {
	var input welcome.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.welcomeService.Create(c.Request.Context(), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	label := m.Content
	if len(label) > 30 {
		label = label[:30] + "..."
	}
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionCreateWelcome,
		TargetType:  audit.TargetTypeWelcome,
		TargetID:    &m.ID,
		TargetLabel: label,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusCreated, m)
}

// UpdateWelcomeMessage 更新欢迎语
func (h *Handler) UpdateWelcomeMessage(c *gin.Context) {
	messageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	var input welcome.UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.welcomeService.Update(c.Request.Context(), messageID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, welcomeService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	label := m.Content
	if len(label) > 30 {
		label = label[:30] + "..."
	}
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:      audit.ActionEditWelcome,
		TargetType:  audit.TargetTypeWelcome,
		TargetID:    &messageID,
		TargetLabel: label,
		IPAddress:   c.ClientIP(),
	})

	c.JSON(http.StatusOK, m)
}

// DeleteWelcomeMessage 删除欢迎语
func (h *Handler) DeleteWelcomeMessage(c *gin.Context) {
	messageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	if err := h.welcomeService.Delete(c.Request.Context(), messageID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, welcomeService.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 记录审计日志
	h.auditService.Log(c.Request.Context(), audit.CreateInput{
		Action:     audit.ActionDeleteWelcome,
		TargetType: audit.TargetTypeWelcome,
		TargetID:   &messageID,
		IPAddress:  c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "welcome message deleted"})
}

// --- 审计日志 ---

// ListAuditLogs 获取审计日志列表
func (h *Handler) ListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := h.auditService.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": logs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

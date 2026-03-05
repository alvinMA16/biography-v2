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

	c.JSON(http.StatusOK, user)
}

// DeleteUser 删除用户（软删除）
func (h *Handler) DeleteUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := h.userService.AdminDelete(c.Request.Context(), userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
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

	if err := h.userService.AdminResetPassword(c.Request.Context(), userID, input.NewPassword); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

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

	c.JSON(http.StatusOK, gin.H{
		"message":   "user status updated",
		"is_active": u.IsActive,
	})
}

// GetUserStats 获取用户统计信息
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

	// 获取对话数量
	conversations, totalConv, _ := h.convService.AdminList(ctx, &userID, nil, 1000, 0)

	// 获取回忆录数量
	memoirCount, _ := h.memoirService.Count(ctx, userID)

	// 获取话题数量
	availableTopics, _ := h.topicService.GetAvailableCount(ctx, userID)
	pendingTopics, _ := h.topicService.GetPendingCount(ctx, userID)

	// 统计对话状态
	activeConv := 0
	completedConv := 0
	for _, conv := range conversations {
		if conv.Status == "active" {
			activeConv++
		} else if conv.Status == "completed" {
			completedConv++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": u,
		"stats": gin.H{
			"conversations": gin.H{
				"total":     totalConv,
				"active":    activeConv,
				"completed": completedConv,
			},
			"memoirs": gin.H{
				"total": memoirCount,
			},
			"topics": gin.H{
				"available": availableTopics,
				"pending":   pendingTopics,
			},
		},
	})
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
	// TODO: 实现统计数据
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
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

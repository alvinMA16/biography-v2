package user

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/peizhengma/biography-v2/internal/domain/conversation"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	"github.com/peizhengma/biography-v2/internal/domain/user"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	flowService "github.com/peizhengma/biography-v2/internal/service/flow"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	presetService "github.com/peizhengma/biography-v2/internal/service/preset"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
	welcomeService "github.com/peizhengma/biography-v2/internal/service/welcome"
)

// Handler 用户 API 处理器
type Handler struct {
	userService    *userService.Service
	convService    *convService.Service
	memoirService  *memoirService.Service
	topicService   *topicService.Service
	presetService  *presetService.Service
	flowService    *flowService.Service
	llmService     *llmService.Service
	welcomeService *welcomeService.Service
}

// NewHandler 创建 Handler
func NewHandler(userSvc *userService.Service, convSvc *convService.Service, memoirSvc *memoirService.Service, topicSvc *topicService.Service, presetSvc *presetService.Service, flowSvc *flowService.Service, llmSvc *llmService.Service, welcomeSvc *welcomeService.Service) *Handler {
	return &Handler{
		userService:    userSvc,
		convService:    convSvc,
		memoirService:  memoirSvc,
		topicService:   topicSvc,
		presetService:  presetSvc,
		flowService:    flowSvc,
		llmService:     llmSvc,
		welcomeService: welcomeSvc,
	}
}

// Register 用户注册
func (h *Handler) Register(c *gin.Context) {
	var input user.CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userService.Register(c.Request.Context(), &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrInvalidPhone) ||
			errors.Is(err, userService.ErrInvalidPassword) {
			status = http.StatusBadRequest
		} else if errors.Is(err, userService.ErrPhoneAlreadyExists) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// Login 用户登录
func (h *Handler) Login(c *gin.Context) {
	var input user.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userService.Login(c.Request.Context(), &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetProfile 获取用户信息
func (h *Handler) GetProfile(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	u, err := h.userService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, u)
}

// GetWelcomeMessages 获取激励语列表（用于首页显示）
func (h *Handler) GetWelcomeMessages(c *gin.Context) {
	messages, err := h.welcomeService.GetActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回简化的结构
	result := make([]gin.H, 0, len(messages))
	for _, m := range messages {
		result = append(result, gin.H{
			"content":       m.Content,
			"show_greeting": m.ShowGreeting,
		})
	}

	c.JSON(http.StatusOK, result)
}

// UpdateProfile 更新用户信息
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input user.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.userService.UpdateProfile(c.Request.Context(), userID, &input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, u)
}

// ChangePassword 修改密码
func (h *Handler) ChangePassword(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.userService.ChangePassword(c.Request.Context(), userID, input.OldPassword, input.NewPassword)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, userService.ErrUserNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, userService.ErrWrongPassword) {
			status = http.StatusBadRequest
		} else if errors.Is(err, userService.ErrInvalidPassword) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// getUserID 从 context 获取用户 ID
func getUserID(c *gin.Context) uuid.UUID {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil
	}

	switch v := userIDStr.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil
		}
		return id
	case uuid.UUID:
		return v
	default:
		return uuid.Nil
	}
}

// ============================================
// 对话相关接口
// ============================================

// ListConversations 获取对话列表
func (h *Handler) ListConversations(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 解析分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// 解析状态过滤
	var status *conversation.Status
	if s := c.Query("status"); s != "" {
		st := conversation.Status(s)
		status = &st
	}

	conversations, total, err := h.convService.List(c.Request.Context(), userID, status, limit, offset)
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

// CreateConversation 创建对话
func (h *Handler) CreateConversation(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input conversation.CreateConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.convService.Create(c.Request.Context(), userID, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, conv)
}

// GetConversation 获取对话详情
func (h *Handler) GetConversation(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	conv, err := h.convService.GetWithMessages(c.Request.Context(), convID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, convService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, convService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// GetMessages 获取对话消息
func (h *Handler) GetMessages(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	// 解析分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := h.convService.GetMessages(c.Request.Context(), convID, userID, limit, offset)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, convService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, convService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	})
}

// ============================================
// 回忆录和话题（待实现）
// ============================================

// ListMemoirs 获取回忆录列表
func (h *Handler) ListMemoirs(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 解析分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	memoirs, total, err := h.memoirService.List(c.Request.Context(), userID, limit, offset)
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

// GetMemoir 获取回忆录详情
func (h *Handler) GetMemoir(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	m, err := h.memoirService.GetByIDForUser(c.Request.Context(), memoirID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, memoirService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, m)
}

// UpdateMemoir 更新回忆录
func (h *Handler) UpdateMemoir(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	var input struct {
		Title      *string `json:"title"`
		Content    *string `json:"content"`
		TimePeriod *string `json:"time_period"`
		StartYear  *int    `json:"start_year"`
		EndYear    *int    `json:"end_year"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.memoirService.Update(c.Request.Context(), memoirID, userID, &memoir.UpdateMemoirInput{
		Title:      input.Title,
		Content:    input.Content,
		TimePeriod: input.TimePeriod,
		StartYear:  input.StartYear,
		EndYear:    input.EndYear,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, memoirService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, m)
}

// RegenerateMemoir 重新生成回忆录内容
func (h *Handler) RegenerateMemoir(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	var input struct {
		Perspective string `json:"perspective"`
	}
	_ = c.ShouldBindJSON(&input) // 目前仅兼容接收参数，暂未区分人称 prompt

	// 获取回忆录与归属校验
	m, err := h.memoirService.GetByIDForUser(c.Request.Context(), memoirID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, memoirService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if m.ConversationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memoir has no source conversation"})
		return
	}

	// 获取对话和消息
	conv, err := h.convService.GetWithMessages(c.Request.Context(), *m.ConversationID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, convService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, convService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	conversationText := buildConversationText(conv.Messages)
	if strings.TrimSpace(conversationText) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation is empty"})
		return
	}

	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user profile"})
		return
	}

	topicTitle := ""
	if conv.Topic != nil {
		topicTitle = *conv.Topic
	}

	generated, err := h.llmService.GenerateMemoir(c.Request.Context(), &llmService.GenerateMemoirInput{
		UserName:     u.DisplayName(),
		BirthYear:    u.BirthYear,
		Hometown:     stringValue(u.Hometown),
		Topic:        topicTitle,
		Conversation: conversationText,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.memoirService.Update(c.Request.Context(), memoirID, userID, &memoir.UpdateMemoirInput{
		Title:      &generated.Title,
		Content:    &generated.Content,
		TimePeriod: &generated.TimePeriod,
		StartYear:  generated.StartYear,
		EndYear:    generated.EndYear,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteMemoir 删除回忆录
func (h *Handler) DeleteMemoir(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	memoirID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memoir id"})
		return
	}

	if err := h.memoirService.Delete(c.Request.Context(), memoirID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memoirService.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, memoirService.ErrNotOwner) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "memoir deleted"})
}

// GetTopicOptions 获取话题选项
func (h *Handler) GetTopicOptions(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 解析数量参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))

	// 先获取用户个性化话题
	options, err := h.topicService.GetTopicOptions(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果用户话题不足，补充预设话题
	if len(options) < limit {
		presets, err := h.presetService.GetActiveTopics(c.Request.Context())
		if err == nil && len(presets) > 0 {
			// 记录已有话题标题，避免重复
			existingTitles := make(map[string]bool)
			for _, opt := range options {
				existingTitles[opt.Title] = true
			}

			// 补充预设话题
			for _, p := range presets {
				if len(options) >= limit {
					break
				}
				// 跳过重复标题
				if existingTitles[p.Topic] {
					continue
				}
				context := ""
				if p.ChatContext != nil {
					context = *p.ChatContext
				}
				options = append(options, topic.TopicOption{
					ID:       p.ID,
					Title:    p.Topic,
					Greeting: p.Greeting,
					Context:  context,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"topics": options,
	})
}

// EndConversation 结束对话（同步处理）
func (h *Handler) EndConversation(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	result, err := h.flowService.EndConversation(c.Request.Context(), convID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, flowService.ErrConversationNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, flowService.ErrNotOwner) {
			status = http.StatusForbidden
		} else if errors.Is(err, flowService.ErrAlreadyCompleted) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// EndConversationQuick 快速结束对话（异步处理）
func (h *Handler) EndConversationQuick(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	err = h.flowService.EndConversationQuick(c.Request.Context(), convID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, flowService.ErrConversationNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, flowService.ErrNotOwner) {
			status = http.StatusForbidden
		} else if errors.Is(err, flowService.ErrAlreadyCompleted) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "对话已结束，正在后台处理"})
}

// GenerateEraMemories 生成时代记忆
func (h *Handler) GenerateEraMemories(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 获取用户信息
	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 检查是否有出生年份
	if u.BirthYear == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "birth year is required"})
		return
	}

	// 检查状态，避免重复生成
	if u.EraMemoriesStatus == user.EraMemoriesStatusGenerating {
		c.JSON(http.StatusBadRequest, gin.H{"error": "era memories is already being generated"})
		return
	}

	if u.EraMemoriesStatus == user.EraMemoriesStatusCompleted && u.EraMemories != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":       "completed",
			"era_memories": *u.EraMemories,
			"message":      "时代记忆已生成",
		})
		return
	}

	// 更新状态为生成中
	if err := h.userService.UpdateEraMemoriesStatus(c.Request.Context(), userID, user.EraMemoriesStatusGenerating); err != nil {
		log.Printf("[EraMemories] 更新状态失败: %v", err)
	}

	// 启动后台生成
	go h.generateEraMemoriesAsync(userID, u)

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "generating",
		"message": "时代记忆生成中，请稍后查询",
	})
}

// generateEraMemoriesAsync 异步生成时代记忆
func (h *Handler) generateEraMemoriesAsync(userID uuid.UUID, u *user.User) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	hometown := ""
	if u.Hometown != nil {
		hometown = *u.Hometown
	}
	mainCity := ""
	if u.MainCity != nil {
		mainCity = *u.MainCity
	}

	eraMemories, err := h.llmService.GenerateEraMemories(ctx, *u.BirthYear, hometown, mainCity)
	if err != nil {
		log.Printf("[EraMemories] 生成失败: %v", err)
		h.userService.UpdateEraMemoriesStatus(ctx, userID, user.EraMemoriesStatusFailed)
		return
	}

	// 保存结果
	if err := h.userService.UpdateEraMemories(ctx, userID, eraMemories, user.EraMemoriesStatusCompleted); err != nil {
		log.Printf("[EraMemories] 保存失败: %v", err)
		h.userService.UpdateEraMemoriesStatus(ctx, userID, user.EraMemoriesStatusFailed)
		return
	}

	log.Printf("[EraMemories] 用户 %s 时代记忆生成成功", userID)
}

// GetEraMemoriesStatus 获取时代记忆状态
func (h *Handler) GetEraMemoriesStatus(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	result := gin.H{
		"status": u.EraMemoriesStatus,
	}

	if u.EraMemories != nil {
		result["era_memories"] = *u.EraMemories
	}

	c.JSON(http.StatusOK, result)
}

// ExportUserData 导出用户数据
func (h *Handler) ExportUserData(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 获取用户信息
	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 获取对话列表
	conversations, _, err := h.convService.List(c.Request.Context(), userID, nil, 1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取回忆录列表
	memoirs, _, err := h.memoirService.List(c.Request.Context(), userID, 1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 构建导出数据
	exportConvs := make([]map[string]any, len(conversations))
	for i, conv := range conversations {
		exportConvs[i] = map[string]any{
			"id":         conv.ID,
			"topic":      conv.Topic,
			"status":     conv.Status,
			"summary":    conv.Summary,
			"created_at": conv.CreatedAt,
		}
	}

	exportMemoirs := make([]map[string]any, len(memoirs))
	for i, m := range memoirs {
		exportMemoirs[i] = map[string]any{
			"id":          m.ID,
			"title":       m.Title,
			"content":     m.Content,
			"time_period": m.TimePeriod,
			"start_year":  m.StartYear,
			"end_year":    m.EndYear,
			"created_at":  m.CreatedAt,
		}
	}

	exportData := &user.ExportData{
		Profile:       u,
		Conversations: exportConvs,
		Memoirs:       exportMemoirs,
		ExportedAt:    time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, exportData)
}

// DeleteAccount 注销账户
func (h *Handler) DeleteAccount(c *gin.Context) {
	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 验证密码
	var input struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}

	// 验证用户身份
	u, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 验证密码 - 通过尝试用旧密码更改来验证
	err = h.userService.ChangePassword(c.Request.Context(), userID, input.Password, input.Password)
	if err != nil && errors.Is(err, userService.ErrWrongPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "password incorrect"})
		return
	}

	// 执行软删除
	if err := h.userService.AdminDelete(c.Request.Context(), u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account deleted successfully"})
}

func buildConversationText(messages []conversation.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "记录师"
		}
		sb.WriteString(fmt.Sprintf("%s：%s\n\n", role, msg.Content))
	}
	return sb.String()
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

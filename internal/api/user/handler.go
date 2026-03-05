package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/peizhengma/biography-v2/internal/domain/conversation"
	"github.com/peizhengma/biography-v2/internal/domain/user"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
)

// Handler 用户 API 处理器
type Handler struct {
	userService *userService.Service
	convService *convService.Service
}

// NewHandler 创建 Handler
func NewHandler(userSvc *userService.Service, convSvc *convService.Service) *Handler {
	return &Handler{
		userService: userSvc,
		convService: convSvc,
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
	// TODO: 需要 MemoirService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// GetMemoir 获取回忆录详情
func (h *Handler) GetMemoir(c *gin.Context) {
	// TODO: 需要 MemoirService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// GetTopicOptions 获取话题选项
func (h *Handler) GetTopicOptions(c *gin.Context) {
	// TODO: 需要 TopicService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/peizhengma/biography-v2/internal/domain/user"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
)

// Handler 用户 API 处理器
type Handler struct {
	userService *userService.Service
}

// NewHandler 创建 Handler
func NewHandler(userSvc *userService.Service) *Handler {
	return &Handler{
		userService: userSvc,
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

// --- 以下是需要其他 Service 实现的接口，暂时保留 TODO ---

// ListConversations 获取对话列表
func (h *Handler) ListConversations(c *gin.Context) {
	// TODO: 需要 ConversationService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// CreateConversation 创建对话
func (h *Handler) CreateConversation(c *gin.Context) {
	// TODO: 需要 ConversationService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// GetConversation 获取对话详情
func (h *Handler) GetConversation(c *gin.Context) {
	// TODO: 需要 ConversationService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// GetMessages 获取对话消息
func (h *Handler) GetMessages(c *gin.Context) {
	// TODO: 需要 ConversationService
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

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

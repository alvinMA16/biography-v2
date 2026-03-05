package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
	"github.com/peizhengma/biography-v2/internal/domain/user"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
)

// Handler Admin API 处理器
type Handler struct {
	llmManager    *llm.Manager
	asrProvider   asr.Provider
	ttsProvider   tts.Provider
	userService   *userService.Service
	convService   *convService.Service
	memoirService *memoirService.Service
	topicService  *topicService.Service
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
) *Handler {
	return &Handler{
		llmManager:    llmManager,
		asrProvider:   asrProvider,
		ttsProvider:   ttsProvider,
		userService:   userSvc,
		convService:   convSvc,
		memoirService: memoirSvc,
		topicService:  topicSvc,
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
	// TODO: 需要 LLM Service 实现回忆录生成
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented - requires LLM service"})
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
	// TODO: 实现
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// CreateQuote 创建激励语
func (h *Handler) CreateQuote(c *gin.Context) {
	// TODO: 实现
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// UpdateQuote 更新激励语
func (h *Handler) UpdateQuote(c *gin.Context) {
	// TODO: 实现
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// DeleteQuote 删除激励语
func (h *Handler) DeleteQuote(c *gin.Context) {
	// TODO: 实现
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
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

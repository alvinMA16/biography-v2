package realtime

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/peizhengma/biography-v2/internal/domain/conversation"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: 在生产环境中应该检查 Origin
		return true
	},
}

// Handler 实时对话处理器
type Handler struct {
	jwtSecret     string
	userService   *userService.Service
	topicService  *topicService.Service
	convService   *convService.Service
	memoirService *memoirService.Service
	llmService    *llmService.Service
	asrProvider   asr.Provider
	llmManager    *llm.Manager
	ttsProvider   tts.Provider
}

// NewHandler 创建处理器
func NewHandler(
	jwtSecret string,
	userSvc *userService.Service,
	topicSvc *topicService.Service,
	convSvc *convService.Service,
	memoirSvc *memoirService.Service,
	llmSvc *llmService.Service,
	asrProvider asr.Provider,
	llmManager *llm.Manager,
	ttsProvider tts.Provider,
) *Handler {
	return &Handler{
		jwtSecret:     jwtSecret,
		userService:   userSvc,
		topicService:  topicSvc,
		convService:   convSvc,
		memoirService: memoirSvc,
		llmService:    llmSvc,
		asrProvider:   asrProvider,
		llmManager:    llmManager,
		ttsProvider:   ttsProvider,
	}
}

// HandleDialog 处理实时对话 WebSocket
func (h *Handler) HandleDialog(c *gin.Context) {
	// 从 query param 获取 token 进行认证
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
		return
	}

	// 验证 token
	userID, err := h.validateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败: " + err.Error()})
		return
	}

	// 获取对话模式
	mode := Mode(c.Query("mode"))
	topicID := c.Query("topic_id")

	// 加载用户信息
	user, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}
	if mode == "" {
		if user.ProfileCompleted {
			mode = ModeNormal
		} else {
			mode = ModeProfileCollection
		}
	}

	// 构建会话配置
	config := &SessionConfig{
		Mode:           mode,
		TopicID:        topicID,
		ConversationID: c.Query("conversation_id"),
		Speaker:        strings.TrimSpace(c.Query("speaker")),
		UserID:         userID.String(),
		UserName:       getUserDisplayName(user),
		BirthYear:      user.BirthYear,
		Hometown:       deref(user.Hometown),
		MainCity:       deref(user.MainCity),
		EraMemories:    deref(user.EraMemories),
	}

	// 支持前端直接传 topic/greeting/context（兼容当前 web 端）
	if config.Mode == ModeNormal {
		if t := c.Query("topic"); t != "" {
			config.TopicTitle = t
		}
		if g := c.Query("greeting"); g != "" {
			config.TopicGreeting = g
		}
		if ctx := c.Query("context"); ctx != "" {
			config.TopicContext = ctx
		}
	}

	// 如果是 normal 模式，加载话题信息
	if mode == ModeNormal && topicID != "" {
		topicUUID, err := uuid.Parse(topicID)
		if err == nil {
			topic, err := h.topicService.GetByIDForUser(c.Request.Context(), topicUUID, userID)
			if err == nil {
				config.TopicTitle = topic.Title
				config.TopicGreeting = deref(topic.Greeting)
				config.TopicContext = deref(topic.Context)
			}
		}
	}

	// 升级为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[Realtime] WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		return nil
	})

	log.Printf("[Realtime] 新连接: user_id=%s, mode=%s, topic_id=%s", userID, mode, topicID)

	// 创建会话
	session := NewSession(conn, config, h.asrProvider, h.llmManager, h.ttsProvider)

	// 运行会话
	if err := session.Run(); err != nil {
		log.Printf("[Realtime] 会话错误: %v", err)
	}

	log.Printf("[Realtime] 连接关闭: user_id=%s", userID)

	// 保存对话历史（在后台执行）
	go h.saveConversation(userID, config, session)
}

// HandlePreview 处理记录师音色预览
func (h *Handler) HandlePreview(c *gin.Context) {
	if h.ttsProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TTS provider not available"})
		return
	}

	text := strings.TrimSpace(c.Query("text"))
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	speaker := strings.TrimSpace(c.Query("speaker"))

	audio, err := h.ttsProvider.Synthesize(c.Request.Context(), text, tts.SynthesisConfig{
		Voice:      speaker,
		SampleRate: 24000,
		Format:     "mp3",
	})
	if err != nil {
		log.Printf("[Realtime] preview TTS failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audio": base64.StdEncoding.EncodeToString(audio),
	})
}

// saveConversation 保存对话历史并生成回忆录
func (h *Handler) saveConversation(userID uuid.UUID, config *SessionConfig, session *Session) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	messages := session.GetMessages()

	// 只有 user 和 assistant 消息，跳过 system
	var userMessages []llm.Message
	for _, msg := range messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			userMessages = append(userMessages, msg)
		}
	}

	// 如果没有对话内容，跳过保存
	if len(userMessages) < 2 {
		log.Printf("[Realtime] 对话内容不足，跳过保存: user_id=%s", userID)
		return
	}

	// Profile collection 模式不保存回忆录
	if config.Mode == ModeProfileCollection {
		log.Printf("[Realtime] Profile collection 模式，跳过保存: user_id=%s", userID)
		// TODO: 可以在这里调用 llm service 提取 profile 并更新用户信息
		return
	}

	// 优先写入前端传入的 conversation_id，避免实时会话与 REST 流程断链
	var convID uuid.UUID
	if config.ConversationID != "" {
		if parsed, err := uuid.Parse(config.ConversationID); err == nil {
			if _, err := h.convService.GetByIDForUser(ctx, parsed, userID); err == nil {
				convID = parsed
			} else {
				log.Printf("[Realtime] 指定对话不可用，回退自动创建: conv=%s err=%v", parsed, err)
			}
		}
	}

	if convID == uuid.Nil {
		conv, err := h.convService.Create(ctx, userID, &conversation.CreateConversationInput{
			Topic:    config.TopicTitle,
			Greeting: config.TopicGreeting,
			Context:  config.TopicContext,
		})
		if err != nil {
			log.Printf("[Realtime] 创建对话失败: %v", err)
			return
		}
		convID = conv.ID
	}

	// 仅保存消息；对话结束、摘要、回忆录由 /conversations/:id/end-quick 流程处理
	for _, msg := range userMessages {
		if _, err := h.convService.AddMessage(ctx, convID, msg.Role, msg.Content); err != nil {
			log.Printf("[Realtime] 保存消息失败: %v", err)
		}
	}

	log.Printf("[Realtime] 对话消息保存成功: conversation_id=%s, messages=%d", convID, len(userMessages))
}

// validateToken 验证 JWT token
func (h *Handler) validateToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			return uuid.Parse(sub)
		}
	}

	return uuid.Nil, jwt.ErrTokenInvalidClaims
}

// getUserDisplayName 获取用户显示名称
func getUserDisplayName(user interface{ DisplayName() string }) string {
	name := user.DisplayName()
	if name == "" {
		return "您"
	}
	return name
}

// deref 安全解引用
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HandleDialogFunc 返回一个 gin.HandlerFunc（用于路由注册）
func HandleDialogFunc(h *Handler) gin.HandlerFunc {
	return h.HandleDialog
}

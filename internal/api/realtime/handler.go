package realtime

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
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
	jwtSecret    string
	userService  *userService.Service
	topicService *topicService.Service
	asrProvider  asr.Provider
	llmManager   *llm.Manager
	ttsProvider  tts.Provider
}

// NewHandler 创建处理器
func NewHandler(
	jwtSecret string,
	userSvc *userService.Service,
	topicSvc *topicService.Service,
	asrProvider asr.Provider,
	llmManager *llm.Manager,
	ttsProvider tts.Provider,
) *Handler {
	return &Handler{
		jwtSecret:    jwtSecret,
		userService:  userSvc,
		topicService: topicSvc,
		asrProvider:  asrProvider,
		llmManager:   llmManager,
		ttsProvider:  ttsProvider,
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
	mode := Mode(c.DefaultQuery("mode", string(ModeNormal)))
	topicID := c.Query("topic_id")

	// 加载用户信息
	user, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	// 构建会话配置
	config := &SessionConfig{
		Mode:        mode,
		TopicID:     topicID,
		UserID:      userID.String(),
		UserName:    getUserDisplayName(user),
		BirthYear:   user.BirthYear,
		Hometown:    deref(user.Hometown),
		MainCity:    deref(user.MainCity),
		EraMemories: deref(user.EraMemories),
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

	// TODO: 保存对话历史到数据库
	// conversation := session.GetConversationText()
	// messages := session.GetMessages()
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

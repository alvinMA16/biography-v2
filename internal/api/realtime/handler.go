package realtime

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: 在生产环境中应该检查 Origin
		return true
	},
}

// HandleDialog 处理实时对话 WebSocket
func HandleDialog(c *gin.Context) {
	// 从 query param 获取 token 进行认证
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
		return
	}

	// TODO: 验证 token 并获取 user_id
	// userID, err := validateToken(token)

	// 升级为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 获取其他参数
	conversationID := c.Query("conversation_id")
	topic := c.Query("topic")

	log.Printf("[Realtime] 新连接: conversation_id=%s, topic=%s", conversationID, topic)

	// 创建会话管理器
	session := NewSession(conn, conversationID, topic)

	// 运行会话
	if err := session.Run(); err != nil {
		log.Printf("[Realtime] 会话错误: %v", err)
	}

	log.Printf("[Realtime] 连接关闭: conversation_id=%s", conversationID)
}

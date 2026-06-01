package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"sports-dashboard/internal/core/security"
	"sports-dashboard/internal/features/realtime/hub"
	"sports-dashboard/internal/shared/enums"
)

type WebSocketHandler struct {
	hub            *hub.Hub
	allowedOrigins []string
	maxMsgSize     int64
	upgrader       websocket.Upgrader
}

func NewWebSocketHandler(h *hub.Hub, allowedOrigins []string, maxMsgSize int64) *WebSocketHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return security.CheckOrigin(r, allowedOrigins)
		},
	}
	return &WebSocketHandler{
		hub:            h,
		allowedOrigins: allowedOrigins,
		maxMsgSize:     maxMsgSize,
		upgrader:       upgrader,
	}
}

func (wh *WebSocketHandler) ServeWS(c *gin.Context) {
	conn, err := wh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade to websocket", "error", err, "client_ip", c.ClientIP())
		return
	}

	client := hub.NewClient(wh.hub, conn, wh.maxMsgSize)
	if !wh.hub.RegisterClient(client) {
		_ = conn.Close()
		return
	}

	go client.WritePump()

	// Send welcome event
	welcomeData := map[string]interface{}{
		"clientId":   client.ID(),
		"serverTime": time.Now().UTC().Format(time.RFC3339),
		"version":    "1.0.0",
	}
	msg := hub.NewWebSocketMessage(string(enums.WSEventWelcome), 0, welcomeData)
	client.SendDirect(msg)

	go client.ReadPump()
}

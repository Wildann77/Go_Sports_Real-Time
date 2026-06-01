package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/realtime/handlers"
)

func SetupWebSocketRouter(r *gin.Engine, handler *handlers.WebSocketHandler) {
	r.GET("/ws", handler.ServeWS)
}

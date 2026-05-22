package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/health/handlers"
)

func SetupHealthRouter(r *gin.Engine, handler *handlers.HealthHandler) {
	r.GET("/health", handler.Live)
	r.GET("/health/live", handler.Live)
	r.GET("/health/ready", handler.Ready)
}

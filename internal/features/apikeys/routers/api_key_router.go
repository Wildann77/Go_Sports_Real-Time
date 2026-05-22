package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/apikeys/handlers"
)

func SetupAPIKeyRouter(r *gin.RouterGroup, handler *handlers.APIKeyHandler, authMiddleware gin.HandlerFunc) {
	apiKeys := r.Group("/api-keys")
	apiKeys.Use(authMiddleware)
	{
		apiKeys.POST("", handler.CreateAPIKey)
		apiKeys.GET("", handler.ListAPIKeys)
		apiKeys.DELETE("/:id", handler.RevokeAPIKey)
	}
}

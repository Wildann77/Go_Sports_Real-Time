package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/auth/handlers"
)

func SetupAuthRouter(r *gin.RouterGroup, handler *handlers.AuthHandler, authMiddleware gin.HandlerFunc) {
	r.POST("/login", handler.Login)
	r.POST("/refresh-token", handler.RefreshToken)
	r.POST("/logout", handler.Logout)

	protected := r.Group("")
	protected.Use(authMiddleware)
	{
		protected.GET("/me", handler.Me)
		protected.POST("/logout-all", handler.LogoutAll)
	}
}

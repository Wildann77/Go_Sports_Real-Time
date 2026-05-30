package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/commentary/handlers"
)

func SetupCommentaryRouter(r *gin.RouterGroup, handler *handlers.CommentaryHandler, writeAuth gin.HandlerFunc) {
	r.GET("/matches/:id/commentary", handler.GetCommentaries)
	r.POST("/matches/:id/commentary", writeAuth, handler.CreateCommentary)
}

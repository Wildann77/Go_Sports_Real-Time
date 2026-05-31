package routers

import (
	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/features/matches/handlers"
)

func SetupMatchRouter(r *gin.RouterGroup, handler *handlers.MatchHandler, writeAuth gin.HandlerFunc) {
	matches := r.Group("/matches")
	{
		matches.POST("", writeAuth, handler.CreateMatch)
		matches.GET("", handler.GetMatches)
		matches.GET("/:id", handler.GetMatch)
	}
}

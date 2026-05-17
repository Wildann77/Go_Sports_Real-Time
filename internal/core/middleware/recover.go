package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/shared/schemas"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("Panic recovered", "error", err, "stack", string(debug.Stack()))
				schemas.Error(c, http.StatusInternalServerError, "Internal Server Error", exceptions.INTERNAL_SERVER_ERROR, nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}

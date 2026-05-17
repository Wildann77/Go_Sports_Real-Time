package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		reqID := uuid.New().String()
		c.Set("request_id", reqID)

		c.Next()

		duration := time.Since(startTime)
		status := c.Writer.Status()

		slog.Info("Request handled",
			"request_id", reqID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration", duration,
			"client_ip", c.ClientIP(),
		)
	}
}

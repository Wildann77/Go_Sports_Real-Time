package exceptions

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/shared/schemas"
)

// ErrorHandlerMiddleware intercepts standard Go error responses globally
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if appErr, ok := err.(*AppError); ok {
				if appErr.Cause != nil {
					slog.Error("Application error",
						"code", appErr.Code,
						"status", appErr.StatusCode,
						"message", appErr.Message,
						"cause", appErr.Cause,
						"path", c.Request.URL.Path,
						"method", c.Request.Method,
					)
				}
				schemas.Error(c, appErr.StatusCode, appErr.Message, appErr.Code, appErr.Details)
				return
			}
			slog.Error("Unhandled error",
				"error", err,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)
			schemas.Error(c, http.StatusInternalServerError, "Internal Server Error", INTERNAL_SERVER_ERROR, nil)
		}
	}
}

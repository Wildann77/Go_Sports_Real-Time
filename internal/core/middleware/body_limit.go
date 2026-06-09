package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/shared/schemas"
)

func BodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			schemas.Error(c, http.StatusRequestEntityTooLarge, "Request body too large", exceptions.BAD_REQUEST, nil)
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		c.Next()
	}
}

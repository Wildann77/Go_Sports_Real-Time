package schemas

import "github.com/gin-gonic/gin"

type Response struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    any            `json:"data"`
	Meta    any            `json:"meta"`
	Error   *ResponseError `json:"error"`
}

type ResponseError struct {
	Code    string `json:"code"`
	Details any    `json:"details"`
}

func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    nil,
		Error:   nil,
	})
}

func SuccessWithMeta(c *gin.Context, status int, message string, data any, meta any) {
	c.JSON(status, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
		Error:   nil,
	})
}

func ErrorResponse(c *gin.Context, status int, message string, code string, details any) {
	c.JSON(status, Response{
		Success: false,
		Message: message,
		Data:    nil,
		Meta:    nil,
		Error: &ResponseError{
			Code:    code,
			Details: details,
		},
	})
}

// Error alias for compliance
func Error(c *gin.Context, status int, message string, code string, details any) {
	ErrorResponse(c, status, message, code, details)
}

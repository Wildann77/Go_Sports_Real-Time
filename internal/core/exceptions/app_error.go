package exceptions

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

const (
	VALIDATION_ERROR          = "VALIDATION_ERROR"
	BAD_REQUEST               = "BAD_REQUEST"
	NOT_FOUND                 = "NOT_FOUND"
	CONFLICT                  = "CONFLICT"
	INTERNAL_SERVER_ERROR     = "INTERNAL_SERVER_ERROR"
	DATABASE_ERROR            = "DATABASE_ERROR"
	SERVICE_UNAVAILABLE       = "SERVICE_UNAVAILABLE"
	UNAUTHORIZED              = "UNAUTHORIZED"
	FORBIDDEN                 = "FORBIDDEN"
	SECURITY_ERROR            = "SECURITY_ERROR"
	RATE_LIMITED              = "RATE_LIMITED"
	WEBSOCKET_ERROR           = "WEBSOCKET_ERROR"
	INVALID_WEBSOCKET_MESSAGE = "INVALID_WEBSOCKET_MESSAGE"
)

type AppError struct {
	Code       string
	Message    string
	StatusCode int
	Details    any
	Cause      error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func NewAppError(code, message string, statusCode int, details any) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Details:    details,
	}
}

func NewAppErrorWithCause(code, message string, statusCode int, details any, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Details:    details,
		Cause:      cause,
	}
}

func ParseValidationError(err error) *AppError {
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		var details []ValidationErrorDetail
		for _, e := range valErrs {
			message := fmt.Sprintf("Validation failed on tag '%s'", e.Tag())
			switch e.Tag() {
			case "required":
				message = "Field ini wajib diisi"
			case "min":
				message = fmt.Sprintf("Nilai minimum adalah %s", e.Param())
			case "max":
				message = fmt.Sprintf("Nilai maksimum adalah %s", e.Param())
			case "email":
				message = "Format email tidak valid"
			case "gt":
				message = fmt.Sprintf("Nilai harus lebih besar dari %s", e.Param())
			case "gte":
				message = fmt.Sprintf("Nilai harus lebih besar atau sama dengan %s", e.Param())
			case "lt":
				message = fmt.Sprintf("Nilai harus lebih kecil dari %s", e.Param())
			case "lte":
				message = fmt.Sprintf("Nilai harus lebih kecil atau sama dengan %s", e.Param())
			}
			details = append(details, ValidationErrorDetail{
				Field:   e.Field(),
				Message: message,
			})
		}
		return NewAppError(VALIDATION_ERROR, "Validation failed", http.StatusBadRequest, details)
	}

	// Fallback if it's not a go-playground/validator error
	return NewAppError(VALIDATION_ERROR, "Validation failed", http.StatusBadRequest, []ValidationErrorDetail{
		{Field: "unknown", Message: err.Error()},
	})
}

func NewValidationError(details any) *AppError {
	return NewAppError(VALIDATION_ERROR, "Validation failed", http.StatusBadRequest, details)
}

func NewNotFoundError(message string) *AppError {
	return NewAppError(NOT_FOUND, message, http.StatusNotFound, nil)
}

func NewInternalServerError() *AppError {
	return NewAppError(INTERNAL_SERVER_ERROR, "Internal server error", http.StatusInternalServerError, nil)
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(UNAUTHORIZED, message, http.StatusUnauthorized, nil)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(FORBIDDEN, message, http.StatusForbidden, nil)
}

func NewSecurityError(message string) *AppError {
	return NewAppError(SECURITY_ERROR, message, http.StatusUnauthorized, nil)
}

func NewDatabaseError(message string, cause error) *AppError {
	return NewAppErrorWithCause(DATABASE_ERROR, message, http.StatusInternalServerError, nil, cause)
}

func NewServiceUnavailableError(message string, cause error) *AppError {
	return NewAppErrorWithCause(SERVICE_UNAVAILABLE, message, http.StatusServiceUnavailable, nil, cause)
}

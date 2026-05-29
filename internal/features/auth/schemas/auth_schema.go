package schemas

import (
	"errors"
	"time"

	"sports-dashboard/internal/core/exceptions"
)

var (
	ErrMissingAuthorizationHeader = exceptions.NewUnauthorizedError("Missing Authorization header")
	ErrInvalidAuthorizationHeader = exceptions.NewUnauthorizedError("Invalid Authorization header")
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuthTokenResponse struct {
	AccessToken string        `json:"accessToken"`
	User        *UserResponse `json:"user"`
}

type AuthenticatedUser struct {
	UserID       int64
	TokenVersion int
	User         *UserResponse
}

func EnsureAuthenticatedUser(authUser *AuthenticatedUser) error {
	if authUser == nil || authUser.User == nil || authUser.UserID <= 0 {
		return exceptions.NewUnauthorizedError("Unauthorized")
	}
	return nil
}

func IsAuthorizationError(err error) bool {
	if err == nil {
		return false
	}

	var appErr *exceptions.AppError
	return errors.As(err, &appErr) && (appErr.Code == exceptions.UNAUTHORIZED || appErr.Code == exceptions.SECURITY_ERROR)
}

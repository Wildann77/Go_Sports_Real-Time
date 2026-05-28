package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/config"
	"sports-dashboard/internal/core/exceptions"
	coreMiddleware "sports-dashboard/internal/core/middleware"
	"sports-dashboard/internal/features/auth/schemas"
	"sports-dashboard/internal/features/auth/services"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type AuthService interface {
	Login(ctx context.Context, req *schemas.LoginRequest, metadata services.RequestMetadata) (*schemas.AuthTokenResponse, string, error)
	GetCurrentUser(ctx context.Context, authUser *schemas.AuthenticatedUser) (*schemas.UserResponse, error)
	RefreshToken(ctx context.Context, rawToken string, metadata services.RequestMetadata) (*schemas.AuthTokenResponse, string, error)
	LogoutCurrentDevice(ctx context.Context, rawToken string) error
	LogoutAllDevices(ctx context.Context, userID int64) error
}

type AuthHandler struct {
	service        AuthService
	refreshCookie  string
	cookieSecure   bool
	cookieDomain   string
	cookiePath     string
	cookieMaxAge   int
	cookieSameSite http.SameSite
}

func NewAuthHandler(service AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		service:        service,
		refreshCookie:  cfg.RefreshCookieName,
		cookieSecure:   cfg.RefreshCookieSecure,
		cookieDomain:   cfg.RefreshCookieDomain,
		cookiePath:     cfg.RefreshCookiePath,
		cookieMaxAge:   cfg.RefreshCookieMaxAge(),
		cookieSameSite: cfg.CookieSameSite(),
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req schemas.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(exceptions.ParseValidationError(err))
		return
	}

	response, refreshToken, err := h.service.Login(c.Request.Context(), &req, requestMetadata(c))
	if err != nil {
		c.Error(err)
		return
	}

	h.setRefreshCookie(c, refreshToken)
	globalSchemas.Success(c, http.StatusOK, "Login successful", response)
}

func (h *AuthHandler) Me(c *gin.Context) {
	authUser, ok := coreMiddleware.CurrentAuthenticatedUser(c)
	if !ok {
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
		return
	}

	response, err := h.service.GetCurrentUser(c.Request.Context(), authUser)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusOK, "Current user retrieved successfully", response)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(h.refreshCookie)
	if err != nil {
		h.clearRefreshCookie(c)
		c.Error(exceptions.NewUnauthorizedError("Refresh token cookie is required"))
		return
	}

	response, rotatedRefreshToken, err := h.service.RefreshToken(c.Request.Context(), refreshToken, requestMetadata(c))
	if err != nil {
		if schemas.IsAuthorizationError(err) {
			h.clearRefreshCookie(c)
		}
		c.Error(err)
		return
	}

	h.setRefreshCookie(c, rotatedRefreshToken)
	globalSchemas.Success(c, http.StatusOK, "Token refreshed successfully", response)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie(h.refreshCookie)
	if err == nil {
		if revokeErr := h.service.LogoutCurrentDevice(c.Request.Context(), refreshToken); revokeErr != nil {
			c.Error(revokeErr)
			return
		}
	}

	h.clearRefreshCookie(c)
	globalSchemas.Success(c, http.StatusOK, "Logout successful", nil)
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	authUser, ok := coreMiddleware.CurrentAuthenticatedUser(c)
	if !ok {
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
		return
	}

	if err := h.service.LogoutAllDevices(c.Request.Context(), authUser.UserID); err != nil {
		c.Error(err)
		return
	}

	h.clearRefreshCookie(c)
	globalSchemas.Success(c, http.StatusOK, "Logout from all devices successful", nil)
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(h.cookieSameSite)
	c.SetCookie(h.refreshCookie, token, h.cookieMaxAge, h.cookiePath, h.cookieDomain, h.cookieSecure, true)
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(h.cookieSameSite)
	c.SetCookie(h.refreshCookie, "", -1, h.cookiePath, h.cookieDomain, h.cookieSecure, true)
}

func requestMetadata(c *gin.Context) services.RequestMetadata {
	return services.RequestMetadata{
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	}
}

// Auth Handler HTTP endpoints

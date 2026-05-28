package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	coreMiddleware "sports-dashboard/internal/core/middleware"
	"sports-dashboard/internal/features/apikeys/schemas"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type APIKeyService interface {
	CreateAPIKey(ctx context.Context, userID int64, req *schemas.CreateAPIKeyRequest) (*schemas.CreateAPIKeyResponse, error)
	ListAPIKeys(ctx context.Context, userID int64) ([]*schemas.APIKeyResponse, error)
	RevokeAPIKey(ctx context.Context, userID, keyID int64) error
}

type APIKeyHandler struct {
	service APIKeyService
}

func NewAPIKeyHandler(service APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{service: service}
}

func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	authUser, ok := coreMiddleware.CurrentAuthenticatedUser(c)
	if !ok {
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
		return
	}

	var req schemas.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(exceptions.ParseValidationError(err))
		return
	}

	response, err := h.service.CreateAPIKey(c.Request.Context(), authUser.UserID, &req)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusCreated, "API key created successfully", response)
}

func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	authUser, ok := coreMiddleware.CurrentAuthenticatedUser(c)
	if !ok {
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
		return
	}

	response, err := h.service.ListAPIKeys(c.Request.Context(), authUser.UserID)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusOK, "API keys retrieved successfully", response)
}

func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	authUser, ok := coreMiddleware.CurrentAuthenticatedUser(c)
	if !ok {
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || keyID <= 0 {
		c.Error(exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "id", Message: "must be a positive integer"}}))
		return
	}

	if err := h.service.RevokeAPIKey(c.Request.Context(), authUser.UserID, keyID); err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusOK, "API key revoked successfully", nil)
}

// API Key Handler HTTP endpoints

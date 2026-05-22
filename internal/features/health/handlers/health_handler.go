package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	healthServices "sports-dashboard/internal/features/health/services"
	"sports-dashboard/internal/shared/schemas"
)

type HealthService interface {
	Liveness() healthServices.Status
	Readiness(ctx context.Context) (healthServices.Status, error)
}

type HealthHandler struct {
	service HealthService
}

func NewHealthHandler(service HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Live(c *gin.Context) {
	schemas.Success(c, http.StatusOK, "Service is alive", h.service.Liveness())
}

func (h *HealthHandler) Ready(c *gin.Context) {
	status, err := h.service.Readiness(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	schemas.Success(c, http.StatusOK, "Service is ready", status)
}

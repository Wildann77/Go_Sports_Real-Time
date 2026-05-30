package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/features/matches/schemas"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type MatchService interface {
	CreateMatch(ctx context.Context, req *schemas.CreateMatchRequest) (*schemas.MatchResponse, error)
	GetMatches(ctx context.Context, query schemas.ListMatchesQuery) ([]*schemas.MatchResponse, globalSchemas.PaginationMeta, error)
	GetMatch(ctx context.Context, id int64) (*schemas.MatchResponse, error)
}

type MatchHandler struct {
	service MatchService
}

func NewMatchHandler(service MatchService) *MatchHandler {
	return &MatchHandler{service: service}
}

func (h *MatchHandler) CreateMatch(c *gin.Context) {
	var req schemas.CreateMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(exceptions.ParseValidationError(err))
		return
	}

	res, err := h.service.CreateMatch(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusCreated, "Match created successfully", res)
}

func (h *MatchHandler) GetMatches(c *gin.Context) {
	var query schemas.ListMatchesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.Error(exceptions.ParseValidationError(err))
		return
	}

	res, meta, err := h.service.GetMatches(c.Request.Context(), query)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.SuccessWithMeta(c, http.StatusOK, "Matches retrieved successfully", res, meta)
}

func (h *MatchHandler) GetMatch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		details := []exceptions.ValidationErrorDetail{{Field: "id", Message: "must be a positive integer"}}
		c.Error(exceptions.NewValidationError(details))
		return
	}

	res, err := h.service.GetMatch(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusOK, "Match retrieved successfully", res)
}

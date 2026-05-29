package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/features/commentary/schemas"
	"sports-dashboard/internal/shared/helpers"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type CommentaryService interface {
	GetCommentaries(ctx context.Context, matchID int64, limit int) ([]*schemas.CommentaryResponse, error)
	CreateCommentary(ctx context.Context, matchID int64, req *schemas.CreateCommentaryRequest) (*schemas.CommentaryResponse, error)
}

type CommentaryHandler struct {
	service CommentaryService
}

func NewCommentaryHandler(service CommentaryService) *CommentaryHandler {
	return &CommentaryHandler{service: service}
}

func (h *CommentaryHandler) GetCommentaries(c *gin.Context) {
	idStr := c.Param("id")
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || matchID <= 0 {
		c.Error(exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "id", Message: "must be a positive integer"}}))
		return
	}

	limitStr := c.Query("limit")
	limit := helpers.ParseLimit(limitStr, 100) // Default 100 as asked by prompt

	res, err := h.service.GetCommentaries(c.Request.Context(), matchID, limit)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusOK, "Commentaries retrieved successfully", res)
}

func (h *CommentaryHandler) CreateCommentary(c *gin.Context) {
	idStr := c.Param("id")
	matchID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || matchID <= 0 {
		c.Error(exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "id", Message: "must be a positive integer"}}))
		return
	}

	var req schemas.CreateCommentaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(exceptions.ParseValidationError(err))
		return
	}

	res, err := h.service.CreateCommentary(c.Request.Context(), matchID, &req)
	if err != nil {
		c.Error(err)
		return
	}

	globalSchemas.Success(c, http.StatusCreated, "Commentary created successfully", res)
}

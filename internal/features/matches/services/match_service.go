package services

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	"sports-dashboard/internal/features/matches/models"
	"sports-dashboard/internal/features/matches/schemas"
	"sports-dashboard/internal/features/matches/utils"
	"sports-dashboard/internal/shared/enums"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type MatchRepository interface {
	Create(ctx context.Context, match *models.Match) error
	FindAll(ctx context.Context, query schemas.ListMatchesQuery) ([]*models.Match, int64, error)
	FindByID(ctx context.Context, id int64) (*models.Match, error)
}

type MatchService struct {
	repo MatchRepository
}

func NewMatchService(repo MatchRepository) *MatchService {
	return &MatchService{repo: repo}
}

func (s *MatchService) CreateMatch(ctx context.Context, req *schemas.CreateMatchRequest) (*schemas.MatchResponse, error) {
	sport := security.SanitizeSlug(req.Sport)
	if sport == "" {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "sport", Message: "must be valid slug"}})
	}

	if req.StartTime.After(req.EndTime) {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "startTime", Message: "must be before endTime"}})
	}

	metadataBytes, _ := json.Marshal(req.Metadata)
	if string(metadataBytes) == "null" {
		metadataBytes = []byte("{}")
	}

	match := &models.Match{
		Sport:     sport,
		HomeTeam:  security.SanitizeString(req.HomeTeam),
		AwayTeam:  security.SanitizeString(req.AwayTeam),
		HomeScore: req.HomeScore,
		AwayScore: req.AwayScore,
		Status:    utils.GetMatchStatus(req.StartTime, req.EndTime),
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Metadata:  datatypes.JSON(metadataBytes),
	}

	if err := s.repo.Create(ctx, match); err != nil {
		return nil, exceptions.NewAppError(exceptions.DATABASE_ERROR, "Failed to create match", 500, err.Error())
	}

	return s.mapToResponse(match), nil
}

func (s *MatchService) GetMatches(ctx context.Context, query schemas.ListMatchesQuery) ([]*schemas.MatchResponse, globalSchemas.PaginationMeta, error) {
	if query.Status != "" {
		if !enums.MatchStatus(query.Status).IsValid() && query.Status != "all" {
			return nil, globalSchemas.PaginationMeta{}, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "status", Message: "invalid status"}})
		}
	}

	matches, total, err := s.repo.FindAll(ctx, query)
	if err != nil {
		return nil, globalSchemas.PaginationMeta{}, exceptions.NewAppError(exceptions.DATABASE_ERROR, "Failed to retrieve matches", 500, err.Error())
	}

	responses := make([]*schemas.MatchResponse, len(matches))
	for i, m := range matches {
		responses[i] = s.mapToResponse(m)
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	meta := globalSchemas.PaginationMeta{
		Page:       page,
		Limit:      limit,
		Count:      len(responses),
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return responses, meta, nil
}

func (s *MatchService) GetMatch(ctx context.Context, id int64) (*schemas.MatchResponse, error) {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, exceptions.NewAppError(exceptions.DATABASE_ERROR, "Failed to retrieve match", 500, err.Error())
	}
	if match == nil {
		return nil, exceptions.NewNotFoundError("Match not found")
	}

	return s.mapToResponse(match), nil
}

func (s *MatchService) mapToResponse(m *models.Match) *schemas.MatchResponse {
	var metadata any
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &metadata)
	} else {
		metadata = map[string]interface{}{}
	}

	return &schemas.MatchResponse{
		ID:        m.ID,
		Sport:     m.Sport,
		HomeTeam:  m.HomeTeam,
		AwayTeam:  m.AwayTeam,
		HomeScore: m.HomeScore,
		AwayScore: m.AwayScore,
		Status:    m.Status,
		StartTime: m.StartTime,
		EndTime:   m.EndTime,
		Metadata:  metadata,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

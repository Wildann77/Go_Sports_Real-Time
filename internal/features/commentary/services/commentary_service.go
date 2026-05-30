package services

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	commentaryRepos "sports-dashboard/internal/features/commentary/repositories"
	commentarySchemas "sports-dashboard/internal/features/commentary/schemas"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchRepos "sports-dashboard/internal/features/matches/repositories"
	matchUtils "sports-dashboard/internal/features/matches/utils"
	"sports-dashboard/internal/features/realtime/hub"
	"sports-dashboard/internal/shared/enums"
)

type CommentaryRepository interface {
	CreateWithTx(ctx context.Context, tx *gorm.DB, c *commentaryModels.Commentary) error
	FindByMatchID(ctx context.Context, matchID int64, limit int) ([]*commentaryModels.Commentary, error)
}

type MatchRepository interface {
	FindByID(ctx context.Context, id int64) (*matchModels.Match, error)
	FindByIDForUpdateWithTx(ctx context.Context, tx *gorm.DB, id int64) (*matchModels.Match, error)
	SaveWithTx(ctx context.Context, tx *gorm.DB, match *matchModels.Match) error
}

type Broadcaster interface {
	BroadcastToRoom(roomID int64, eventType string, data any)
}

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type gormTransactionManager struct {
	db *gorm.DB
}

type CommentaryService struct {
	commentaryRepo CommentaryRepository
	matchRepo      MatchRepository
	broadcaster    Broadcaster
	txManager      TransactionManager
	timeoutPolicy  *coreDatabase.TimeoutPolicy
}

func NewCommentaryService(
	commentaryRepo *commentaryRepos.CommentaryRepository,
	matchRepo *matchRepos.MatchRepository,
	wsHub *hub.Hub,
	db *gorm.DB,
	timeoutPolicy *coreDatabase.TimeoutPolicy,
) *CommentaryService {
	return NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		wsHub,
		NewGormTransactionManager(db),
		timeoutPolicy,
	)
}

func NewCommentaryServiceWithDependencies(
	commentaryRepo CommentaryRepository,
	matchRepo MatchRepository,
	broadcaster Broadcaster,
	txManager TransactionManager,
	timeoutPolicy *coreDatabase.TimeoutPolicy,
) *CommentaryService {
	return &CommentaryService{
		commentaryRepo: commentaryRepo,
		matchRepo:      matchRepo,
		broadcaster:    broadcaster,
		txManager:      txManager,
		timeoutPolicy:  timeoutPolicy,
	}
}

func NewGormTransactionManager(db *gorm.DB) TransactionManager {
	return &gormTransactionManager{db: db}
}

func (m *gormTransactionManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}

func (s *CommentaryService) GetCommentaries(ctx context.Context, matchID int64, limit int) ([]*commentarySchemas.CommentaryResponse, error) {
	match, err := s.matchRepo.FindByID(ctx, matchID)
	if err != nil {
		return nil, exceptions.NewDatabaseError("Failed to retrieve match", err)
	}
	if match == nil {
		return nil, exceptions.NewNotFoundError("Match not found")
	}

	comments, err := s.commentaryRepo.FindByMatchID(ctx, matchID, limit)
	if err != nil {
		return nil, exceptions.NewDatabaseError("Failed to retrieve commentary", err)
	}

	responses := make([]*commentarySchemas.CommentaryResponse, len(comments))
	for i, c := range comments {
		responses[i] = s.mapToResponse(c)
	}
	return responses, nil
}

func (s *CommentaryService) CreateCommentary(ctx context.Context, matchID int64, req *commentarySchemas.CreateCommentaryRequest) (*commentarySchemas.CommentaryResponse, error) {
	eventType := security.SanitizeSlug(req.EventType)
	message := security.SanitizeString(req.Message)
	if message == "" {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "message", Message: "cannot be empty"}})
	}

	var parsedPayload commentarySchemas.ParsedPayload
	if req.Payload != nil {
		payloadBytes, _ := json.Marshal(req.Payload)
		_ = json.Unmarshal(payloadBytes, &parsedPayload)
	}

	var commentary *commentaryModels.Commentary
	var scoreUpdated bool
	var matchChanged bool
	var match *matchModels.Match
	var matchUpdateData map[string]interface{}

	txCtx, cancel := s.timeoutPolicy.WithTransactionTimeout(ctx)
	defer cancel()

	if s.txManager == nil {
		return nil, exceptions.NewDatabaseError("Failed to create commentary", fmt.Errorf("commentary service transaction manager is nil"))
	}

	err := s.txManager.WithinTransaction(txCtx, func(tx *gorm.DB) error {
		var err error
		match, err = s.matchRepo.FindByIDForUpdateWithTx(txCtx, tx, matchID)
		if err != nil {
			return exceptions.NewDatabaseError("Failed to lock match for commentary update", err)
		}
		if match == nil {
			return exceptions.NewNotFoundError("Match not found")
		}

		previousStatus := match.Status

		payloadBytes, _ := json.Marshal(req.Payload)
		if string(payloadBytes) == "null" {
			payloadBytes = []byte("{}")
		}

		commentary = &commentaryModels.Commentary{
			MatchID:   matchID,
			Minute:    req.Minute,
			EventType: eventType,
			Message:   message,
			Payload:   datatypes.JSON(payloadBytes),
		}

		if err := s.commentaryRepo.CreateWithTx(txCtx, tx, commentary); err != nil {
			return exceptions.NewDatabaseError("Failed to save commentary", err)
		}

		if parsedPayload.HomeScore != nil && *parsedPayload.HomeScore >= 0 {
			match.HomeScore = *parsedPayload.HomeScore
			scoreUpdated = true
		}
		if parsedPayload.AwayScore != nil && *parsedPayload.AwayScore >= 0 {
			match.AwayScore = *parsedPayload.AwayScore
			scoreUpdated = true
		}

		match.Status = matchUtils.GetMatchStatus(match.StartTime, match.EndTime)

		matchChanged = scoreUpdated || previousStatus != match.Status
		if matchChanged {
			if err := s.matchRepo.SaveWithTx(txCtx, tx, match); err != nil {
				return exceptions.NewDatabaseError("Failed to persist match state after commentary update", err)
			}
		}

		return nil
	})

	if err != nil {
		if _, ok := err.(*exceptions.AppError); !ok {
			return nil, exceptions.NewDatabaseError("Failed to create commentary", fmt.Errorf("commentary service transaction: %w", err))
		}
		return nil, err
	}

	res := s.mapToResponse(commentary)

	s.broadcaster.BroadcastToRoom(matchID, string(enums.WSEventCommentaryCreated), res)

	if matchChanged {
		matchUpdateData = map[string]interface{}{
			"homeScore": match.HomeScore,
			"awayScore": match.AwayScore,
			"status":    match.Status,
		}
		s.broadcaster.BroadcastToRoom(matchID, string(enums.WSEventMatchUpdated), matchUpdateData)
	}

	return res, nil
}

func (s *CommentaryService) mapToResponse(c *commentaryModels.Commentary) *commentarySchemas.CommentaryResponse {
	var payload any
	if len(c.Payload) > 0 {
		_ = json.Unmarshal(c.Payload, &payload)
	} else {
		payload = map[string]interface{}{}
	}

	return &commentarySchemas.CommentaryResponse{
		ID:        c.ID,
		MatchID:   c.MatchID,
		Minute:    c.Minute,
		EventType: c.EventType,
		Message:   c.Message,
		Payload:   payload,
		CreatedAt: c.CreatedAt,
	}
}

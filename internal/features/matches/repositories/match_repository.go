package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/features/matches/models"
	"sports-dashboard/internal/features/matches/schemas"
)

type MatchRepository struct {
	db            *gorm.DB
	timeoutPolicy *coreDatabase.TimeoutPolicy
}

func NewMatchRepository(db *gorm.DB, timeoutPolicy *coreDatabase.TimeoutPolicy) *MatchRepository {
	return &MatchRepository{db: db, timeoutPolicy: timeoutPolicy}
}

func (r *MatchRepository) Create(ctx context.Context, match *models.Match) error {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()
	if err := r.db.WithContext(ctx).Create(match).Error; err != nil {
		return fmt.Errorf("match repository create: %w", err)
	}
	return nil
}

func (r *MatchRepository) FindAll(ctx context.Context, listQuery schemas.ListMatchesQuery) ([]*models.Match, int64, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var matches []*models.Match
	dbQuery := r.db.WithContext(ctx).Model(&models.Match{})

	if listQuery.Status != "" && listQuery.Status != "all" {
		dbQuery = dbQuery.Where("status = ?", listQuery.Status)
	}
	if listQuery.Sport != "" {
		dbQuery = dbQuery.Where("sport ILIKE ?", "%"+listQuery.Sport+"%")
	}
	if listQuery.Team != "" {
		dbQuery = dbQuery.Where("home_team ILIKE ? OR away_team ILIKE ?", "%"+listQuery.Team+"%", "%"+listQuery.Team+"%")
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("match repository find all count: %w", err)
	}

	var orderBy string
	switch listQuery.Sort {
	case "date_asc":
		orderBy = "start_time ASC"
	case "date_desc":
		orderBy = "start_time DESC"
	case "status":
		orderBy = "CASE WHEN status = 'live' THEN 1 WHEN status = 'scheduled' THEN 2 WHEN status = 'finished' THEN 3 ELSE 4 END ASC, start_time DESC"
	case "team":
		orderBy = "home_team ASC, away_team ASC"
	default:
		orderBy = "start_time DESC"
	}
	dbQuery = dbQuery.Order(orderBy)

	page := listQuery.Page
	if page <= 0 {
		page = 1
	}
	limit := listQuery.Limit
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit
	dbQuery = dbQuery.Limit(limit).Offset(offset)

	if err := dbQuery.Find(&matches).Error; err != nil {
		return nil, 0, fmt.Errorf("match repository find all query: %w", err)
	}
	return matches, total, nil
}

func (r *MatchRepository) FindByID(ctx context.Context, id int64) (*models.Match, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var match models.Match
	err := r.db.WithContext(ctx).First(&match, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Return nil when not found to maintain existing behavior
		}
		return nil, fmt.Errorf("match repository find by id: %w", err)
	}
	return &match, nil
}

func (r *MatchRepository) FindByIDForUpdateWithTx(ctx context.Context, tx *gorm.DB, id int64) (*models.Match, error) {
	var match models.Match
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&match, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("match repository find by id for update: %w", err)
	}
	return &match, nil
}

func (r *MatchRepository) SaveWithTx(ctx context.Context, tx *gorm.DB, match *models.Match) error {
	if err := tx.WithContext(ctx).Save(match).Error; err != nil {
		return fmt.Errorf("match repository save with tx: %w", err)
	}
	return nil
}

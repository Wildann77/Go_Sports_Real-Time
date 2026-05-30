package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/features/commentary/models"
)

type CommentaryRepository struct {
	db            *gorm.DB
	timeoutPolicy *coreDatabase.TimeoutPolicy
}

func NewCommentaryRepository(db *gorm.DB, timeoutPolicy *coreDatabase.TimeoutPolicy) *CommentaryRepository {
	return &CommentaryRepository{db: db, timeoutPolicy: timeoutPolicy}
}

func (r *CommentaryRepository) CreateWithTx(ctx context.Context, tx *gorm.DB, c *models.Commentary) error {
	if err := tx.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("commentary repository create with tx: %w", err)
	}
	return nil
}

func (r *CommentaryRepository) FindByMatchID(ctx context.Context, matchID int64, limit int) ([]*models.Commentary, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var commentaries []*models.Commentary
	err := r.db.WithContext(ctx).
		Where("match_id = ?", matchID).
		Order("created_at ASC").
		Limit(limit).
		Find(&commentaries).Error
	if err != nil {
		return nil, fmt.Errorf("commentary repository find by match id: %w", err)
	}
	return commentaries, nil
}

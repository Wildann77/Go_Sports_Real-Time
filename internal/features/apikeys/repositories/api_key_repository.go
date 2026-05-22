package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/features/apikeys/models"
)

type APIKeyRepository struct {
	db            *gorm.DB
	timeoutPolicy *coreDatabase.TimeoutPolicy
}

func NewAPIKeyRepository(db *gorm.DB, timeoutPolicy *coreDatabase.TimeoutPolicy) *APIKeyRepository {
	return &APIKeyRepository{db: db, timeoutPolicy: timeoutPolicy}
}

func (r *APIKeyRepository) Create(ctx context.Context, apiKey *models.APIKey) error {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	if err := r.db.WithContext(ctx).Create(apiKey).Error; err != nil {
		return fmt.Errorf("api key repository create: %w", err)
	}

	return nil
}

func (r *APIKeyRepository) ListByUserID(ctx context.Context, userID int64) ([]*models.APIKey, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var apiKeys []*models.APIKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&apiKeys).Error; err != nil {
		return nil, fmt.Errorf("api key repository list by user id: %w", err)
	}

	return apiKeys, nil
}

func (r *APIKeyRepository) FindByIDAndUserID(ctx context.Context, keyID, userID int64) (*models.APIKey, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var apiKey models.APIKey
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", keyID, userID).
		First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("api key repository find by id and user id: %w", err)
	}

	return &apiKey, nil
}

func (r *APIKeyRepository) FindActiveByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var apiKey models.APIKey
	err := r.db.WithContext(ctx).
		Where("key_hash = ? AND revoked_at IS NULL", keyHash).
		First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("api key repository find active by hash: %w", err)
	}

	return &apiKey, nil
}

func (r *APIKeyRepository) UpdateLastUsedAt(ctx context.Context, keyID int64, usedAt time.Time) error {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	result := r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ? AND revoked_at IS NULL", keyID).
		Updates(map[string]any{
			"last_used_at": usedAt,
			"updated_at":   usedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("api key repository update last used at: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("api key repository update last used at: %w", gorm.ErrRecordNotFound)
	}

	return nil
}

func (r *APIKeyRepository) RevokeByIDAndUserID(ctx context.Context, keyID, userID int64, revokedAt time.Time) error {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	if err := r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", keyID, userID).
		Updates(map[string]any{
			"revoked_at": revokedAt,
			"updated_at": revokedAt,
		}).Error; err != nil {
		return fmt.Errorf("api key repository revoke by id and user id: %w", err)
	}

	return nil
}

// API Key Repository persistence actions

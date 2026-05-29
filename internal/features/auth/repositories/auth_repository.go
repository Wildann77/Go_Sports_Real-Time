package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/features/auth/models"
)

type AuthRepository struct {
	db            *gorm.DB
	timeoutPolicy *coreDatabase.TimeoutPolicy
}

func NewAuthRepository(db *gorm.DB, timeoutPolicy *coreDatabase.TimeoutPolicy) *AuthRepository {
	return &AuthRepository{db: db, timeoutPolicy: timeoutPolicy}
}

func (r *AuthRepository) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var user models.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("auth repository find user by email: %w", err)
	}

	return &user, nil
}

func (r *AuthRepository) FindUserByID(ctx context.Context, userID int64) (*models.User, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var user models.User
	err := r.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("auth repository find user by id: %w", err)
	}

	return &user, nil
}

func (r *AuthRepository) FindUserByIDForUpdateWithTx(ctx context.Context, tx *gorm.DB, userID int64) (*models.User, error) {
	var user models.User
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&user, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("auth repository find user by id for update: %w", err)
	}

	return &user, nil
}

func (r *AuthRepository) CreateRefreshSession(ctx context.Context, session *models.RefreshSession) error {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("auth repository create refresh session: %w", err)
	}

	return nil
}

func (r *AuthRepository) CreateRefreshSessionWithTx(ctx context.Context, tx *gorm.DB, session *models.RefreshSession) error {
	if err := tx.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("auth repository create refresh session with tx: %w", err)
	}

	return nil
}

func (r *AuthRepository) FindRefreshSessionByJTI(ctx context.Context, jti string) (*models.RefreshSession, error) {
	ctx, cancel := r.timeoutPolicy.WithQueryTimeout(ctx)
	defer cancel()

	var session models.RefreshSession
	err := r.db.WithContext(ctx).Where("jti = ?", jti).First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("auth repository find refresh session by jti: %w", err)
	}

	return &session, nil
}

func (r *AuthRepository) FindRefreshSessionByJTIForUpdateWithTx(ctx context.Context, tx *gorm.DB, jti string) (*models.RefreshSession, error) {
	var session models.RefreshSession
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("jti = ?", jti).
		First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("auth repository find refresh session by jti for update: %w", err)
	}

	return &session, nil
}

func (r *AuthRepository) RevokeRefreshSessionByIDWithTx(ctx context.Context, tx *gorm.DB, sessionID int64, revokedAt time.Time, replacedBy *int64) error {
	updates := map[string]any{
		"revoked_at": revokedAt,
		"updated_at": revokedAt,
	}
	if replacedBy != nil {
		updates["replaced_by"] = *replacedBy
	}

	if err := tx.WithContext(ctx).
		Model(&models.RefreshSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("auth repository revoke refresh session by id with tx: %w", err)
	}

	return nil
}

func (r *AuthRepository) RevokeFamilySessionsWithTx(ctx context.Context, tx *gorm.DB, userID int64, familyID string, revokedAt time.Time) error {
	if err := tx.WithContext(ctx).
		Model(&models.RefreshSession{}).
		Where("user_id = ? AND family_id = ? AND revoked_at IS NULL", userID, familyID).
		Updates(map[string]any{
			"revoked_at": revokedAt,
			"updated_at": revokedAt,
		}).Error; err != nil {
		return fmt.Errorf("auth repository revoke family sessions with tx: %w", err)
	}

	return nil
}

func (r *AuthRepository) RevokeAllUserSessionsWithTx(ctx context.Context, tx *gorm.DB, userID int64, revokedAt time.Time) error {
	if err := tx.WithContext(ctx).
		Model(&models.RefreshSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{
			"revoked_at": revokedAt,
			"updated_at": revokedAt,
		}).Error; err != nil {
		return fmt.Errorf("auth repository revoke all user sessions with tx: %w", err)
	}

	return nil
}

func (r *AuthRepository) IncrementUserTokenVersionWithTx(ctx context.Context, tx *gorm.DB, userID int64) error {
	if err := tx.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return fmt.Errorf("auth repository increment user token version with tx: %w", err)
	}

	return nil
}

// Auth Repository persistence actions

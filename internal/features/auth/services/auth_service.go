package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	"sports-dashboard/internal/features/auth/models"
	"sports-dashboard/internal/features/auth/repositories"
	"sports-dashboard/internal/features/auth/schemas"
	"sports-dashboard/internal/features/auth/utils"
)

type AuthRepository interface {
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	FindUserByID(ctx context.Context, userID int64) (*models.User, error)
	FindUserByIDForUpdateWithTx(ctx context.Context, tx *gorm.DB, userID int64) (*models.User, error)
	CreateRefreshSession(ctx context.Context, session *models.RefreshSession) error
	CreateRefreshSessionWithTx(ctx context.Context, tx *gorm.DB, session *models.RefreshSession) error
	FindRefreshSessionByJTI(ctx context.Context, jti string) (*models.RefreshSession, error)
	FindRefreshSessionByJTIForUpdateWithTx(ctx context.Context, tx *gorm.DB, jti string) (*models.RefreshSession, error)
	RevokeRefreshSessionByIDWithTx(ctx context.Context, tx *gorm.DB, sessionID int64, revokedAt time.Time, replacedBy *int64) error
	RevokeFamilySessionsWithTx(ctx context.Context, tx *gorm.DB, userID int64, familyID string, revokedAt time.Time) error
	RevokeAllUserSessionsWithTx(ctx context.Context, tx *gorm.DB, userID int64, revokedAt time.Time) error
	IncrementUserTokenVersionWithTx(ctx context.Context, tx *gorm.DB, userID int64) error
}

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type gormTransactionManager struct {
	db *gorm.DB
}

type RequestMetadata struct {
	UserAgent string
	IPAddress string
}

type AuthService struct {
	repo          AuthRepository
	txManager     TransactionManager
	timeoutPolicy *coreDatabase.TimeoutPolicy
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewAuthService(repo *repositories.AuthRepository, db *gorm.DB, cfg *config.Config, timeoutPolicy *coreDatabase.TimeoutPolicy) *AuthService {
	return &AuthService{
		repo:          repo,
		txManager:     NewGormTransactionManager(db),
		timeoutPolicy: timeoutPolicy,
		accessSecret:  cfg.JWTAccessSecret,
		refreshSecret: cfg.JWTRefreshSecret,
		accessTTL:     cfg.AccessTokenTTL(),
		refreshTTL:    cfg.RefreshTokenTTL(),
	}
}

func NewGormTransactionManager(db *gorm.DB) TransactionManager {
	return &gormTransactionManager{db: db}
}

func (m *gormTransactionManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}

func (s *AuthService) Login(ctx context.Context, req *schemas.LoginRequest, metadata RequestMetadata) (*schemas.AuthTokenResponse, string, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, "", err
	}

	email := sanitizeEmail(req.Email)
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, "", exceptions.NewDatabaseError("Failed to retrieve user", err)
	}
	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, "", exceptions.NewUnauthorizedError("Invalid email or password")
	}

	now := time.Now().UTC()
	accessToken, err := s.generateAccessToken(user, now)
	if err != nil {
		return nil, "", err
	}

	session, refreshToken, err := s.buildRefreshSession(user, metadata, "", now)
	if err != nil {
		return nil, "", err
	}

	if err := s.repo.CreateRefreshSession(ctx, session); err != nil {
		return nil, "", exceptions.NewDatabaseError("Failed to save refresh session", err)
	}

	return &schemas.AuthTokenResponse{
		AccessToken: accessToken,
		User:        s.mapUser(user),
	}, refreshToken, nil
}

func (s *AuthService) GetCurrentUser(ctx context.Context, authUser *schemas.AuthenticatedUser) (*schemas.UserResponse, error) {
	if err := schemas.EnsureAuthenticatedUser(authUser); err != nil {
		return nil, err
	}

	return authUser.User, nil
}

func (s *AuthService) VerifyAccessToken(ctx context.Context, rawToken string) (*schemas.AuthenticatedUser, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}

	claims, err := utils.ParseAndVerifyToken(rawToken, s.accessSecret, utils.TokenTypeAccess, time.Now().UTC())
	if err != nil {
		return nil, s.mapTokenError(err, "Invalid or expired access token")
	}

	user, err := s.repo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, exceptions.NewDatabaseError("Failed to retrieve user", err)
	}
	if user == nil || user.TokenVersion != claims.TokenVersion {
		return nil, exceptions.NewUnauthorizedError("Invalid or expired access token")
	}

	return &schemas.AuthenticatedUser{
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		User:         s.mapUser(user),
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, rawToken string, metadata RequestMetadata) (*schemas.AuthTokenResponse, string, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, "", err
	}

	claims, err := utils.ParseAndVerifyToken(rawToken, s.refreshSecret, utils.TokenTypeRefresh, time.Now().UTC())
	if err != nil {
		return nil, "", s.mapTokenError(err, "Invalid or expired refresh token")
	}

	var response *schemas.AuthTokenResponse
	var newRefreshToken string

	txCtx, cancel := s.timeoutPolicy.WithTransactionTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	err = s.txManager.WithinTransaction(txCtx, func(tx *gorm.DB) error {
		user, err := s.repo.FindUserByIDForUpdateWithTx(txCtx, tx, claims.UserID)
		if err != nil {
			return exceptions.NewDatabaseError("Failed to lock user", err)
		}
		if user == nil {
			return exceptions.NewUnauthorizedError("Invalid or expired refresh token")
		}

		session, err := s.repo.FindRefreshSessionByJTIForUpdateWithTx(txCtx, tx, claims.JTI)
		if err != nil {
			return exceptions.NewDatabaseError("Failed to lock refresh session", err)
		}
		if session == nil || session.UserID != claims.UserID || session.FamilyID != claims.FamilyID || !utils.CompareTokenHash(rawToken, session.TokenHash) {
			return exceptions.NewUnauthorizedError("Invalid or expired refresh token")
		}

		if session.RevokedAt != nil {
			if user.TokenVersion == claims.TokenVersion {
				if err := s.repo.RevokeFamilySessionsWithTx(txCtx, tx, user.ID, session.FamilyID, now); err != nil {
					return exceptions.NewDatabaseError("Failed to revoke refresh token family", err)
				}
				if err := s.repo.IncrementUserTokenVersionWithTx(txCtx, tx, user.ID); err != nil {
					return exceptions.NewDatabaseError("Failed to revoke compromised access tokens", err)
				}
			}
			return exceptions.NewSecurityError("Refresh token reuse detected")
		}

		if now.After(session.ExpiresAt) || user.TokenVersion != claims.TokenVersion {
			return exceptions.NewUnauthorizedError("Invalid or expired refresh token")
		}

		newSession, issuedRefreshToken, err := s.buildRefreshSession(user, metadata, session.FamilyID, now)
		if err != nil {
			return err
		}

		if err := s.repo.CreateRefreshSessionWithTx(txCtx, tx, newSession); err != nil {
			return exceptions.NewDatabaseError("Failed to create rotated refresh session", err)
		}

		replacedBy := newSession.ID
		if err := s.repo.RevokeRefreshSessionByIDWithTx(txCtx, tx, session.ID, now, &replacedBy); err != nil {
			return exceptions.NewDatabaseError("Failed to revoke rotated refresh session", err)
		}

		accessToken, err := s.generateAccessToken(user, now)
		if err != nil {
			return err
		}

		response = &schemas.AuthTokenResponse{
			AccessToken: accessToken,
			User:        s.mapUser(user),
		}
		newRefreshToken = issuedRefreshToken

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return response, newRefreshToken, nil
}

func (s *AuthService) LogoutCurrentDevice(ctx context.Context, rawToken string) error {
	if err := s.ensureConfigured(); err != nil {
		return err
	}

	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}

	claims, err := utils.ParseAndVerifyTokenAllowExpired(rawToken, s.refreshSecret, utils.TokenTypeRefresh, time.Now().UTC())
	if err != nil {
		return nil
	}

	txCtx, cancel := s.timeoutPolicy.WithTransactionTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	return s.txManager.WithinTransaction(txCtx, func(tx *gorm.DB) error {
		session, err := s.repo.FindRefreshSessionByJTIForUpdateWithTx(txCtx, tx, claims.JTI)
		if err != nil {
			return exceptions.NewDatabaseError("Failed to lock refresh session", err)
		}
		if session == nil || session.UserID != claims.UserID || !utils.CompareTokenHash(rawToken, session.TokenHash) || session.RevokedAt != nil {
			return nil
		}

		if err := s.repo.RevokeRefreshSessionByIDWithTx(txCtx, tx, session.ID, now, nil); err != nil {
			return exceptions.NewDatabaseError("Failed to revoke refresh session", err)
		}

		return nil
	})
}

func (s *AuthService) LogoutAllDevices(ctx context.Context, userID int64) error {
	if err := s.ensureConfigured(); err != nil {
		return err
	}

	txCtx, cancel := s.timeoutPolicy.WithTransactionTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	return s.txManager.WithinTransaction(txCtx, func(tx *gorm.DB) error {
		user, err := s.repo.FindUserByIDForUpdateWithTx(txCtx, tx, userID)
		if err != nil {
			return exceptions.NewDatabaseError("Failed to lock user", err)
		}
		if user == nil {
			return exceptions.NewUnauthorizedError("Unauthorized")
		}

		if err := s.repo.RevokeAllUserSessionsWithTx(txCtx, tx, user.ID, now); err != nil {
			return exceptions.NewDatabaseError("Failed to revoke refresh sessions", err)
		}
		if err := s.repo.IncrementUserTokenVersionWithTx(txCtx, tx, user.ID); err != nil {
			return exceptions.NewDatabaseError("Failed to revoke access tokens", err)
		}

		return nil
	})
}

func (s *AuthService) buildRefreshSession(user *models.User, metadata RequestMetadata, familyID string, now time.Time) (*models.RefreshSession, string, error) {
	if familyID == "" {
		familyID = uuid.NewString()
	}

	refreshToken, claims, err := utils.GenerateToken(utils.TokenOptions{
		Secret:       s.refreshSecret,
		TTL:          s.refreshTTL,
		TokenType:    utils.TokenTypeRefresh,
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		JTI:          uuid.NewString(),
		FamilyID:     familyID,
		Now:          now,
	})
	if err != nil {
		return nil, "", s.wrapTokenConfigError("Failed to issue refresh token", err)
	}

	return &models.RefreshSession{
		UserID:    user.ID,
		TokenHash: utils.HashToken(refreshToken),
		JTI:       claims.JTI,
		FamilyID:  claims.FamilyID,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
		UserAgent: security.SanitizeString(metadata.UserAgent),
		IPAddress: security.SanitizeString(metadata.IPAddress),
	}, refreshToken, nil
}

func (s *AuthService) generateAccessToken(user *models.User, now time.Time) (string, error) {
	accessToken, _, err := utils.GenerateToken(utils.TokenOptions{
		Secret:       s.accessSecret,
		TTL:          s.accessTTL,
		TokenType:    utils.TokenTypeAccess,
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		Now:          now,
	})
	if err != nil {
		return "", s.wrapTokenConfigError("Failed to issue access token", err)
	}

	return accessToken, nil
}

func (s *AuthService) mapUser(user *models.User) *schemas.UserResponse {
	return &schemas.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (s *AuthService) ensureConfigured() error {
	if strings.TrimSpace(s.accessSecret) == "" || strings.TrimSpace(s.refreshSecret) == "" || s.accessTTL <= 0 || s.refreshTTL <= 0 {
		return exceptions.NewServiceUnavailableError("Authentication is not configured", errors.New("missing auth secrets or token TTL configuration"))
	}
	return nil
}

func (s *AuthService) mapTokenError(err error, fallbackMessage string) error {
	switch {
	case errors.Is(err, utils.ErrExpiredToken), errors.Is(err, utils.ErrInvalidToken), errors.Is(err, utils.ErrTokenTypeMismatch):
		return exceptions.NewUnauthorizedError(fallbackMessage)
	case errors.Is(err, utils.ErrTokenConfig):
		return exceptions.NewServiceUnavailableError("Authentication is not configured", err)
	default:
		return exceptions.NewAppErrorWithCause(exceptions.INTERNAL_SERVER_ERROR, fallbackMessage, 500, nil, err)
	}
}

func (s *AuthService) wrapTokenConfigError(message string, err error) error {
	if errors.Is(err, utils.ErrTokenConfig) {
		return exceptions.NewServiceUnavailableError("Authentication is not configured", err)
	}
	return exceptions.NewAppErrorWithCause(exceptions.INTERNAL_SERVER_ERROR, message, 500, nil, err)
}

func sanitizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

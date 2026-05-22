package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"sports-dashboard/internal/core/config"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	"sports-dashboard/internal/features/apikeys/models"
	"sports-dashboard/internal/features/apikeys/repositories"
	"sports-dashboard/internal/features/apikeys/schemas"
)

type APIKeyRepository interface {
	Create(ctx context.Context, apiKey *models.APIKey) error
	ListByUserID(ctx context.Context, userID int64) ([]*models.APIKey, error)
	FindByIDAndUserID(ctx context.Context, keyID, userID int64) (*models.APIKey, error)
	FindActiveByHash(ctx context.Context, keyHash string) (*models.APIKey, error)
	UpdateLastUsedAt(ctx context.Context, keyID int64, usedAt time.Time) error
	RevokeByIDAndUserID(ctx context.Context, keyID, userID int64, revokedAt time.Time) error
}

type APIKeyService struct {
	repo   APIKeyRepository
	appEnv string
}

func NewAPIKeyService(repo *repositories.APIKeyRepository, cfg *config.Config) *APIKeyService {
	return &APIKeyService{
		repo:   repo,
		appEnv: cfg.AppEnv,
	}
}

func NewAPIKeyServiceWithRepository(repo APIKeyRepository, appEnv string) *APIKeyService {
	return &APIKeyService{
		repo:   repo,
		appEnv: appEnv,
	}
}

func (s *APIKeyService) CreateAPIKey(ctx context.Context, userID int64, req *schemas.CreateAPIKeyRequest) (*schemas.CreateAPIKeyResponse, error) {
	if userID <= 0 {
		return nil, exceptions.NewUnauthorizedError("Unauthorized")
	}

	name := security.SanitizeString(req.Name)
	if name == "" {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "name", Message: "cannot be empty"}})
	}

	scopes, err := validateScopes(req.Scopes)
	if err != nil {
		return nil, err
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now().UTC()) {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "expiresAt", Message: "must be in the future"}})
	}

	rawKey, keyPrefix, keyLastFour, keyHash, err := generateAPIKey(s.appEnv)
	if err != nil {
		return nil, exceptions.NewAppErrorWithCause(exceptions.INTERNAL_SERVER_ERROR, "Failed to generate API key", 500, nil, err)
	}

	apiKey := &models.APIKey{
		UserID:      userID,
		Name:        name,
		KeyPrefix:   keyPrefix,
		KeyHash:     keyHash,
		KeyLastFour: keyLastFour,
		Scopes:      scopes,
		ExpiresAt:   req.ExpiresAt,
	}
	if err := s.repo.Create(ctx, apiKey); err != nil {
		return nil, exceptions.NewDatabaseError("Failed to save API key", err)
	}

	return &schemas.CreateAPIKeyResponse{
		APIKey: rawKey,
		Key:    mapAPIKeyResponse(apiKey),
	}, nil
}

func (s *APIKeyService) ListAPIKeys(ctx context.Context, userID int64) ([]*schemas.APIKeyResponse, error) {
	if userID <= 0 {
		return nil, exceptions.NewUnauthorizedError("Unauthorized")
	}

	apiKeys, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, exceptions.NewDatabaseError("Failed to retrieve API keys", err)
	}

	responses := make([]*schemas.APIKeyResponse, len(apiKeys))
	for i, apiKey := range apiKeys {
		responses[i] = mapAPIKeyResponse(apiKey)
	}

	return responses, nil
}

func (s *APIKeyService) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	if userID <= 0 {
		return exceptions.NewUnauthorizedError("Unauthorized")
	}

	apiKey, err := s.repo.FindByIDAndUserID(ctx, keyID, userID)
	if err != nil {
		return exceptions.NewDatabaseError("Failed to retrieve API key", err)
	}
	if apiKey == nil {
		return exceptions.NewNotFoundError("API key not found")
	}
	if apiKey.RevokedAt != nil {
		return nil
	}

	if err := s.repo.RevokeByIDAndUserID(ctx, keyID, userID, time.Now().UTC()); err != nil {
		return exceptions.NewDatabaseError("Failed to revoke API key", err)
	}

	return nil
}

func (s *APIKeyService) VerifyAPIKey(ctx context.Context, rawToken, requiredScope string) (*schemas.AuthenticatedAPIKey, error) {
	token := strings.TrimSpace(rawToken)
	if !schemas.IsAPIKeyToken(token) {
		return nil, exceptions.NewUnauthorizedError("Invalid or expired API key")
	}

	apiKey, err := s.repo.FindActiveByHash(ctx, hashKey(token))
	if err != nil {
		return nil, exceptions.NewDatabaseError("Failed to verify API key", err)
	}
	if apiKey == nil {
		return nil, exceptions.NewUnauthorizedError("Invalid or expired API key")
	}

	now := time.Now().UTC()
	if apiKey.ExpiresAt != nil && now.After(*apiKey.ExpiresAt) {
		return nil, exceptions.NewUnauthorizedError("Invalid or expired API key")
	}
	if !hasScope(apiKey.Scopes, requiredScope) {
		return nil, exceptions.NewForbiddenError("API key does not have required scope")
	}

	if err := s.repo.UpdateLastUsedAt(ctx, apiKey.ID, now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, exceptions.NewUnauthorizedError("Invalid or expired API key")
		}
		return nil, exceptions.NewDatabaseError("Failed to update API key usage", err)
	}

	return &schemas.AuthenticatedAPIKey{
		KeyID:      apiKey.ID,
		UserID:     apiKey.UserID,
		Name:       apiKey.Name,
		Scopes:     append([]string(nil), apiKey.Scopes...),
		LastUsedAt: &now,
		ExpiresAt:  apiKey.ExpiresAt,
	}, nil
}

func mapAPIKeyResponse(apiKey *models.APIKey) *schemas.APIKeyResponse {
	return &schemas.APIKeyResponse{
		ID:          apiKey.ID,
		Name:        apiKey.Name,
		KeyPrefix:   apiKey.KeyPrefix,
		KeyLastFour: apiKey.KeyLastFour,
		Scopes:      append([]string(nil), apiKey.Scopes...),
		LastUsedAt:  apiKey.LastUsedAt,
		ExpiresAt:   apiKey.ExpiresAt,
		RevokedAt:   apiKey.RevokedAt,
		CreatedAt:   apiKey.CreatedAt,
		UpdatedAt:   apiKey.UpdatedAt,
	}
}

func validateScopes(rawScopes []string) ([]string, error) {
	if len(rawScopes) == 0 {
		return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "scopes", Message: "must contain at least one scope"}})
	}

	allowedScopes := schemas.AllowedScopes()
	seen := make(map[string]struct{}, len(rawScopes))
	normalized := make([]string, 0, len(rawScopes))

	for _, rawScope := range rawScopes {
		scope := strings.TrimSpace(rawScope)
		if scope == "" {
			return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "scopes", Message: "contains empty scope"}})
		}
		if _, ok := allowedScopes[scope]; !ok {
			return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "scopes", Message: "contains invalid scope"}})
		}
		if _, exists := seen[scope]; exists {
			return nil, exceptions.NewValidationError([]exceptions.ValidationErrorDetail{{Field: "scopes", Message: "contains duplicate scope"}})
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}

	return normalized, nil
}

func generateAPIKey(appEnv string) (rawKey string, keyPrefix string, keyLastFour string, keyHash string, err error) {
	keyPrefix = "sk_test"
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		keyPrefix = "sk_live"
	}

	secretBytes := make([]byte, 32)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", "", err
	}

	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	rawKey = keyPrefix + "_" + secret
	keyLastFour = rawKey[len(rawKey)-4:]
	keyHash = hashKey(rawKey)
	return rawKey, keyPrefix, keyLastFour, keyHash, nil
}

func hashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

func hasScope(scopes []string, requiredScope string) bool {
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}

	return false
}

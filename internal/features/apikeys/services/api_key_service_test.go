package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/features/apikeys/models"
	"sports-dashboard/internal/features/apikeys/schemas"
)

type fakeAPIKeyRepository struct {
	keysByID            map[int64]*models.APIKey
	keysByHash          map[string]*models.APIKey
	nextID              int64
	lastCreated         *models.APIKey
	updateLastUsedKeyID int64
	updateLastUsedValue time.Time
	createErr           error
	listErr             error
	findByIDErr         error
	findActiveErr       error
	updateLastUsedErr   error
	revokeErr           error
}

func TestAPIKeyServiceCreateAPIKeyStoresHashOnly(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	service := NewAPIKeyServiceWithRepository(repo, "test")

	response, err := service.CreateAPIKey(context.Background(), 42, &schemas.CreateAPIKeyRequest{
		Name:   " Match Writer ",
		Scopes: []string{schemas.ScopeMatchesWrite},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.APIKey == "" || !strings.HasPrefix(response.APIKey, "sk_test_") {
		t.Fatalf("expected test api key, got %q", response.APIKey)
	}
	if repo.lastCreated == nil {
		t.Fatal("expected repository create call")
	}
	if repo.lastCreated.KeyHash == response.APIKey {
		t.Fatal("expected raw API key not to be stored")
	}
	if repo.lastCreated.Name != "Match Writer" {
		t.Fatalf("expected sanitized name, got %q", repo.lastCreated.Name)
	}
	if response.Key == nil || response.Key.KeyPrefix != "sk_test" {
		t.Fatalf("expected response metadata with sk_test prefix, got %#v", response.Key)
	}
}

func TestAPIKeyServiceCreateAPIKeyRejectsDuplicateScopes(t *testing.T) {
	service := NewAPIKeyServiceWithRepository(newFakeAPIKeyRepository(), "test")

	_, err := service.CreateAPIKey(context.Background(), 42, &schemas.CreateAPIKeyRequest{
		Name:   "Writer",
		Scopes: []string{schemas.ScopeMatchesWrite, schemas.ScopeMatchesWrite},
	})

	assertAPIKeyAppErrorCode(t, err, exceptions.VALIDATION_ERROR)
}

func TestAPIKeyServiceVerifyAPIKeyRejectsExpiredKey(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	service := NewAPIKeyServiceWithRepository(repo, "test")

	rawKey := "sk_test_expired_key"
	expiredAt := time.Now().UTC().Add(-time.Minute)
	repo.seedActiveKey(&models.APIKey{
		ID:          1,
		UserID:      9,
		Name:        "Expired",
		KeyHash:     hashKey(rawKey),
		KeyPrefix:   "sk_test",
		KeyLastFour: "_key",
		Scopes:      []string{schemas.ScopeMatchesWrite},
		ExpiresAt:   &expiredAt,
	})

	_, err := service.VerifyAPIKey(context.Background(), rawKey, schemas.ScopeMatchesWrite)

	assertAPIKeyAppErrorCode(t, err, exceptions.UNAUTHORIZED)
}

func TestAPIKeyServiceVerifyAPIKeyRejectsMissingScope(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	service := NewAPIKeyServiceWithRepository(repo, "test")

	rawKey := "sk_test_scope_key"
	repo.seedActiveKey(&models.APIKey{
		ID:          2,
		UserID:      9,
		Name:        "Writer",
		KeyHash:     hashKey(rawKey),
		KeyPrefix:   "sk_test",
		KeyLastFour: "_key",
		Scopes:      []string{schemas.ScopeMatchesWrite},
	})

	_, err := service.VerifyAPIKey(context.Background(), rawKey, schemas.ScopeCommentaryWrite)

	assertAPIKeyAppErrorCode(t, err, exceptions.FORBIDDEN)
}

func TestAPIKeyServiceVerifyAPIKeyUpdatesLastUsedAt(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	service := NewAPIKeyServiceWithRepository(repo, "production")

	rawKey := "sk_live_valid_key"
	repo.seedActiveKey(&models.APIKey{
		ID:          3,
		UserID:      77,
		Name:        "Machine Writer",
		KeyHash:     hashKey(rawKey),
		KeyPrefix:   "sk_live",
		KeyLastFour: "_key",
		Scopes:      []string{schemas.ScopeMatchesWrite},
	})

	authKey, err := service.VerifyAPIKey(context.Background(), rawKey, schemas.ScopeMatchesWrite)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if authKey == nil || authKey.KeyID != 3 || authKey.UserID != 77 {
		t.Fatalf("unexpected authenticated key %#v", authKey)
	}
	if repo.updateLastUsedKeyID != 3 || repo.updateLastUsedValue.IsZero() {
		t.Fatalf("expected last_used_at update, got key=%d time=%v", repo.updateLastUsedKeyID, repo.updateLastUsedValue)
	}
}

func TestAPIKeyServiceRevokeKeyNotOwnedReturnsNotFound(t *testing.T) {
	service := NewAPIKeyServiceWithRepository(newFakeAPIKeyRepository(), "test")

	err := service.RevokeAPIKey(context.Background(), 15, 999)

	assertAPIKeyAppErrorCode(t, err, exceptions.NOT_FOUND)
}

func (r *fakeAPIKeyRepository) Create(_ context.Context, apiKey *models.APIKey) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.nextID++
	apiKey.ID = r.nextID
	now := time.Now().UTC()
	apiKey.CreatedAt = now
	apiKey.UpdatedAt = now
	r.lastCreated = cloneAPIKey(apiKey)
	r.keysByID[apiKey.ID] = cloneAPIKey(apiKey)
	r.keysByHash[apiKey.KeyHash] = cloneAPIKey(apiKey)
	return nil
}

func (r *fakeAPIKeyRepository) ListByUserID(_ context.Context, userID int64) ([]*models.APIKey, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var apiKeys []*models.APIKey
	for _, apiKey := range r.keysByID {
		if apiKey.UserID == userID {
			apiKeys = append(apiKeys, cloneAPIKey(apiKey))
		}
	}
	return apiKeys, nil
}

func (r *fakeAPIKeyRepository) FindByIDAndUserID(_ context.Context, keyID, userID int64) (*models.APIKey, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	apiKey, ok := r.keysByID[keyID]
	if !ok || apiKey.UserID != userID {
		return nil, nil
	}
	return cloneAPIKey(apiKey), nil
}

func (r *fakeAPIKeyRepository) FindActiveByHash(_ context.Context, keyHash string) (*models.APIKey, error) {
	if r.findActiveErr != nil {
		return nil, r.findActiveErr
	}
	apiKey, ok := r.keysByHash[keyHash]
	if !ok || apiKey.RevokedAt != nil {
		return nil, nil
	}
	return cloneAPIKey(apiKey), nil
}

func (r *fakeAPIKeyRepository) UpdateLastUsedAt(_ context.Context, keyID int64, usedAt time.Time) error {
	if r.updateLastUsedErr != nil {
		return r.updateLastUsedErr
	}
	apiKey, ok := r.keysByID[keyID]
	if !ok || apiKey.RevokedAt != nil {
		return gorm.ErrRecordNotFound
	}
	r.updateLastUsedKeyID = keyID
	r.updateLastUsedValue = usedAt
	apiKey.LastUsedAt = &usedAt
	apiKey.UpdatedAt = usedAt
	return nil
}

func (r *fakeAPIKeyRepository) RevokeByIDAndUserID(_ context.Context, keyID, userID int64, revokedAt time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	apiKey, ok := r.keysByID[keyID]
	if !ok || apiKey.UserID != userID {
		return nil
	}
	apiKey.RevokedAt = &revokedAt
	apiKey.UpdatedAt = revokedAt
	return nil
}

func (r *fakeAPIKeyRepository) seedActiveKey(apiKey *models.APIKey) {
	r.keysByID[apiKey.ID] = cloneAPIKey(apiKey)
	r.keysByHash[apiKey.KeyHash] = cloneAPIKey(apiKey)
}

func newFakeAPIKeyRepository() *fakeAPIKeyRepository {
	return &fakeAPIKeyRepository{
		keysByID:   map[int64]*models.APIKey{},
		keysByHash: map[string]*models.APIKey{},
	}
}

func cloneAPIKey(apiKey *models.APIKey) *models.APIKey {
	if apiKey == nil {
		return nil
	}
	copyKey := *apiKey
	copyKey.Scopes = append([]string(nil), apiKey.Scopes...)
	return &copyKey
}

func assertAPIKeyAppErrorCode(t *testing.T, err error, expectedCode string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != expectedCode {
		t.Fatalf("expected code %s, got %s", expectedCode, appErr.Code)
	}
}

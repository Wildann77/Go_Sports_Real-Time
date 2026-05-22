package repositories

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	apiKeyModels "sports-dashboard/internal/features/apikeys/models"
	authModels "sports-dashboard/internal/features/auth/models"
)

func TestAPIKeyRepositoryCreateFindUpdateAndRevoke(t *testing.T) {
	db := openAPIKeyRepositoryTestDB(t)
	resetAPIKeyRepositoryTables(t, db)

	repo := NewAPIKeyRepository(db, newAPIKeyRepositoryTimeoutPolicy())
	user := &authModels.User{
		Email:        "user@example.com",
		Name:         "User",
		PasswordHash: "bcrypt-hash",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	apiKey := &apiKeyModels.APIKey{
		UserID:      user.ID,
		Name:        "Writer",
		KeyPrefix:   "sk_test",
		KeyHash:     "hashed-key",
		KeyLastFour: "1234",
		Scopes:      []string{"matches:write"},
	}
	if err := repo.Create(context.Background(), apiKey); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	foundActive, err := repo.FindActiveByHash(context.Background(), "hashed-key")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if foundActive == nil || foundActive.UserID != user.ID || foundActive.KeyPrefix != "sk_test" {
		t.Fatalf("unexpected active key %#v", foundActive)
	}

	listed, err := repo.ListByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(listed) != 1 || listed[0].KeyHash != "hashed-key" {
		t.Fatalf("unexpected listed keys %#v", listed)
	}

	usedAt := time.Now().UTC()
	if err := repo.UpdateLastUsedAt(context.Background(), apiKey.ID, usedAt); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var refreshed apiKeyModels.APIKey
	if err := db.First(&refreshed, apiKey.ID).Error; err != nil {
		t.Fatalf("failed to reload api key: %v", err)
	}
	if refreshed.LastUsedAt == nil || refreshed.LastUsedAt.Before(usedAt.Add(-time.Second)) {
		t.Fatalf("expected last_used_at to be updated, got %#v", refreshed.LastUsedAt)
	}

	if err := repo.RevokeByIDAndUserID(context.Background(), apiKey.ID, user.ID, usedAt.Add(time.Minute)); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	foundAfterRevoke, err := repo.FindActiveByHash(context.Background(), "hashed-key")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if foundAfterRevoke != nil {
		t.Fatalf("expected revoked key not to be active, got %#v", foundAfterRevoke)
	}
}

func openAPIKeyRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping api key repository integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&authModels.User{}, &apiKeyModels.APIKey{}); err != nil {
		t.Fatalf("failed to migrate api key repository tables: %v", err)
	}

	return db
}

func resetAPIKeyRepositoryTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE api_keys, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate api key repository tables: %v", err)
	}
}

func newAPIKeyRepositoryTimeoutPolicy() *coreDatabase.TimeoutPolicy {
	return coreDatabase.NewTimeoutPolicy(config.LoadConfig())
}

// API Key Repository unit tests

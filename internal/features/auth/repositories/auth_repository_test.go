package repositories

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	authModels "sports-dashboard/internal/features/auth/models"
)

func TestAuthRepositoryFindUserAndSessionQueries(t *testing.T) {
	db := openAuthRepositoryTestDB(t)
	resetAuthRepositoryTables(t, db)

	repo := NewAuthRepository(db, newAuthRepositoryTimeoutPolicy())
	user := &authModels.User{
		Email:        "user@example.com",
		Name:         "User",
		PasswordHash: "bcrypt-hash",
		TokenVersion: 2,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	session := &authModels.RefreshSession{
		UserID:    user.ID,
		TokenHash: "hashed-refresh-token",
		JTI:       "jti-query",
		FamilyID:  "family-query",
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
		UserAgent: "Browser/1.0",
		IPAddress: "203.0.113.20",
	}
	if err := repo.CreateRefreshSession(context.Background(), session); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	foundByEmail, err := repo.FindUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if foundByEmail == nil || foundByEmail.ID != user.ID {
		t.Fatalf("expected user lookup by email to succeed, got %#v", foundByEmail)
	}

	foundByID, err := repo.FindUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if foundByID == nil || foundByID.Email != user.Email {
		t.Fatalf("expected user lookup by id to succeed, got %#v", foundByID)
	}

	foundSession, err := repo.FindRefreshSessionByJTI(context.Background(), session.JTI)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if foundSession == nil || foundSession.UserID != user.ID || foundSession.FamilyID != session.FamilyID {
		t.Fatalf("expected refresh session lookup by jti to succeed, got %#v", foundSession)
	}
}

func TestAuthRepositoryCreateUserHonorsUniqueEmailConstraint(t *testing.T) {
	db := openAuthRepositoryTestDB(t)
	resetAuthRepositoryTables(t, db)

	firstUser := &authModels.User{
		Email:        "user@example.com",
		Name:         "User One",
		PasswordHash: "bcrypt-hash",
	}
	secondUser := &authModels.User{
		Email:        "user@example.com",
		Name:         "User Two",
		PasswordHash: "bcrypt-hash",
	}

	if err := db.Create(firstUser).Error; err != nil {
		t.Fatalf("failed to seed first user: %v", err)
	}
	if err := db.Create(secondUser).Error; err == nil {
		t.Fatal("expected unique email constraint error, got nil")
	}
}

func TestAuthRepositoryRevocationHelpersAndTokenVersionIncrement(t *testing.T) {
	db := openAuthRepositoryTestDB(t)
	resetAuthRepositoryTables(t, db)

	repo := NewAuthRepository(db, newAuthRepositoryTimeoutPolicy())
	user := &authModels.User{
		Email:        "user@example.com",
		Name:         "User",
		PasswordHash: "bcrypt-hash",
		TokenVersion: 0,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	sessionOne := &authModels.RefreshSession{
		UserID:    user.ID,
		TokenHash: "hash-1",
		JTI:       "jti-1",
		FamilyID:  "family-a",
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	sessionTwo := &authModels.RefreshSession{
		UserID:    user.ID,
		TokenHash: "hash-2",
		JTI:       "jti-2",
		FamilyID:  "family-a",
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	sessionThree := &authModels.RefreshSession{
		UserID:    user.ID,
		TokenHash: "hash-3",
		JTI:       "jti-3",
		FamilyID:  "family-b",
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	if err := db.Create(sessionOne).Error; err != nil {
		t.Fatalf("failed to seed sessionOne: %v", err)
	}
	if err := db.Create(sessionTwo).Error; err != nil {
		t.Fatalf("failed to seed sessionTwo: %v", err)
	}
	if err := db.Create(sessionThree).Error; err != nil {
		t.Fatalf("failed to seed sessionThree: %v", err)
	}

	revokedAt := time.Now().UTC()
	replacedBy := sessionTwo.ID
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := repo.RevokeRefreshSessionByIDWithTx(context.Background(), tx, sessionOne.ID, revokedAt, &replacedBy); err != nil {
			return err
		}
		if err := repo.RevokeFamilySessionsWithTx(context.Background(), tx, user.ID, "family-a", revokedAt); err != nil {
			return err
		}
		if err := repo.IncrementUserTokenVersionWithTx(context.Background(), tx, user.ID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var refreshedOne authModels.RefreshSession
	if err := db.First(&refreshedOne, sessionOne.ID).Error; err != nil {
		t.Fatalf("failed to reload sessionOne: %v", err)
	}
	if refreshedOne.RevokedAt == nil || refreshedOne.ReplacedBy == nil || *refreshedOne.ReplacedBy != sessionTwo.ID {
		t.Fatalf("expected sessionOne to be revoked and replaced, got %#v", refreshedOne)
	}

	var refreshedTwo authModels.RefreshSession
	if err := db.First(&refreshedTwo, sessionTwo.ID).Error; err != nil {
		t.Fatalf("failed to reload sessionTwo: %v", err)
	}
	if refreshedTwo.RevokedAt == nil {
		t.Fatalf("expected sessionTwo to be revoked by family revoke, got %#v", refreshedTwo)
	}

	var refreshedThree authModels.RefreshSession
	if err := db.First(&refreshedThree, sessionThree.ID).Error; err != nil {
		t.Fatalf("failed to reload sessionThree: %v", err)
	}
	if refreshedThree.RevokedAt != nil {
		t.Fatalf("expected family-b session to stay active before revoke-all, got %#v", refreshedThree)
	}

	var refreshedUser authModels.User
	if err := db.First(&refreshedUser, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if refreshedUser.TokenVersion != 1 {
		t.Fatalf("expected token version incremented to 1, got %d", refreshedUser.TokenVersion)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return repo.RevokeAllUserSessionsWithTx(context.Background(), tx, user.ID, revokedAt.Add(time.Minute))
	}); err != nil {
		t.Fatalf("expected nil error from revoke-all, got %v", err)
	}

	if err := db.First(&refreshedThree, sessionThree.ID).Error; err != nil {
		t.Fatalf("failed to reload sessionThree after revoke-all: %v", err)
	}
	if refreshedThree.RevokedAt == nil {
		t.Fatalf("expected family-b session to be revoked by revoke-all, got %#v", refreshedThree)
	}
}

func openAuthRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping auth repository integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&authModels.User{}, &authModels.RefreshSession{}); err != nil {
		t.Fatalf("failed to migrate auth repository tables: %v", err)
	}

	return db
}

func resetAuthRepositoryTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE refresh_sessions, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate auth repository tables: %v", err)
	}
}

func newAuthRepositoryTimeoutPolicy() *coreDatabase.TimeoutPolicy {
	cfg := config.LoadConfig()
	return coreDatabase.NewTimeoutPolicy(cfg)
}

// Auth Repository unit tests

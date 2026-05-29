package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/features/auth/models"
	"sports-dashboard/internal/features/auth/schemas"
	"sports-dashboard/internal/features/auth/utils"
)

type fakeAuthState struct {
	users              map[int64]*models.User
	usersByEmail       map[string]int64
	sessionsByJTI      map[string]*models.RefreshSession
	nextSessionID      int64
	familyRevokeCalls  []string
	allRevokeCalls     []int64
	incrementUserCalls []int64
}

type fakeAuthRepository struct {
	state *fakeAuthState
}

type fakeAuthTransactionManager struct {
	committed  bool
	rolledBack bool
}

func TestAuthServiceLoginCreatesHashedRefreshSession(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 10, "user@example.com", "secret123", 2)
	service := newTestAuthService(state)

	res, refreshToken, err := service.Login(context.Background(), &schemas.LoginRequest{
		Email:    " USER@example.com ",
		Password: "secret123",
	}, RequestMetadata{
		UserAgent: " Test Agent ",
		IPAddress: " 127.0.0.1 ",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if res.AccessToken == "" || refreshToken == "" {
		t.Fatal("expected access and refresh token to be returned")
	}
	if res.User == nil || res.User.ID != user.ID || res.User.Email != user.Email {
		t.Fatalf("unexpected user response %#v", res.User)
	}

	if len(state.sessionsByJTI) != 1 {
		t.Fatalf("expected one refresh session, got %d", len(state.sessionsByJTI))
	}

	var persistedSession *models.RefreshSession
	for _, session := range state.sessionsByJTI {
		persistedSession = session
	}

	if persistedSession.TokenHash == refreshToken {
		t.Fatal("expected refresh token to be stored hashed, not raw")
	}
	if !utils.CompareTokenHash(refreshToken, persistedSession.TokenHash) {
		t.Fatal("expected stored refresh token hash to match raw refresh token")
	}
	if persistedSession.UserAgent != "Test Agent" || persistedSession.IPAddress != "127.0.0.1" {
		t.Fatalf("expected sanitized request metadata, got %#v", persistedSession)
	}

	accessClaims, err := utils.ParseAndVerifyToken(res.AccessToken, service.accessSecret, utils.TokenTypeAccess, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected valid access token, got %v", err)
	}
	if accessClaims.UserID != user.ID || accessClaims.TokenVersion != user.TokenVersion {
		t.Fatalf("unexpected access claims %#v", accessClaims)
	}

	refreshClaims, err := utils.ParseAndVerifyToken(refreshToken, service.refreshSecret, utils.TokenTypeRefresh, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected valid refresh token, got %v", err)
	}
	if refreshClaims.JTI != persistedSession.JTI || refreshClaims.FamilyID != persistedSession.FamilyID {
		t.Fatalf("unexpected refresh claims %#v and session %#v", refreshClaims, persistedSession)
	}
}

func TestAuthServiceLoginRejectsInvalidPassword(t *testing.T) {
	state := newFakeAuthState()
	seedAuthUser(t, state, 11, "user@example.com", "secret123", 0)
	service := newTestAuthService(state)

	_, _, err := service.Login(context.Background(), &schemas.LoginRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	}, RequestMetadata{})
	assertAuthAppErrorCode(t, err, exceptions.UNAUTHORIZED)
}

func TestAuthServiceVerifyAccessTokenRejectsTokenVersionMismatch(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 12, "user@example.com", "secret123", 1)
	service := newTestAuthService(state)

	token, _, err := utils.GenerateToken(utils.TokenOptions{
		Secret:       service.accessSecret,
		TTL:          service.accessTTL,
		TokenType:    utils.TokenTypeAccess,
		UserID:       user.ID,
		TokenVersion: 0,
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = service.VerifyAccessToken(context.Background(), token)
	assertAuthAppErrorCode(t, err, exceptions.UNAUTHORIZED)
}

func TestAuthServiceVerifyAccessTokenRejectsInvalidToken(t *testing.T) {
	state := newFakeAuthState()
	service := newTestAuthService(state)

	_, err := service.VerifyAccessToken(context.Background(), "not-a-valid-token")
	assertAuthAppErrorCode(t, err, exceptions.UNAUTHORIZED)
}

func TestAuthServiceRefreshTokenRotatesSession(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 13, "user@example.com", "secret123", 0)
	service := newTestAuthService(state)
	txManager := &fakeAuthTransactionManager{}
	service.txManager = txManager

	refreshToken, session := seedRefreshSession(t, state, service, user, "family-1", "jti-old", user.TokenVersion)

	res, rotatedRefreshToken, err := service.RefreshToken(context.Background(), refreshToken, RequestMetadata{
		UserAgent: "Browser/1.0",
		IPAddress: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !txManager.committed || txManager.rolledBack {
		t.Fatalf("expected committed transaction, got committed=%v rolledBack=%v", txManager.committed, txManager.rolledBack)
	}
	if res.AccessToken == "" || rotatedRefreshToken == "" {
		t.Fatal("expected rotated tokens to be returned")
	}
	if rotatedRefreshToken == refreshToken {
		t.Fatal("expected rotated refresh token to differ from original")
	}

	if session.RevokedAt == nil {
		t.Fatal("expected original session to be revoked")
	}
	if session.ReplacedBy == nil || *session.ReplacedBy == 0 {
		t.Fatalf("expected original session to reference replacement, got %#v", session.ReplacedBy)
	}

	if len(state.sessionsByJTI) != 2 {
		t.Fatalf("expected two sessions after rotation, got %d", len(state.sessionsByJTI))
	}

	rotatedClaims, err := utils.ParseAndVerifyToken(rotatedRefreshToken, service.refreshSecret, utils.TokenTypeRefresh, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected valid rotated refresh token, got %v", err)
	}
	if rotatedClaims.FamilyID != "family-1" || rotatedClaims.JTI == "jti-old" {
		t.Fatalf("unexpected rotated claims %#v", rotatedClaims)
	}

	newSession := state.sessionsByJTI[rotatedClaims.JTI]
	if newSession == nil {
		t.Fatalf("expected replacement session for jti %s", rotatedClaims.JTI)
	}
	if !utils.CompareTokenHash(rotatedRefreshToken, newSession.TokenHash) {
		t.Fatal("expected replacement session hash to match rotated refresh token")
	}
	if newSession.UserAgent != "Browser/1.0" || newSession.IPAddress != "203.0.113.10" {
		t.Fatalf("expected replacement session metadata persisted, got %#v", newSession)
	}
}

func TestAuthServiceRefreshTokenReuseRevokesFamilyAndIncrementsTokenVersion(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 14, "user@example.com", "secret123", 0)
	service := newTestAuthService(state)

	reusedToken, reusedSession := seedRefreshSession(t, state, service, user, "family-2", "jti-reused", user.TokenVersion)
	now := time.Now().UTC()
	reusedSession.RevokedAt = &now
	_, siblingSession := seedRefreshSession(t, state, service, user, "family-2", "jti-sibling", user.TokenVersion)

	_, _, err := service.RefreshToken(context.Background(), reusedToken, RequestMetadata{})
	assertAuthAppErrorCode(t, err, exceptions.SECURITY_ERROR)

	if user.TokenVersion != 1 {
		t.Fatalf("expected token version incremented to 1, got %d", user.TokenVersion)
	}
	if siblingSession.RevokedAt == nil {
		t.Fatal("expected sibling session in same family to be revoked")
	}
	if len(state.familyRevokeCalls) != 1 || state.familyRevokeCalls[0] != "family-2" {
		t.Fatalf("expected family revoke call for family-2, got %#v", state.familyRevokeCalls)
	}
}

func TestAuthServiceLogoutCurrentDeviceRevokesSession(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 15, "user@example.com", "secret123", 0)
	service := newTestAuthService(state)

	refreshToken, session := seedRefreshSession(t, state, service, user, "family-3", "jti-logout", user.TokenVersion)

	if err := service.LogoutCurrentDevice(context.Background(), refreshToken); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if session.RevokedAt == nil {
		t.Fatal("expected session to be revoked on logout")
	}
}

func TestAuthServiceLogoutAllDevicesRevokesAllSessionsAndIncrementsTokenVersion(t *testing.T) {
	state := newFakeAuthState()
	user := seedAuthUser(t, state, 16, "user@example.com", "secret123", 4)
	service := newTestAuthService(state)

	_, sessionOne := seedRefreshSession(t, state, service, user, "family-a", "jti-a", user.TokenVersion)
	_, sessionTwo := seedRefreshSession(t, state, service, user, "family-b", "jti-b", user.TokenVersion)

	if err := service.LogoutAllDevices(context.Background(), user.ID); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if sessionOne.RevokedAt == nil || sessionTwo.RevokedAt == nil {
		t.Fatal("expected all active sessions to be revoked")
	}
	if user.TokenVersion != 5 {
		t.Fatalf("expected token version incremented to 5, got %d", user.TokenVersion)
	}
	if len(state.allRevokeCalls) != 1 || state.allRevokeCalls[0] != user.ID {
		t.Fatalf("expected revoke-all call for user %d, got %#v", user.ID, state.allRevokeCalls)
	}
}

func (r *fakeAuthRepository) FindUserByEmail(_ context.Context, email string) (*models.User, error) {
	userID, ok := r.state.usersByEmail[email]
	if !ok {
		return nil, nil
	}
	return r.state.users[userID], nil
}

func (r *fakeAuthRepository) FindUserByID(_ context.Context, userID int64) (*models.User, error) {
	return r.state.users[userID], nil
}

func (r *fakeAuthRepository) FindUserByIDForUpdateWithTx(_ context.Context, _ *gorm.DB, userID int64) (*models.User, error) {
	return r.state.users[userID], nil
}

func (r *fakeAuthRepository) CreateRefreshSession(_ context.Context, session *models.RefreshSession) error {
	return r.createRefreshSession(session)
}

func (r *fakeAuthRepository) CreateRefreshSessionWithTx(_ context.Context, _ *gorm.DB, session *models.RefreshSession) error {
	return r.createRefreshSession(session)
}

func (r *fakeAuthRepository) FindRefreshSessionByJTI(_ context.Context, jti string) (*models.RefreshSession, error) {
	return r.state.sessionsByJTI[jti], nil
}

func (r *fakeAuthRepository) FindRefreshSessionByJTIForUpdateWithTx(_ context.Context, _ *gorm.DB, jti string) (*models.RefreshSession, error) {
	return r.state.sessionsByJTI[jti], nil
}

func (r *fakeAuthRepository) RevokeRefreshSessionByIDWithTx(_ context.Context, _ *gorm.DB, sessionID int64, revokedAt time.Time, replacedBy *int64) error {
	for _, session := range r.state.sessionsByJTI {
		if session.ID != sessionID {
			continue
		}
		session.RevokedAt = &revokedAt
		session.ReplacedBy = replacedBy
		return nil
	}
	return nil
}

func (r *fakeAuthRepository) RevokeFamilySessionsWithTx(_ context.Context, _ *gorm.DB, userID int64, familyID string, revokedAt time.Time) error {
	r.state.familyRevokeCalls = append(r.state.familyRevokeCalls, familyID)
	for _, session := range r.state.sessionsByJTI {
		if session.UserID == userID && session.FamilyID == familyID && session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
		}
	}
	return nil
}

func (r *fakeAuthRepository) RevokeAllUserSessionsWithTx(_ context.Context, _ *gorm.DB, userID int64, revokedAt time.Time) error {
	r.state.allRevokeCalls = append(r.state.allRevokeCalls, userID)
	for _, session := range r.state.sessionsByJTI {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
		}
	}
	return nil
}

func (r *fakeAuthRepository) IncrementUserTokenVersionWithTx(_ context.Context, _ *gorm.DB, userID int64) error {
	r.state.incrementUserCalls = append(r.state.incrementUserCalls, userID)
	if user := r.state.users[userID]; user != nil {
		user.TokenVersion++
	}
	return nil
}

func (r *fakeAuthRepository) createRefreshSession(session *models.RefreshSession) error {
	r.state.nextSessionID++
	session.ID = r.state.nextSessionID
	r.state.sessionsByJTI[session.JTI] = session
	return nil
}

func (m *fakeAuthTransactionManager) WithinTransaction(_ context.Context, fn func(tx *gorm.DB) error) error {
	err := fn(nil)
	if err != nil {
		m.rolledBack = true
		return err
	}
	m.committed = true
	return nil
}

func newTestAuthService(state *fakeAuthState) *AuthService {
	return &AuthService{
		repo:          &fakeAuthRepository{state: state},
		txManager:     &fakeAuthTransactionManager{},
		timeoutPolicy: nil,
		accessSecret:  "access-secret",
		refreshSecret: "refresh-secret",
		accessTTL:     15 * time.Minute,
		refreshTTL:    30 * 24 * time.Hour,
	}
}

func newFakeAuthState() *fakeAuthState {
	return &fakeAuthState{
		users:         map[int64]*models.User{},
		usersByEmail:  map[string]int64{},
		sessionsByJTI: map[string]*models.RefreshSession{},
	}
}

func seedAuthUser(t *testing.T, state *fakeAuthState, userID int64, email, password string, tokenVersion int) *models.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &models.User{
		ID:           userID,
		Email:        strings.ToLower(strings.TrimSpace(email)),
		Name:         "Test User",
		PasswordHash: string(passwordHash),
		TokenVersion: tokenVersion,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	state.users[userID] = user
	state.usersByEmail[user.Email] = userID
	return user
}

func seedRefreshSession(t *testing.T, state *fakeAuthState, service *AuthService, user *models.User, familyID, jti string, tokenVersion int) (string, *models.RefreshSession) {
	t.Helper()

	token, claims, err := utils.GenerateToken(utils.TokenOptions{
		Secret:       service.refreshSecret,
		TTL:          service.refreshTTL,
		TokenType:    utils.TokenTypeRefresh,
		UserID:       user.ID,
		TokenVersion: tokenVersion,
		JTI:          jti,
		FamilyID:     familyID,
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	state.nextSessionID++
	session := &models.RefreshSession{
		ID:        state.nextSessionID,
		UserID:    user.ID,
		TokenHash: utils.HashToken(token),
		JTI:       claims.JTI,
		FamilyID:  claims.FamilyID,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	state.sessionsByJTI[session.JTI] = session

	return token, session
}

func assertAuthAppErrorCode(t *testing.T, err error, expectedCode string) {
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

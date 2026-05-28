package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	internalpkg "sports-dashboard/internal"
	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	authModels "sports-dashboard/internal/features/auth/models"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

func TestAuthE2ELoginRefreshLogoutAllFlow(t *testing.T) {
	db := openAuthE2ETestDB(t)
	resetAuthE2ETestTables(t, db)

	seedAuthE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newAuthE2ETestConfig()
	router, cleanup := internalpkg.SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	loginBody := []byte(`{"email":"user@example.com","password":"secret123"}`)
	loginReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/login", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("failed to create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("failed to execute login request: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", loginResp.StatusCode)
	}

	loginPayload := decodeAuthE2EResponse(t, loginResp)
	loginData := loginPayload.Data.(map[string]any)
	accessToken := loginData["accessToken"].(string)
	refreshCookie := firstCookie(t, loginResp.Cookies(), cfg.RefreshCookieName)

	meReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/me", nil)
	if err != nil {
		t.Fatalf("failed to create me request: %v", err)
	}
	meReq.Header.Set("Authorization", "Bearer "+accessToken)

	meResp, err := client.Do(meReq)
	if err != nil {
		t.Fatalf("failed to execute me request: %v", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from /me, got %d", meResp.StatusCode)
	}

	refreshReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/refresh-token", nil)
	if err != nil {
		t.Fatalf("failed to create refresh request: %v", err)
	}
	refreshReq.AddCookie(refreshCookie)

	refreshResp, err := client.Do(refreshReq)
	if err != nil {
		t.Fatalf("failed to execute refresh request: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from refresh, got %d", refreshResp.StatusCode)
	}

	refreshPayload := decodeAuthE2EResponse(t, refreshResp)
	refreshData := refreshPayload.Data.(map[string]any)
	refreshedAccessToken := refreshData["accessToken"].(string)
	rotatedCookie := firstCookie(t, refreshResp.Cookies(), cfg.RefreshCookieName)
	if rotatedCookie.Value == refreshCookie.Value {
		t.Fatal("expected rotated refresh cookie to differ from original")
	}

	logoutAllReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/logout-all", nil)
	if err != nil {
		t.Fatalf("failed to create logout-all request: %v", err)
	}
	logoutAllReq.Header.Set("Authorization", "Bearer "+refreshedAccessToken)
	logoutAllReq.AddCookie(rotatedCookie)

	logoutAllResp, err := client.Do(logoutAllReq)
	if err != nil {
		t.Fatalf("failed to execute logout-all request: %v", err)
	}
	defer logoutAllResp.Body.Close()

	if logoutAllResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from logout-all, got %d", logoutAllResp.StatusCode)
	}

	meAfterLogoutReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/me", nil)
	if err != nil {
		t.Fatalf("failed to create follow-up me request: %v", err)
	}
	meAfterLogoutReq.Header.Set("Authorization", "Bearer "+refreshedAccessToken)

	meAfterLogoutResp, err := client.Do(meAfterLogoutReq)
	if err != nil {
		t.Fatalf("failed to execute follow-up me request: %v", err)
	}
	defer meAfterLogoutResp.Body.Close()

	if meAfterLogoutResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 after logout-all, got %d", meAfterLogoutResp.StatusCode)
	}
}

func TestAuthE2ELogoutCurrentDeviceRevokesRefreshSession(t *testing.T) {
	db := openAuthE2ETestDB(t)
	resetAuthE2ETestTables(t, db)

	seedAuthE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newAuthE2ETestConfig()
	router, cleanup := internalpkg.SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	loginBody := []byte(`{"email":"user@example.com","password":"secret123"}`)
	loginReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("failed to execute login request: %v", err)
	}
	defer loginResp.Body.Close()

	refreshCookie := firstCookie(t, loginResp.Cookies(), cfg.RefreshCookieName)

	logoutReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/logout", nil)
	if err != nil {
		t.Fatalf("failed to create logout request: %v", err)
	}
	logoutReq.AddCookie(refreshCookie)

	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("failed to execute logout request: %v", err)
	}
	defer logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from logout, got %d", logoutResp.StatusCode)
	}

	refreshReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/refresh-token", nil)
	if err != nil {
		t.Fatalf("failed to create refresh request: %v", err)
	}
	refreshReq.AddCookie(refreshCookie)

	refreshResp, err := client.Do(refreshReq)
	if err != nil {
		t.Fatalf("failed to execute refresh request: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 from refresh after logout, got %d", refreshResp.StatusCode)
	}
}

func TestAuthE2ERefreshTokenReuseRevokesFamily(t *testing.T) {
	db := openAuthE2ETestDB(t)
	resetAuthE2ETestTables(t, db)

	seedAuthE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newAuthE2ETestConfig()
	router, cleanup := internalpkg.SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	loginBody := []byte(`{"email":"user@example.com","password":"secret123"}`)
	loginReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("failed to execute login request: %v", err)
	}
	defer loginResp.Body.Close()

	originalCookie := firstCookie(t, loginResp.Cookies(), cfg.RefreshCookieName)

	refreshReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/refresh-token", nil)
	refreshReq.AddCookie(originalCookie)
	refreshResp, err := client.Do(refreshReq)
	if err != nil {
		t.Fatalf("failed to execute initial refresh request: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from initial refresh, got %d", refreshResp.StatusCode)
	}

	rotatedCookie := firstCookie(t, refreshResp.Cookies(), cfg.RefreshCookieName)

	reuseReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/refresh-token", nil)
	reuseReq.AddCookie(originalCookie)
	reuseResp, err := client.Do(reuseReq)
	if err != nil {
		t.Fatalf("failed to execute refresh reuse request: %v", err)
	}
	defer reuseResp.Body.Close()

	if reuseResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 from revoked refresh-token reuse, got %d", reuseResp.StatusCode)
	}

	reusePayload := decodeAuthE2EResponse(t, reuseResp)
	if reusePayload.Error == nil || reusePayload.Error.Code != "SECURITY_ERROR" {
		t.Fatalf("expected SECURITY_ERROR on refresh-token reuse, got %#v", reusePayload)
	}

	postTheftReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/refresh-token", nil)
	postTheftReq.AddCookie(rotatedCookie)
	postTheftResp, err := client.Do(postTheftReq)
	if err != nil {
		t.Fatalf("failed to execute post-theft refresh request: %v", err)
	}
	defer postTheftResp.Body.Close()

	if postTheftResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 from family-revoked refresh token, got %d", postTheftResp.StatusCode)
	}
}

func openAuthE2ETestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping auth e2e test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&authModels.User{}, &authModels.RefreshSession{}); err != nil {
		t.Fatalf("failed to migrate auth e2e tables: %v", err)
	}

	return db
}

func resetAuthE2ETestTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE refresh_sessions, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate auth e2e tables: %v", err)
	}
}

func seedAuthE2EUser(t *testing.T, db *gorm.DB, email, password string) *authModels.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &authModels.User{
		Email:        email,
		Name:         "E2E User",
		PasswordHash: string(passwordHash),
		TokenVersion: 0,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed auth e2e user: %v", err)
	}

	return user
}

func newAuthE2ETestConfig() *config.Config {
	cfg := config.LoadConfig()
	cfg.AppEnv = "test"
	cfg.AllowedOrigins = []string{"http://localhost:3000"}
	cfg.RateLimitRPS = 1000
	cfg.RateLimitBurst = 1000
	cfg.JWTAccessSecret = "auth-e2e-access-secret"
	cfg.JWTRefreshSecret = "auth-e2e-refresh-secret"
	cfg.AccessTokenTTLMinutes = 15
	cfg.RefreshTokenTTLDays = 30
	cfg.RefreshCookieName = "refresh_token"
	cfg.RefreshCookiePath = "/"
	cfg.RefreshCookieSameSite = "lax"
	cfg.RefreshCookieSecure = false
	cfg.DbQueryTimeoutSeconds = 5
	cfg.DbTxTimeoutSeconds = 10
	return cfg
}

func decodeAuthE2EResponse(t *testing.T, resp *http.Response) globalSchemas.Response {
	t.Helper()

	var payload globalSchemas.Response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return payload
}

func firstCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %q, got %#v", name, cookies)
	return nil
}

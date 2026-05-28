package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	internalpkg "sports-dashboard/internal"
	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	apiKeyModels "sports-dashboard/internal/features/apikeys/models"
	authModels "sports-dashboard/internal/features/auth/models"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	matchModels "sports-dashboard/internal/features/matches/models"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

func TestAPIKeyE2ECreateAndUseAPIKeyForWriteRoutes(t *testing.T) {
	db := openAPIKeyE2ETestDB(t)
	resetAPIKeyE2ETables(t, db)
	seedAPIKeyE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newAPIKeyE2ETestConfig()
	router, cleanup := internalpkg.SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}

	accessToken := loginAPIKeyE2EUser(t, client, server.URL, "user@example.com", "secret123")
	rawAPIKey := createAPIKeyE2E(t, client, server.URL, accessToken)

	matchPayload := map[string]any{
		"sport":     "football",
		"homeTeam":  "Alpha",
		"awayTeam":  "Beta",
		"startTime": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"endTime":   time.Now().Add(80 * time.Minute).Format(time.RFC3339),
	}
	matchID := createMatchWithBearer(t, client, server.URL, rawAPIKey, matchPayload)

	commentaryPayload := map[string]any{
		"minute":    10,
		"eventType": "goal",
		"message":   "Goal by Alpha!",
		"payload": map[string]any{
			"homeScore": 1,
		},
	}
	createCommentaryWithBearer(t, client, server.URL, rawAPIKey, matchID, commentaryPayload)

	matchReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/matches/"+strconv.FormatInt(matchID, 10), nil)
	matchResp, err := client.Do(matchReq)
	if err != nil {
		t.Fatalf("failed to fetch match: %v", err)
	}
	defer matchResp.Body.Close()
	if matchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected public GET match status 200, got %d", matchResp.StatusCode)
	}

	commentaryReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/matches/"+strconv.FormatInt(matchID, 10)+"/commentary", nil)
	commentaryResp, err := client.Do(commentaryReq)
	if err != nil {
		t.Fatalf("failed to fetch commentary: %v", err)
	}
	defer commentaryResp.Body.Close()
	if commentaryResp.StatusCode != http.StatusOK {
		t.Fatalf("expected public GET commentary status 200, got %d", commentaryResp.StatusCode)
	}

	secondMatchPayload := map[string]any{
		"sport":     "basketball",
		"homeTeam":  "Gamma",
		"awayTeam":  "Delta",
		"startTime": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"endTime":   time.Now().Add(60 * time.Minute).Format(time.RFC3339),
	}
	createMatchWithBearer(t, client, server.URL, accessToken, secondMatchPayload)
}

func openAPIKeyE2ETestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping api key e2e test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(
		&authModels.User{},
		&authModels.RefreshSession{},
		&apiKeyModels.APIKey{},
		&matchModels.Match{},
		&commentaryModels.Commentary{},
	); err != nil {
		t.Fatalf("failed to migrate api key e2e tables: %v", err)
	}

	return db
}

func resetAPIKeyE2ETables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches, api_keys, refresh_sessions, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate api key e2e tables: %v", err)
	}
}

func seedAPIKeyE2EUser(t *testing.T, db *gorm.DB, email, password string) *authModels.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &authModels.User{
		Email:        email,
		Name:         "API Key User",
		PasswordHash: string(passwordHash),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	return user
}

func newAPIKeyE2ETestConfig() *config.Config {
	cfg := config.LoadConfig()
	cfg.AppEnv = "test"
	cfg.AllowedOrigins = []string{"http://localhost:3000"}
	cfg.RateLimitRPS = 1000
	cfg.RateLimitBurst = 1000
	cfg.JWTAccessSecret = "api-key-e2e-access-secret"
	cfg.JWTRefreshSecret = "api-key-e2e-refresh-secret"
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

func loginAPIKeyE2EUser(t *testing.T, client *http.Client, baseURL, email, password string) string {
	t.Helper()

	body := []byte(`{"email":"` + email + `","password":"` + password + `"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/login", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected login status 200, got %d", resp.StatusCode)
	}

	payload := decodeAPIKeyE2EResponse(t, resp)
	data := payload.Data.(map[string]any)
	return data["accessToken"].(string)
}

func createAPIKeyE2E(t *testing.T, client *http.Client, baseURL, accessToken string) string {
	t.Helper()

	body := []byte(`{"name":"Writer","scopes":["matches:write","commentary:write"]}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/api-keys", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create api key request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute api key create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected api key create status 201, got %d", resp.StatusCode)
	}

	payload := decodeAPIKeyE2EResponse(t, resp)
	data := payload.Data.(map[string]any)
	return data["apiKey"].(string)
}

func createMatchWithBearer(t *testing.T, client *http.Client, baseURL, bearerToken string, payload map[string]any) int64 {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal match payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/matches", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create match request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute match create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected match create status 201, got %d", resp.StatusCode)
	}

	response := decodeAPIKeyE2EResponse(t, resp)
	data := response.Data.(map[string]any)
	return int64(data["id"].(float64))
}

func createCommentaryWithBearer(t *testing.T, client *http.Client, baseURL, bearerToken string, matchID int64, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal commentary payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/matches/"+strconv.FormatInt(matchID, 10)+"/commentary", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create commentary request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute commentary create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected commentary create status 201, got %d", resp.StatusCode)
	}
}

func decodeAPIKeyE2EResponse(t *testing.T, resp *http.Response) globalSchemas.Response {
	t.Helper()

	var payload globalSchemas.Response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return payload
}

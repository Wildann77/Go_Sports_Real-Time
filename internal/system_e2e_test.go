package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"golang.org/x/crypto/bcrypt"
	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	authModels "sports-dashboard/internal/features/auth/models"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	matchModels "sports-dashboard/internal/features/matches/models"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	envPath := filepath.Join(dir, "../.env")
	_ = godotenv.Load(envPath)
}

func openSystemE2ETestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping system e2e test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&authModels.User{}, &authModels.RefreshSession{}, &matchModels.Match{}, &commentaryModels.Commentary{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	return db
}

func resetSystemE2ETestTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches, refresh_sessions, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

func TestSystemE2EHappyPath(t *testing.T) {
	db := openSystemE2ETestDB(t)
	resetSystemE2ETestTables(t, db)
	seedSystemE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newSystemE2ETestConfig()
	router, cleanup := SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	accessToken := loginSystemE2EUser(t, client, server.URL, "user@example.com", "secret123")

	// 1. Create Match via REST
	matchPayload := map[string]any{
		"sport":     "football",
		"homeTeam":  "Team-Alpha",
		"awayTeam":  "Team-Beta",
		"startTime": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"endTime":   time.Now().Add(80 * time.Minute).Format(time.RFC3339),
	}
	bodyBytes, err := json.Marshal(matchPayload)
	if err != nil {
		t.Fatalf("failed to marshal match payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/matches", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to create match via HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var matchRes globalSchemas.Response
	if err := json.NewDecoder(resp.Body).Decode(&matchRes); err != nil {
		t.Fatalf("failed to decode match response: %v", err)
	}

	matchData, ok := matchRes.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected match data map, got %T", matchRes.Data)
	}
	matchIDFloat, ok := matchData["id"].(float64)
	if !ok {
		t.Fatalf("expected id to be float64, got %T", matchData["id"])
	}
	matchID := int64(matchIDFloat)

	// 2. Subscribe via WebSocket
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	wsURL := "ws://" + u.Host + "/ws"

	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn.Close()

	// Read welcome event
	var welcomeMsg map[string]any
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := wsConn.ReadJSON(&welcomeMsg); err != nil {
		t.Fatalf("failed to read welcome message: %v", err)
	}
	if welcomeMsg["type"] != "welcome" {
		t.Fatalf("expected welcome event, got %v", welcomeMsg["type"])
	}

	// Send subscribe command
	subCmd := map[string]any{
		"type":    "subscribe",
		"matchId": matchID,
	}
	if err := wsConn.WriteJSON(subCmd); err != nil {
		t.Fatalf("failed to send subscribe cmd: %v", err)
	}

	// Read subscribed response
	var subResp map[string]any
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := wsConn.ReadJSON(&subResp); err != nil {
		t.Fatalf("failed to read subscribe response: %v", err)
	}
	if subResp["type"] != "subscribed" {
		t.Fatalf("expected subscribed response, got %v", subResp["type"])
	}

	// 3. Create Commentary via REST with Score Update
	commentaryPayload := map[string]any{
		"minute":    10,
		"eventType": "goal",
		"message":   "Goal by Alpha!",
		"payload": map[string]any{
			"homeScore": 1,
			"awayScore": 0,
		},
	}
	commBytes, err := json.Marshal(commentaryPayload)
	if err != nil {
		t.Fatalf("failed to marshal commentary payload: %v", err)
	}

	commURL := server.URL + "/api/v1/matches/" + strconv.FormatInt(matchID, 10) + "/commentary"
	reqComm, err := http.NewRequest(http.MethodPost, commURL, bytes.NewBuffer(commBytes))
	if err != nil {
		t.Fatalf("failed to create commentary request: %v", err)
	}
	reqComm.Header.Set("Content-Type", "application/json")
	reqComm.Header.Set("Authorization", "Bearer "+accessToken)

	respComm, err := client.Do(reqComm)
	if err != nil {
		t.Fatalf("failed to create commentary: %v", err)
	}
	defer respComm.Body.Close()

	if respComm.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", respComm.StatusCode)
	}

	// 4. Receive Live WebSocket Events (commentary.created and match.updated)
	events := []map[string]any{}
	timeout := time.After(2 * time.Second)
	for len(events) < 2 {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for 2 events, only got %d events: %#v", len(events), events)
		default:
			evs := readSystemE2EEvents(t, wsConn)
			events = append(events, evs...)
		}
	}

	var commEvent, matchEvent map[string]any
	for _, event := range events {
		if event["type"] == "commentary.created" {
			commEvent = event
		} else if event["type"] == "match.updated" {
			matchEvent = event
		}
	}

	if commEvent == nil {
		t.Fatal("expected commentary.created event, got nil")
	}
	if matchEvent == nil {
		t.Fatal("expected match.updated event, got nil")
	}

	commData, ok := commEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected commentary data map, got %T", commEvent["data"])
	}
	if commData["message"] != "Goal by Alpha!" {
		t.Fatalf("expected message 'Goal by Alpha!', got %v", commData["message"])
	}

	matchDataUpdated, ok := matchEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected match updated data map, got %T", matchEvent["data"])
	}
	if matchDataUpdated["homeScore"] != float64(1) || matchDataUpdated["awayScore"] != float64(0) {
		t.Fatalf("expected updated score 1-0 in websocket event, got %#v", matchDataUpdated)
	}

	// 5. Match Read Path Reflects Committed Score
	reqGet, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/matches/"+strconv.FormatInt(matchID, 10), nil)
	if err != nil {
		t.Fatalf("failed to create get match request: %v", err)
	}
	respGet, err := client.Do(reqGet)
	if err != nil {
		t.Fatalf("failed to perform get match: %v", err)
	}
	defer respGet.Body.Close()

	if respGet.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", respGet.StatusCode)
	}

	var getRes globalSchemas.Response
	if err := json.NewDecoder(respGet.Body).Decode(&getRes); err != nil {
		t.Fatalf("failed to decode get match response: %v", err)
	}

	getData, ok := getRes.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected match data map, got %T", getRes.Data)
	}
	if getData["homeScore"] != float64(1) || getData["awayScore"] != float64(0) {
		t.Fatalf("expected score 1-0 in GET response, got home=%v, away=%v", getData["homeScore"], getData["awayScore"])
	}
}

func TestSystemE2EFailureScenario(t *testing.T) {
	db := openSystemE2ETestDB(t)
	resetSystemE2ETestTables(t, db)
	seedSystemE2EUser(t, db, "user@example.com", "secret123")

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := newSystemE2ETestConfig()
	router, cleanup := SetupRouter(appCtx, cfg, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{}
	accessToken := loginSystemE2EUser(t, client, server.URL, "user@example.com", "secret123")

	// 1. Missing Match Commentary Write returns NOT_FOUND
	commURL := server.URL + "/api/v1/matches/99999/commentary"
	commentaryPayload := map[string]any{
		"minute":    15,
		"eventType": "card",
		"message":   "Yellow Card",
	}
	commBytes, err := json.Marshal(commentaryPayload)
	if err != nil {
		t.Fatalf("failed to marshal commentary payload: %v", err)
	}

	reqComm, err := http.NewRequest(http.MethodPost, commURL, bytes.NewBuffer(commBytes))
	if err != nil {
		t.Fatalf("failed to create commentary request: %v", err)
	}
	reqComm.Header.Set("Content-Type", "application/json")
	reqComm.Header.Set("Authorization", "Bearer "+accessToken)

	respComm, err := client.Do(reqComm)
	if err != nil {
		t.Fatalf("failed to perform commentary write: %v", err)
	}
	defer respComm.Body.Close()

	if respComm.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", respComm.StatusCode)
	}

	var errRes globalSchemas.Response
	if err := json.NewDecoder(respComm.Body).Decode(&errRes); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errRes.Success {
		t.Fatal("expected success to be false")
	}
	if errRes.Error == nil {
		t.Fatal("expected error payload to be non-nil")
	}
	if errRes.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected error code NOT_FOUND, got %v", errRes.Error.Code)
	}

	// 2. Malformed Request returns VALIDATION_ERROR
	reqBad, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/matches", bytes.NewBufferString(`{malformedJSON`))
	if err != nil {
		t.Fatalf("failed to create bad request: %v", err)
	}
	reqBad.Header.Set("Content-Type", "application/json")
	reqBad.Header.Set("Authorization", "Bearer "+accessToken)

	respBad, err := client.Do(reqBad)
	if err != nil {
		t.Fatalf("failed to post malformed JSON: %v", err)
	}
	defer respBad.Body.Close()

	if respBad.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", respBad.StatusCode)
	}

	var badRes globalSchemas.Response
	if err := json.NewDecoder(respBad.Body).Decode(&badRes); err != nil {
		t.Fatalf("failed to decode bad request response: %v", err)
	}
	if badRes.Success {
		t.Fatal("expected success to be false")
	}
	if badRes.Error == nil {
		t.Fatal("expected error payload to be non-nil")
	}
	if badRes.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected error code VALIDATION_ERROR, got %v", badRes.Error.Code)
	}

	// 3. Failed Write Does Not Emit WebSocket Event
	// First, create a valid match to subscribe to
	matchPayload := map[string]any{
		"sport":     "football",
		"homeTeam":  "Team-A",
		"awayTeam":  "Team-B",
		"startTime": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"endTime":   time.Now().Add(80 * time.Minute).Format(time.RFC3339),
	}
	matchBytes, _ := json.Marshal(matchPayload)
	reqM, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/matches", bytes.NewBuffer(matchBytes))
	reqM.Header.Set("Content-Type", "application/json")
	reqM.Header.Set("Authorization", "Bearer "+accessToken)
	respM, err := client.Do(reqM)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	defer respM.Body.Close()
	var mRes globalSchemas.Response
	json.NewDecoder(respM.Body).Decode(&mRes)
	mData := mRes.Data.(map[string]any)
	mID := int64(mData["id"].(float64))

	// Subscribe to it via WS
	u, _ := url.Parse(server.URL)
	wsURL := "ws://" + u.Host + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn.Close()

	var welcomeMsg map[string]any
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	wsConn.ReadJSON(&welcomeMsg) // Read welcome

	wsConn.WriteJSON(map[string]any{"type": "subscribe", "matchId": mID})
	var subResp map[string]any
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	wsConn.ReadJSON(&subResp) // Read subscribed

	// Write commentary that fails validation (empty message after trim)
	invalidCommPayload := map[string]any{
		"minute":    10,
		"eventType": "goal",
		"message":   "   ", // empty message after trim
	}
	invalidBytes, _ := json.Marshal(invalidCommPayload)
	reqInvalid, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/matches/"+strconv.FormatInt(mID, 10)+"/commentary", bytes.NewBuffer(invalidBytes))
	reqInvalid.Header.Set("Content-Type", "application/json")
	reqInvalid.Header.Set("Authorization", "Bearer "+accessToken)
	respInvalid, err := client.Do(reqInvalid)
	if err != nil {
		t.Fatalf("failed to post invalid commentary: %v", err)
	}
	defer respInvalid.Body.Close()

	if respInvalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", respInvalid.StatusCode)
	}

	// Verify WebSocket received nothing
	wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, errRead := wsConn.ReadMessage()
	if errRead == nil {
		t.Fatal("expected no websocket message for failed commentary write")
	}

	// 4. Readiness Returns Failure (503) When DB Unavailable (simulated with nil DB)
	routerNilDB, cleanupNilDB := SetupRouter(appCtx, cfg, nil)
	defer cleanupNilDB()
	serverNilDB := httptest.NewServer(routerNilDB)
	defer serverNilDB.Close()

	respReady, err := client.Get(serverNilDB.URL + "/health/ready")
	if err != nil {
		t.Fatalf("failed to call readiness with nil DB: %v", err)
	}
	defer respReady.Body.Close()

	if respReady.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness status 503, got %d", respReady.StatusCode)
	}

	var readyRes globalSchemas.Response
	if err := json.NewDecoder(respReady.Body).Decode(&readyRes); err != nil {
		t.Fatalf("failed to decode readiness response: %v", err)
	}
	if readyRes.Success {
		t.Fatal("expected readiness success to be false")
	}
	if readyRes.Error == nil {
		t.Fatal("expected error payload to be non-nil")
	}
	if readyRes.Error.Code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("expected error code SERVICE_UNAVAILABLE, got %v", readyRes.Error.Code)
	}
}

func readSystemE2EEvents(t *testing.T, conn *websocket.Conn) []map[string]any {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	lines := bytes.Split(raw, []byte{'\n'})
	var events []map[string]any
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("failed to unmarshal websocket event: %v. Raw was: %s", err, string(line))
		}
		events = append(events, ev)
	}

	return events
}

func seedSystemE2EUser(t *testing.T, db *gorm.DB, email, password string) *authModels.User {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash system e2e password: %v", err)
	}

	user := &authModels.User{
		Email:        email,
		Name:         "System E2E User",
		PasswordHash: string(passwordHash),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed system e2e user: %v", err)
	}

	return user
}

func loginSystemE2EUser(t *testing.T, client *http.Client, baseURL, email, password string) string {
	t.Helper()

	body := []byte(`{"email":"` + email + `","password":"` + password + `"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/login", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create system e2e login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute system e2e login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected system e2e login status 200, got %d", resp.StatusCode)
	}

	var payload globalSchemas.Response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode system e2e login response: %v", err)
	}

	data := payload.Data.(map[string]any)
	return data["accessToken"].(string)
}

func newSystemE2ETestConfig() *config.Config {
	cfg := config.LoadConfig()
	cfg.AppEnv = "test"
	cfg.AllowedOrigins = []string{"http://localhost:3000"}
	cfg.RateLimitRPS = 1000
	cfg.RateLimitBurst = 1000
	cfg.JWTAccessSecret = "system-e2e-access-secret"
	cfg.JWTRefreshSecret = "system-e2e-refresh-secret"
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

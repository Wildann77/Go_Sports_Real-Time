package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	commentaryHandlers "sports-dashboard/internal/features/commentary/handlers"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	commentaryRepositories "sports-dashboard/internal/features/commentary/repositories"
	commentaryServices "sports-dashboard/internal/features/commentary/services"
	matchHandlers "sports-dashboard/internal/features/matches/handlers"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchRepositories "sports-dashboard/internal/features/matches/repositories"
	matchServices "sports-dashboard/internal/features/matches/services"
	"sports-dashboard/internal/features/realtime/hub"
)

func TestWebSocketHandlerUpgradeSuccessAndWelcomeMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testHub, cleanupHub := startRealtimeTestHub(t)
	defer cleanupHub()

	router := gin.New()
	handler := NewWebSocketHandler(testHub, []string{"http://localhost:3000"}, 1024)
	router.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(router)
	defer server.Close()

	conn := openRealtimeWebSocket(t, server.URL, "/ws", "http://localhost:3000")
	defer conn.Close()

	message := readRealtimeMessage(t, conn)
	if message["type"] != "welcome" {
		t.Fatalf("expected welcome event, got %#v", message["type"])
	}
}

func TestWebSocketHandlerRejectsInvalidOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testHub, cleanupHub := startRealtimeTestHub(t)
	defer cleanupHub()

	router := gin.New()
	handler := NewWebSocketHandler(testHub, []string{"http://localhost:3000"}, 1024)
	router.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := httpURLToWebSocket(server.URL, "/ws")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"https://evil.example.com"}})
	if err == nil {
		t.Fatal("expected websocket upgrade failure for invalid origin")
	}
	if resp == nil {
		t.Fatal("expected handshake response for invalid origin")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestWebSocketHandlerPayloadSizeLimitClosesConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testHub, cleanupHub := startRealtimeTestHub(t)
	defer cleanupHub()

	router := gin.New()
	handler := NewWebSocketHandler(testHub, []string{"http://localhost:3000"}, 8)
	router.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(router)
	defer server.Close()

	conn := openRealtimeWebSocket(t, server.URL, "/ws", "http://localhost:3000")
	defer conn.Close()

	_ = readRealtimeMessage(t, conn) // welcome

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping","pad":"too-large"}`)); err != nil {
		t.Fatalf("failed to write oversize message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close after oversized payload")
	}
}

func TestRealtimeIntegrationBroadcastFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openRealtimeFeatureTestDB(t)
	resetRealtimeFeatureTables(t, db)

	router, cleanup := newRealtimeIntegrationRouter(t, db)
	defer cleanup()

	server := httptest.NewServer(router)
	defer server.Close()

	matchID := createRealtimeIntegrationMatch(t, server.URL)

	subscriberConn := openRealtimeWebSocket(t, server.URL, "/ws", "http://localhost:3000")
	defer subscriberConn.Close()
	_ = readRealtimeMessage(t, subscriberConn)

	unrelatedConn := openRealtimeWebSocket(t, server.URL, "/ws", "http://localhost:3000")
	defer unrelatedConn.Close()
	_ = readRealtimeMessage(t, unrelatedConn)

	writeRealtimeJSON(t, subscriberConn, map[string]any{
		"type":    "subscribe",
		"matchId": matchID,
	})
	subscribedMsg := readRealtimeMessage(t, subscriberConn)
	if subscribedMsg["type"] != "subscribed" {
		t.Fatalf("expected subscribed event, got %#v", subscribedMsg["type"])
	}

	writeRealtimeJSON(t, unrelatedConn, map[string]any{
		"type":    "subscribe",
		"matchId": matchID + 999,
	})
	unrelatedSubscribed := readRealtimeMessage(t, unrelatedConn)
	if unrelatedSubscribed["type"] != "subscribed" {
		t.Fatalf("expected unrelated subscribed event, got %#v", unrelatedSubscribed["type"])
	}

	createRealtimeIntegrationCommentary(t, server.URL, matchID, `{"minute":33,"eventType":"goal","message":"Goal","payload":{"homeScore":1,"awayScore":0}}`)

	events := []map[string]any{}
	timeout := time.After(2 * time.Second)
	for len(events) < 2 {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for 2 events, only got %d events: %#v", len(events), events)
		default:
			subscriberConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, raw, err := subscriberConn.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read websocket message: %v", err)
			}
			lines := bytes.Split(raw, []byte{'\n'})
			for _, line := range lines {
				if len(bytes.TrimSpace(line)) == 0 {
					continue
				}
				var ev map[string]any
				if err := json.Unmarshal(line, &ev); err != nil {
					t.Fatalf("failed to decode event: %v. Raw was: %s", err, string(line))
				}
				events = append(events, ev)
			}
		}
	}

	assertEventSequence(t, events[0], events[1])
	assertNoRealtimeMessage(t, unrelatedConn)
}

func startRealtimeTestHub(t *testing.T) (*hub.Hub, func()) {
	t.Helper()

	testHub := hub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	testHub.Start(ctx)

	cleanup := func() {
		testHub.Stop()
		cancel()
		select {
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for test hub shutdown")
		case <-testHub.StopCh():
		}
	}

	return testHub, cleanup
}

func openRealtimeFeatureTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping realtime integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}, &commentaryModels.Commentary{}); err != nil {
		t.Fatalf("failed to migrate realtime integration tables: %v", err)
	}

	return db
}

func resetRealtimeFeatureTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate realtime integration tables: %v", err)
	}
}

func newRealtimeIntegrationRouter(t *testing.T, db *gorm.DB) (*gin.Engine, func()) {
	t.Helper()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		security.RegisterCustomValidators(v)
	}

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	timeoutPolicy := coreDatabase.NewTimeoutPolicy(config.LoadConfig())
	matchRepo := matchRepositories.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepositories.NewCommentaryRepository(db, timeoutPolicy)

	testHub, cleanupHub := startRealtimeTestHub(t)

	matchHandler := matchHandlers.NewMatchHandler(matchServices.NewMatchService(matchRepo))
	commentaryHandler := commentaryHandlers.NewCommentaryHandler(commentaryServices.NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		testHub,
		commentaryServices.NewGormTransactionManager(db),
		timeoutPolicy,
	))
	wsHandler := NewWebSocketHandler(testHub, []string{"http://localhost:3000"}, 1024)

	v1 := router.Group("/api/v1")
	v1.POST("/matches", matchHandler.CreateMatch)
	v1.POST("/matches/:id/commentary", commentaryHandler.CreateCommentary)
	router.GET("/ws", wsHandler.ServeWS)

	return router, cleanupHub
}

func createRealtimeIntegrationMatch(t *testing.T, serverURL string) int64 {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches", bytes.NewBufferString(`{"sport":"football","homeTeam":"Home","awayTeam":"Away","startTime":"2020-01-01T10:00:00Z","endTime":"2099-01-01T12:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")

	w := performRealtimeHTTP(t, serverURL, req)
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode create match response: %v", err)
	}
	data := res["data"].(map[string]any)
	return int64(data["id"].(float64))
}

func createRealtimeIntegrationCommentary(t *testing.T, serverURL string, matchID int64, body string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/"+strconv.FormatInt(matchID, 10)+"/commentary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := performRealtimeHTTP(t, serverURL, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected commentary create 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func performRealtimeHTTP(t *testing.T, serverURL string, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	target, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	recorder := httptest.NewRecorder()
	client := &http.Client{}
	httpReq, err := http.NewRequest(req.Method, req.URL.String(), req.Body)
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}
	httpReq.Header = req.Header.Clone()

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to perform http request: %v", err)
	}
	defer resp.Body.Close()

	recorder.Code = resp.StatusCode
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatalf("failed to read http response body: %v", err)
	}
	recorder.Body = body
	return recorder
}

func openRealtimeWebSocket(t *testing.T, serverURL, path, origin string) *websocket.Conn {
	t.Helper()

	wsURL := httpURLToWebSocket(serverURL, path)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	return conn
}

func httpURLToWebSocket(serverURL, path string) string {
	u, _ := url.Parse(serverURL)
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return scheme + "://" + u.Host + path
}

func writeRealtimeJSON(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()

	if err := conn.WriteJSON(payload); err != nil {
		t.Fatalf("failed to write websocket json: %v", err)
	}
}

func readRealtimeMessage(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("failed to decode websocket message: %v", err)
	}
	return msg
}

func assertEventSequence(t *testing.T, firstEvent, secondEvent map[string]any) {
	t.Helper()

	if firstEvent["type"] != "commentary.created" {
		t.Fatalf("expected first event commentary.created, got %#v", firstEvent["type"])
	}
	if secondEvent["type"] != "match.updated" {
		t.Fatalf("expected second event match.updated, got %#v", secondEvent["type"])
	}

	firstData, ok := firstEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected commentary.created data map, got %T", firstEvent["data"])
	}
	if firstData["message"] != "Goal" {
		t.Fatalf("expected commentary message Goal, got %#v", firstData["message"])
	}

	secondData, ok := secondEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected match.updated data map, got %T", secondEvent["data"])
	}
	if secondData["homeScore"] != float64(1) || secondData["awayScore"] != float64(0) {
		t.Fatalf("expected score update 1-0, got %#v", secondData)
	}
}

func assertNoRealtimeMessage(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected no unrelated room message, but message arrived")
	}
	if !websocket.IsUnexpectedCloseError(err) && !isTimeoutError(err) {
		// normal timeout path fine
	}
}

func isTimeoutError(err error) bool {
	type timeout interface{ Timeout() bool }
	if te, ok := err.(timeout); ok {
		return te.Timeout()
	}
	return false
}

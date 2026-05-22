package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"sports-dashboard/internal/core/config"
)

func TestSetupRouterSmoke(t *testing.T) {
	cfg := &config.Config{
		AppEnv:                "test",
		AllowedOrigins:        []string{"http://localhost:3000"},
		RateLimitRPS:          1000,
		RateLimitBurst:        1000,
		WsMaxPayloadBytes:     1024,
		DbQueryTimeoutSeconds: 1,
		DbTxTimeoutSeconds:    1,
	}

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router, cleanup := SetupRouter(appCtx, cfg, nil)
	if router == nil {
		t.Fatal("expected router, got nil")
	}
	if cleanup == nil {
		t.Fatal("expected cleanup function, got nil")
	}

	t.Cleanup(cleanup)

	expectedRoutes := map[string]string{
		"GET /health":                         "",
		"GET /health/live":                    "",
		"GET /health/ready":                   "",
		"POST /api/v1/api-keys":               "",
		"GET /api/v1/api-keys":                "",
		"DELETE /api/v1/api-keys/:id":         "",
		"POST /api/v1/matches":                "",
		"GET /api/v1/matches":                 "",
		"GET /api/v1/matches/:id":             "",
		"GET /api/v1/matches/:id/commentary":  "",
		"POST /api/v1/matches/:id/commentary": "",
		"GET /ws":                             "",
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expectedRoutes[key]; ok {
			expectedRoutes[key] = route.Handler
		}
	}

	for routeKey, handler := range expectedRoutes {
		if handler == "" {
			t.Fatalf("expected route %s to be registered", routeKey)
		}
	}

	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	resHealth := httptest.NewRecorder()
	router.ServeHTTP(resHealth, reqHealth)
	if resHealth.Code != http.StatusOK {
		t.Fatalf("expected /health status 200, got %d", resHealth.Code)
	}

	reqHealthLive := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	resHealthLive := httptest.NewRecorder()
	router.ServeHTTP(resHealthLive, reqHealthLive)
	if resHealthLive.Code != http.StatusOK {
		t.Fatalf("expected /health/live status 200, got %d", resHealthLive.Code)
	}

	reqReady := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	resReady := httptest.NewRecorder()
	router.ServeHTTP(resReady, reqReady)
	if resReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /health/ready status 503 with nil db, got %d", resReady.Code)
	}

	cleanup()
	cancel()
}

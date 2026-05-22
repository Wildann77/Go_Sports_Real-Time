package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	healthServices "sports-dashboard/internal/features/health/services"
	"sports-dashboard/internal/shared/schemas"
)

type fakeHealthService struct {
	livenessStatus  healthServices.Status
	readinessStatus healthServices.Status
	readinessErr    error
}

func (f *fakeHealthService) Liveness() healthServices.Status {
	return f.livenessStatus
}

func (f *fakeHealthService) Readiness(context.Context) (healthServices.Status, error) {
	return f.readinessStatus, f.readinessErr
}

func TestHealthLive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHealthHandler(&fakeHealthService{
		livenessStatus: healthServices.Status{
			Status:  "ok",
			Service: "go-sports-realtime-api",
			Check:   "liveness",
		},
	})
	r := gin.New()
	r.GET("/health", handler.Live)
	r.GET("/health/live", handler.Live)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var res schemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !res.Success {
		t.Error("Expected success=true for health endpoint")
	}

	reqLive, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	wLive := httptest.NewRecorder()
	r.ServeHTTP(wLive, reqLive)

	if wLive.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health/live, got %d", wLive.Code)
	}
	if w.Body.String() != wLive.Body.String() {
		t.Fatalf("expected /health/live to mirror /health response, got %s vs %s", w.Body.String(), wLive.Body.String())
	}
}

func TestHealthReady(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHealthHandler(&fakeHealthService{
		readinessStatus: healthServices.Status{
			Status:  "ok",
			Service: "go-sports-realtime-api",
			Check:   "readiness",
			Dependencies: map[string]string{
				"database": "ok",
			},
		},
	})
	r := gin.New()
	r.Use(exceptions.ErrorHandlerMiddleware())
	r.GET("/health/ready", handler.Ready)

	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var res schemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !res.Success {
		t.Error("Expected success=true for readiness endpoint")
	}
}

func TestHealthReadyFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHealthHandler(&fakeHealthService{
		readinessErr: exceptions.NewServiceUnavailableError("Service is not ready", context.DeadlineExceeded),
	})
	r := gin.New()
	r.Use(exceptions.ErrorHandlerMiddleware())
	r.GET("/health/ready", handler.Ready)

	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var res schemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if res.Success {
		t.Error("Expected success=false for failed readiness endpoint")
	}

	if res.Error == nil || res.Error.Code != exceptions.SERVICE_UNAVAILABLE {
		t.Error("Expected SERVICE_UNAVAILABLE error code for failed readiness endpoint")
	}

	if res.Message != "Service is not ready" {
		t.Fatalf("expected sanitized readiness failure message, got %q", res.Message)
	}
}

package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchRepositories "sports-dashboard/internal/features/matches/repositories"
	matchServices "sports-dashboard/internal/features/matches/services"
)

func TestMatchFeatureRESTEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openMatchFeatureTestDB(t)
	resetMatchFeatureTable(t, db)

	router := newMatchFeatureE2ERouter(db)

	scheduledID := createMatchThroughHTTP(t, router, `{"sport":"football","homeTeam":"Alpha","awayTeam":"Beta","startTime":"2030-01-01T10:00:00Z","endTime":"2030-01-01T12:00:00Z","metadata":{"venue":"north"}}`)
	liveID := createMatchThroughHTTP(t, router, `{"sport":"football","homeTeam":"Gamma","awayTeam":"Delta","startTime":"2020-01-01T10:00:00Z","endTime":"2099-01-01T12:00:00Z","metadata":{"venue":"south"}}`)
	latestScheduledID := createMatchThroughHTTP(t, router, `{"sport":"football","homeTeam":"Omega","awayTeam":"Sigma","startTime":"2031-01-01T10:00:00Z","endTime":"2031-01-01T12:00:00Z","metadata":{"venue":"east"}}`)

	t.Run("fetch single match through http", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+int64ToString(scheduledID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		res := decodeMatchResponse(t, w)
		if !res.Success || res.Error != nil {
			t.Fatalf("expected success response, got %#v", res)
		}
		if res.Message != "Match retrieved successfully" {
			t.Fatalf("expected success message, got %q", res.Message)
		}

		data := assertMatchResponseDataMap(t, res)
		if data["id"] != float64(scheduledID) {
			t.Fatalf("expected id %d, got %#v", scheduledID, data["id"])
		}
		if data["sport"] != "football" {
			t.Fatalf("expected sport football, got %#v", data["sport"])
		}
	})

	t.Run("list matches with filter and limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?status=scheduled&limit=1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		res := decodeMatchResponse(t, w)
		if !res.Success || res.Error != nil {
			t.Fatalf("expected success response, got %#v", res)
		}
		if res.Message != "Matches retrieved successfully" {
			t.Fatalf("expected success message, got %q", res.Message)
		}

		data, ok := res.Data.([]interface{})
		if !ok || len(data) != 1 {
			t.Fatalf("expected one match in data, got %#v", res.Data)
		}

		firstMatch, ok := data[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected match object, got %T", data[0])
		}
		if firstMatch["id"] != float64(latestScheduledID) {
			t.Fatalf("expected latest scheduled id %d, got %#v", latestScheduledID, firstMatch["id"])
		}

		meta, ok := res.Meta.(map[string]interface{})
		if !ok {
			t.Fatalf("expected meta map, got %T", res.Meta)
		}
		if meta["limit"] != float64(1) || meta["count"] != float64(1) {
			t.Fatalf("unexpected meta payload %#v", meta)
		}
	})

	t.Run("list all matches keeps consistent wrapper", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=5", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		res := decodeMatchResponse(t, w)
		if !res.Success || res.Error != nil {
			t.Fatalf("expected success response, got %#v", res)
		}
		data, ok := res.Data.([]interface{})
		if !ok || len(data) != 3 {
			t.Fatalf("expected three matches, got %#v", res.Data)
		}
	})

	_ = liveID
}

func newMatchFeatureE2ERouter(db *gorm.DB) *gin.Engine {
	matchValidatorsOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			security.RegisterCustomValidators(v)
		}
	})

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	timeoutPolicy := coreDatabase.NewTimeoutPolicy(config.LoadConfig())
	matchRepo := matchRepositories.NewMatchRepository(db, timeoutPolicy)
	matchService := matchServices.NewMatchService(matchRepo)
	matchHandler := NewMatchHandler(matchService)

	v1 := router.Group("/api/v1")
	v1.POST("/matches", matchHandler.CreateMatch)
	v1.GET("/matches", matchHandler.GetMatches)
	v1.GET("/matches/:id", matchHandler.GetMatch)

	return router
}

func openMatchFeatureTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping match feature e2e test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}); err != nil {
		t.Fatalf("failed to migrate matches table: %v", err)
	}

	return db
}

func resetMatchFeatureTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate matches table: %v", err)
	}
}

func createMatchThroughHTTP(t *testing.T, router *gin.Engine, body string) int64 {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", w.Code, w.Body.String())
	}

	res := decodeMatchResponse(t, w)
	if !res.Success || res.Error != nil {
		t.Fatalf("expected success response, got %#v", res)
	}
	if res.Message != "Match created successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	if res.Meta != nil {
		t.Fatalf("expected nil meta on create, got %#v", res.Meta)
	}

	data := assertMatchResponseDataMap(t, res)
	return int64(data["id"].(float64))
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

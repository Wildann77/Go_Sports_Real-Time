package handlers

import (
	"bytes"
	"encoding/json"
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
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	commentaryRepositories "sports-dashboard/internal/features/commentary/repositories"
	commentaryServices "sports-dashboard/internal/features/commentary/services"
	matchHandlers "sports-dashboard/internal/features/matches/handlers"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchRepositories "sports-dashboard/internal/features/matches/repositories"
	matchServices "sports-dashboard/internal/features/matches/services"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type featureNoopBroadcaster struct{}

func (featureNoopBroadcaster) BroadcastToRoom(int64, string, any) {}

func TestCommentaryFeatureRESTEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openCommentaryFeatureTestDB(t)
	resetCommentaryFeatureTables(t, db)

	router := newCommentaryFeatureE2ERouter(db)
	matchID := createCommentaryFeatureMatch(t, router)

	commentaryID := createCommentaryThroughHTTP(t, router, matchID, `{"minute":22,"eventType":"goal","message":"Goal","payload":{"homeScore":1,"awayScore":0}}`)
	if commentaryID == 0 {
		t.Fatal("expected commentary id to be assigned")
	}

	t.Run("commentary list reflects write", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+strconv.FormatInt(matchID, 10)+"/commentary?limit=10", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		res := decodeCommentaryResponse(t, w)
		if !res.Success || res.Error != nil {
			t.Fatalf("expected success response, got %#v", res)
		}
		data, ok := res.Data.([]interface{})
		if !ok || len(data) != 1 {
			t.Fatalf("expected one commentary, got %#v", res.Data)
		}
		first, ok := data[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected commentary object, got %T", data[0])
		}
		if first["id"] != float64(commentaryID) {
			t.Fatalf("expected commentary id %d, got %#v", commentaryID, first["id"])
		}
	})

	t.Run("score mutation visible through match read path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/"+strconv.FormatInt(matchID, 10), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		res := matchHandlersDecodeResponse(t, w)
		if !res.Success || res.Error != nil {
			t.Fatalf("expected success response, got %#v", res)
		}
		data := matchHandlersDataMap(t, res)
		if data["homeScore"] != float64(1) || data["awayScore"] != float64(0) {
			t.Fatalf("expected updated score 1-0, got %#v", data)
		}
	})
}

func newCommentaryFeatureE2ERouter(db *gorm.DB) *gin.Engine {
	commentaryValidatorsOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			security.RegisterCustomValidators(v)
		}
	})

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	timeoutPolicy := coreDatabase.NewTimeoutPolicy(config.LoadConfig())
	matchRepo := matchRepositories.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepositories.NewCommentaryRepository(db, timeoutPolicy)

	matchHandler := matchHandlers.NewMatchHandler(matchServices.NewMatchService(matchRepo))
	commentaryHandler := NewCommentaryHandler(commentaryServices.NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		featureNoopBroadcaster{},
		commentaryServices.NewGormTransactionManager(db),
		timeoutPolicy,
	))

	v1 := router.Group("/api/v1")
	v1.POST("/matches", matchHandler.CreateMatch)
	v1.GET("/matches", matchHandler.GetMatches)
	v1.GET("/matches/:id", matchHandler.GetMatch)
	v1.GET("/matches/:id/commentary", commentaryHandler.GetCommentaries)
	v1.POST("/matches/:id/commentary", commentaryHandler.CreateCommentary)

	return router
}

func openCommentaryFeatureTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping commentary feature e2e test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}, &commentaryModels.Commentary{}); err != nil {
		t.Fatalf("failed to migrate commentary feature tables: %v", err)
	}

	return db
}

func resetCommentaryFeatureTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate commentary feature tables: %v", err)
	}
}

func createCommentaryFeatureMatch(t *testing.T, router *gin.Engine) int64 {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches", bytes.NewBufferString(`{"sport":"football","homeTeam":"Alpha","awayTeam":"Beta","startTime":"2020-01-01T10:00:00Z","endTime":"2099-01-01T12:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", w.Code, w.Body.String())
	}

	res := matchHandlersDecodeResponse(t, w)
	data := matchHandlersDataMap(t, res)
	return int64(data["id"].(float64))
}

func createCommentaryThroughHTTP(t *testing.T, router *gin.Engine, matchID int64, body string) int64 {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/"+strconv.FormatInt(matchID, 10)+"/commentary", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", w.Code, w.Body.String())
	}

	res := decodeCommentaryResponse(t, w)
	if !res.Success || res.Error != nil {
		t.Fatalf("expected success response, got %#v", res)
	}
	data := assertCommentaryResponseDataMap(t, res)
	return int64(data["id"].(float64))
}

func matchHandlersDecodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()
	return decodeMatchFeatureResponse(t, recorder)
}

func matchHandlersDataMap(t *testing.T, res globalSchemas.Response) map[string]interface{} {
	t.Helper()
	return decodeMatchFeatureDataMap(t, res)
}

func decodeMatchFeatureResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

func decodeMatchFeatureDataMap(t *testing.T, res globalSchemas.Response) map[string]interface{} {
	t.Helper()

	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", res.Data)
	}

	return data
}

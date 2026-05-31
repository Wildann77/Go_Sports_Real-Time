package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/security"
	matchSchemas "sports-dashboard/internal/features/matches/schemas"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

var matchValidatorsOnce sync.Once

type fakeMatchService struct {
	createMatchFn func(ctx context.Context, req *matchSchemas.CreateMatchRequest) (*matchSchemas.MatchResponse, error)
	getMatchesFn  func(ctx context.Context, query matchSchemas.ListMatchesQuery) ([]*matchSchemas.MatchResponse, globalSchemas.PaginationMeta, error)
	getMatchFn    func(ctx context.Context, id int64) (*matchSchemas.MatchResponse, error)
}

func TestMatchHandlerGetMatchInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newMatchTestRouter(&fakeMatchService{})
	req := httptest.NewRequest(http.MethodGet, "/matches/abc", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertMatchResponseErrorCode(t, w, http.StatusBadRequest, exceptions.VALIDATION_ERROR)
}

func TestMatchHandlerGetMatchNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newMatchTestRouter(&fakeMatchService{
		getMatchFn: func(context.Context, int64) (*matchSchemas.MatchResponse, error) {
			return nil, exceptions.NewNotFoundError("Match not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/matches/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertMatchResponseErrorCode(t, w, http.StatusNotFound, exceptions.NOT_FOUND)
}

func TestMatchHandlerCreateMatchValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newMatchTestRouter(&fakeMatchService{})
	body := []byte(`{"sport":"Bad Sport","homeTeam":"Home","awayTeam":"Away","startTime":"2026-01-01T10:00:00Z","endTime":"2026-01-01T12:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/matches", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertMatchResponseErrorCode(t, w, http.StatusBadRequest, exceptions.VALIDATION_ERROR)
}

func TestMatchHandlerCreateMatchSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	router := newMatchTestRouter(&fakeMatchService{
		createMatchFn: func(_ context.Context, req *matchSchemas.CreateMatchRequest) (*matchSchemas.MatchResponse, error) {
			return &matchSchemas.MatchResponse{
				ID:        10,
				Sport:     req.Sport,
				HomeTeam:  req.HomeTeam,
				AwayTeam:  req.AwayTeam,
				Status:    "scheduled",
				StartTime: req.StartTime,
				EndTime:   req.EndTime,
				Metadata:  req.Metadata,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	})

	body := []byte(`{"sport":"football","homeTeam":"Home","awayTeam":"Away","startTime":"2026-01-01T10:00:00Z","endTime":"2026-01-01T12:00:00Z","metadata":{"venue":"main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/matches", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	res := decodeMatchResponse(t, w)
	if !res.Success {
		t.Fatal("expected success=true")
	}
	if res.Message != "Match created successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	data := assertMatchResponseDataMap(t, res)
	if data["id"] != float64(10) {
		t.Fatalf("expected id 10, got %#v", data["id"])
	}
	if data["sport"] != "football" {
		t.Fatalf("expected sport football, got %#v", data["sport"])
	}
	if res.Meta != nil {
		t.Fatalf("expected nil meta, got %#v", res.Meta)
	}
}

func TestMatchHandlerGetMatchesSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	var gotQuery matchSchemas.ListMatchesQuery

	router := newMatchTestRouter(&fakeMatchService{
		getMatchesFn: func(_ context.Context, query matchSchemas.ListMatchesQuery) ([]*matchSchemas.MatchResponse, globalSchemas.PaginationMeta, error) {
			gotQuery = query
			return []*matchSchemas.MatchResponse{
				{
					ID:        2,
					Sport:     "football",
					HomeTeam:  "A",
					AwayTeam:  "B",
					Status:    "live",
					StartTime: now.Add(-time.Minute),
					EndTime:   now.Add(time.Hour),
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, globalSchemas.PaginationMeta{Limit: query.Limit, Count: 1}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/matches?status=live&limit=1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if gotQuery.Status != "live" || gotQuery.Limit != 1 {
		t.Fatalf("expected service to receive status=live limit=1, got status=%q limit=%d", gotQuery.Status, gotQuery.Limit)
	}

	res := decodeMatchResponse(t, w)
	if !res.Success {
		t.Fatal("expected success=true")
	}
	if res.Message != "Matches retrieved successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	data, ok := res.Data.([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected one match in list response, got %#v", res.Data)
	}
	meta, ok := res.Meta.(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta map, got %T", res.Meta)
	}
	if meta["limit"] != float64(1) || meta["count"] != float64(1) {
		t.Fatalf("unexpected meta payload %#v", meta)
	}
}

func TestMatchHandlerGetMatchSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	router := newMatchTestRouter(&fakeMatchService{
		getMatchFn: func(_ context.Context, id int64) (*matchSchemas.MatchResponse, error) {
			return &matchSchemas.MatchResponse{
				ID:        id,
				Sport:     "football",
				HomeTeam:  "Home",
				AwayTeam:  "Away",
				Status:    "scheduled",
				StartTime: now.Add(time.Hour),
				EndTime:   now.Add(2 * time.Hour),
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/matches/55", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	res := decodeMatchResponse(t, w)
	if !res.Success {
		t.Fatal("expected success=true")
	}
	if res.Message != "Match retrieved successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	data := assertMatchResponseDataMap(t, res)
	if data["id"] != float64(55) {
		t.Fatalf("expected id 55, got %#v", data["id"])
	}
}

func (f *fakeMatchService) CreateMatch(ctx context.Context, req *matchSchemas.CreateMatchRequest) (*matchSchemas.MatchResponse, error) {
	if f.createMatchFn != nil {
		return f.createMatchFn(ctx, req)
	}

	now := time.Now()
	return &matchSchemas.MatchResponse{
		ID:        1,
		Sport:     req.Sport,
		HomeTeam:  req.HomeTeam,
		AwayTeam:  req.AwayTeam,
		Status:    "scheduled",
		StartTime: now,
		EndTime:   now.Add(time.Hour),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (f *fakeMatchService) GetMatches(ctx context.Context, query matchSchemas.ListMatchesQuery) ([]*matchSchemas.MatchResponse, globalSchemas.PaginationMeta, error) {
	if f.getMatchesFn != nil {
		return f.getMatchesFn(ctx, query)
	}
	return []*matchSchemas.MatchResponse{}, globalSchemas.PaginationMeta{}, nil
}

func (f *fakeMatchService) GetMatch(ctx context.Context, id int64) (*matchSchemas.MatchResponse, error) {
	if f.getMatchFn != nil {
		return f.getMatchFn(ctx, id)
	}
	return nil, nil
}

func newMatchTestRouter(service MatchService) *gin.Engine {
	matchValidatorsOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			security.RegisterCustomValidators(v)
		}
	})

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	handler := NewMatchHandler(service)
	router.POST("/matches", handler.CreateMatch)
	router.GET("/matches", handler.GetMatches)
	router.GET("/matches/:id", handler.GetMatch)

	return router
}

func assertMatchResponseErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
	}

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Error == nil {
		t.Fatal("expected error response, got nil")
	}

	if res.Error.Code != expectedCode {
		t.Fatalf("expected error code %s, got %s", expectedCode, res.Error.Code)
	}
}

func decodeMatchResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

func assertMatchResponseDataMap(t *testing.T, res globalSchemas.Response) map[string]interface{} {
	t.Helper()

	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", res.Data)
	}

	return data
}

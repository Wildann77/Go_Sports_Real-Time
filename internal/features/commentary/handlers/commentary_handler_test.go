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
	commentarySchemas "sports-dashboard/internal/features/commentary/schemas"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

var commentaryValidatorsOnce sync.Once

type fakeCommentaryService struct {
	getCommentariesFn  func(ctx context.Context, matchID int64, limit int) ([]*commentarySchemas.CommentaryResponse, error)
	createCommentaryFn func(ctx context.Context, matchID int64, req *commentarySchemas.CreateCommentaryRequest) (*commentarySchemas.CommentaryResponse, error)
}

func TestCommentaryHandlerGetCommentariesInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newCommentaryTestRouter(&fakeCommentaryService{})
	req := httptest.NewRequest(http.MethodGet, "/matches/abc/commentary", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertResponseErrorCode(t, w, http.StatusBadRequest, exceptions.VALIDATION_ERROR)
}

func TestCommentaryHandlerGetCommentariesNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newCommentaryTestRouter(&fakeCommentaryService{
		getCommentariesFn: func(context.Context, int64, int) ([]*commentarySchemas.CommentaryResponse, error) {
			return nil, exceptions.NewNotFoundError("Match not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/matches/99/commentary", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertResponseErrorCode(t, w, http.StatusNotFound, exceptions.NOT_FOUND)
}

func TestCommentaryHandlerCreateCommentaryValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newCommentaryTestRouter(&fakeCommentaryService{})
	body := []byte(`{"minute": 12, "eventType": "goal", "message": "   "}`)
	req := httptest.NewRequest(http.MethodPost, "/matches/10/commentary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertResponseErrorCode(t, w, http.StatusBadRequest, exceptions.VALIDATION_ERROR)
}

func TestCommentaryHandlerCreateCommentaryNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newCommentaryTestRouter(&fakeCommentaryService{
		createCommentaryFn: func(context.Context, int64, *commentarySchemas.CreateCommentaryRequest) (*commentarySchemas.CommentaryResponse, error) {
			return nil, exceptions.NewNotFoundError("Match not found")
		},
	})
	body := []byte(`{"minute": 12, "eventType": "goal", "message": "Goal"}`)
	req := httptest.NewRequest(http.MethodPost, "/matches/404/commentary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertResponseErrorCode(t, w, http.StatusNotFound, exceptions.NOT_FOUND)
}

func TestCommentaryHandlerGetCommentariesSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	var gotMatchID int64
	var gotLimit int

	router := newCommentaryTestRouter(&fakeCommentaryService{
		getCommentariesFn: func(_ context.Context, matchID int64, limit int) ([]*commentarySchemas.CommentaryResponse, error) {
			gotMatchID = matchID
			gotLimit = limit
			return []*commentarySchemas.CommentaryResponse{
				{
					ID:        1,
					MatchID:   matchID,
					Minute:    12,
					EventType: "goal",
					Message:   "Goal",
					Payload: map[string]any{
						"homeScore": 1,
					},
					CreatedAt: now,
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/matches/10/commentary?limit=1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if gotMatchID != 10 || gotLimit != 1 {
		t.Fatalf("expected service args matchID=10 limit=1, got %d and %d", gotMatchID, gotLimit)
	}

	res := decodeCommentaryResponse(t, w)
	if !res.Success || res.Error != nil {
		t.Fatalf("expected success response, got %#v", res)
	}
	if res.Message != "Commentaries retrieved successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	data, ok := res.Data.([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected one commentary, got %#v", res.Data)
	}
}

func TestCommentaryHandlerCreateCommentarySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	router := newCommentaryTestRouter(&fakeCommentaryService{
		createCommentaryFn: func(_ context.Context, matchID int64, req *commentarySchemas.CreateCommentaryRequest) (*commentarySchemas.CommentaryResponse, error) {
			return &commentarySchemas.CommentaryResponse{
				ID:        5,
				MatchID:   matchID,
				Minute:    req.Minute,
				EventType: req.EventType,
				Message:   req.Message,
				Payload:   req.Payload,
				CreatedAt: now,
			}, nil
		},
	})

	body := []byte(`{"minute":12,"eventType":"goal","message":"Goal","payload":{"homeScore":1}}`)
	req := httptest.NewRequest(http.MethodPost, "/matches/10/commentary", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	res := decodeCommentaryResponse(t, w)
	if !res.Success || res.Error != nil {
		t.Fatalf("expected success response, got %#v", res)
	}
	if res.Message != "Commentary created successfully" {
		t.Fatalf("expected success message, got %q", res.Message)
	}
	data := assertCommentaryResponseDataMap(t, res)
	if data["id"] != float64(5) || data["matchId"] != float64(10) {
		t.Fatalf("unexpected response data %#v", data)
	}
}

func (f *fakeCommentaryService) GetCommentaries(ctx context.Context, matchID int64, limit int) ([]*commentarySchemas.CommentaryResponse, error) {
	if f.getCommentariesFn != nil {
		return f.getCommentariesFn(ctx, matchID, limit)
	}
	return []*commentarySchemas.CommentaryResponse{}, nil
}

func (f *fakeCommentaryService) CreateCommentary(ctx context.Context, matchID int64, req *commentarySchemas.CreateCommentaryRequest) (*commentarySchemas.CommentaryResponse, error) {
	if f.createCommentaryFn != nil {
		return f.createCommentaryFn(ctx, matchID, req)
	}
	return &commentarySchemas.CommentaryResponse{
		ID:        1,
		MatchID:   matchID,
		Minute:    req.Minute,
		EventType: req.EventType,
		Message:   req.Message,
		Payload:   req.Payload,
		CreatedAt: time.Now(),
	}, nil
}

func newCommentaryTestRouter(service CommentaryService) *gin.Engine {
	commentaryValidatorsOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			security.RegisterCustomValidators(v)
		}
	})

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	handler := NewCommentaryHandler(service)
	router.GET("/matches/:id/commentary", handler.GetCommentaries)
	router.POST("/matches/:id/commentary", handler.CreateCommentary)

	return router
}

func assertResponseErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
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

func decodeCommentaryResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

func assertCommentaryResponseDataMap(t *testing.T, res globalSchemas.Response) map[string]interface{} {
	t.Helper()

	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", res.Data)
	}

	return data
}

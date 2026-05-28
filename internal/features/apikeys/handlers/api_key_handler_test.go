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
	apiKeySchemas "sports-dashboard/internal/features/apikeys/schemas"
	authSchemas "sports-dashboard/internal/features/auth/schemas"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

var apiKeyValidatorsOnce sync.Once

type fakeAPIKeyService struct {
	createFn func(ctx context.Context, userID int64, req *apiKeySchemas.CreateAPIKeyRequest) (*apiKeySchemas.CreateAPIKeyResponse, error)
	listFn   func(ctx context.Context, userID int64) ([]*apiKeySchemas.APIKeyResponse, error)
	revokeFn func(ctx context.Context, userID, keyID int64) error
}

func TestAPIKeyHandlerCreateSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	router := newAPIKeyHandlerTestRouter(&fakeAPIKeyService{
		createFn: func(_ context.Context, userID int64, req *apiKeySchemas.CreateAPIKeyRequest) (*apiKeySchemas.CreateAPIKeyResponse, error) {
			if userID != 15 {
				t.Fatalf("expected userID 15, got %d", userID)
			}
			return &apiKeySchemas.CreateAPIKeyResponse{
				APIKey: "sk_test_secret",
				Key: &apiKeySchemas.APIKeyResponse{
					ID:          1,
					Name:        req.Name,
					KeyPrefix:   "sk_test",
					KeyLastFour: "cret",
					Scopes:      req.Scopes,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			}, nil
		},
	})

	body := []byte(`{"name":"Writer","scopes":["matches:write"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	res := decodeAPIKeyHandlerResponse(t, w)
	if !res.Success || res.Message != "API key created successfully" {
		t.Fatalf("unexpected response %#v", res)
	}
}

func TestAPIKeyHandlerCreateValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAPIKeyHandlerTestRouter(&fakeAPIKeyService{})
	req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader([]byte(`{"name":" ","scopes":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAPIKeyHandlerErrorCode(t, w, http.StatusBadRequest, exceptions.VALIDATION_ERROR)
}

func TestAPIKeyHandlerListSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	router := newAPIKeyHandlerTestRouter(&fakeAPIKeyService{
		listFn: func(_ context.Context, userID int64) ([]*apiKeySchemas.APIKeyResponse, error) {
			if userID != 15 {
				t.Fatalf("expected userID 15, got %d", userID)
			}
			return []*apiKeySchemas.APIKeyResponse{{
				ID:          1,
				Name:        "Writer",
				KeyPrefix:   "sk_test",
				KeyLastFour: "1234",
				Scopes:      []string{apiKeySchemas.ScopeMatchesWrite},
				CreatedAt:   now,
				UpdatedAt:   now,
			}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	res := decodeAPIKeyHandlerResponse(t, w)
	if !res.Success || res.Message != "API keys retrieved successfully" {
		t.Fatalf("unexpected response %#v", res)
	}
}

func TestAPIKeyHandlerRevokeSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotUserID, gotKeyID int64
	router := newAPIKeyHandlerTestRouter(&fakeAPIKeyService{
		revokeFn: func(_ context.Context, userID, keyID int64) error {
			gotUserID = userID
			gotKeyID = keyID
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api-keys/9", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if gotUserID != 15 || gotKeyID != 9 {
		t.Fatalf("expected revoke args userID=15 keyID=9, got %d and %d", gotUserID, gotKeyID)
	}
}

func TestAPIKeyHandlerRevokeNotOwnedReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAPIKeyHandlerTestRouter(&fakeAPIKeyService{
		revokeFn: func(_ context.Context, _, _ int64) error {
			return exceptions.NewNotFoundError("API key not found")
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api-keys/7", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAPIKeyHandlerErrorCode(t, w, http.StatusNotFound, exceptions.NOT_FOUND)
}

func (f *fakeAPIKeyService) CreateAPIKey(ctx context.Context, userID int64, req *apiKeySchemas.CreateAPIKeyRequest) (*apiKeySchemas.CreateAPIKeyResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, userID, req)
	}
	return nil, nil
}

func (f *fakeAPIKeyService) ListAPIKeys(ctx context.Context, userID int64) ([]*apiKeySchemas.APIKeyResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, userID)
	}
	return nil, nil
}

func (f *fakeAPIKeyService) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, userID, keyID)
	}
	return nil
}

func newAPIKeyHandlerTestRouter(service APIKeyService) *gin.Engine {
	apiKeyValidatorsOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			security.RegisterCustomValidators(v)
		}
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth_user", &authSchemas.AuthenticatedUser{
			UserID: 15,
			User:   &authSchemas.UserResponse{ID: 15, Email: "user@example.com", Name: "User"},
		})
		c.Next()
	})
	router.Use(exceptions.ErrorHandlerMiddleware())

	handler := NewAPIKeyHandler(service)
	router.POST("/api-keys", handler.CreateAPIKey)
	router.GET("/api-keys", handler.ListAPIKeys)
	router.DELETE("/api-keys/:id", handler.RevokeAPIKey)
	return router
}

func decodeAPIKeyHandlerResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

func assertAPIKeyHandlerErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
	}

	res := decodeAPIKeyHandlerResponse(t, recorder)
	if res.Error == nil || res.Error.Code != expectedCode {
		t.Fatalf("expected error code %s, got %#v", expectedCode, res.Error)
	}
}

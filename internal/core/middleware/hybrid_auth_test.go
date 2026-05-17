package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	apiKeySchemas "sports-dashboard/internal/features/apikeys/schemas"
	authSchemas "sports-dashboard/internal/features/auth/schemas"
	sharedSchemas "sports-dashboard/internal/shared/schemas"
)

type fakeAPIKeyVerifier struct {
	apiKey *apiKeySchemas.AuthenticatedAPIKey
	err    error
}

func TestRequireJWTOrAPIKeyAllowsValidJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newHybridAuthTestRouter(
		&fakeAccessTokenVerifier{
			authUser: &authSchemas.AuthenticatedUser{
				UserID: 7,
				User:   &authSchemas.UserResponse{ID: 7, Email: "user@example.com", Name: "User"},
			},
		},
		&fakeAPIKeyVerifier{},
	)

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer jwt-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := decodeHybridAuthResponse(t, w)
	data := body.Data.(map[string]any)
	if data["kind"] != "jwt" {
		t.Fatalf("expected jwt kind, got %#v", data)
	}
}

func TestRequireJWTOrAPIKeyAllowsValidAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newHybridAuthTestRouter(
		&fakeAccessTokenVerifier{},
		&fakeAPIKeyVerifier{
			apiKey: &apiKeySchemas.AuthenticatedAPIKey{
				KeyID:  11,
				UserID: 22,
				Name:   "machine",
				Scopes: []string{apiKeySchemas.ScopeMatchesWrite},
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer sk_test_abc123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := decodeHybridAuthResponse(t, w)
	data := body.Data.(map[string]any)
	if data["kind"] != "apiKey" {
		t.Fatalf("expected apiKey kind, got %#v", data)
	}
}

func TestRequireJWTOrAPIKeyRejectsMalformedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newHybridAuthTestRouter(&fakeAccessTokenVerifier{}, &fakeAPIKeyVerifier{})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Token nope")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAuthMiddlewareErrorCode(t, w, http.StatusUnauthorized, exceptions.UNAUTHORIZED)
}

func TestRequireJWTOrAPIKeyRejectsMissingScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newHybridAuthTestRouter(
		&fakeAccessTokenVerifier{},
		&fakeAPIKeyVerifier{
			err: exceptions.NewForbiddenError("API key does not have required scope"),
		},
	)
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer sk_test_missing_scope")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAuthMiddlewareErrorCode(t, w, http.StatusForbidden, exceptions.FORBIDDEN)
}

func TestRequireJWTOrAPIKeyRejectsExpiredAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newHybridAuthTestRouter(
		&fakeAccessTokenVerifier{},
		&fakeAPIKeyVerifier{
			err: exceptions.NewUnauthorizedError("Invalid or expired API key"),
		},
	)
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer sk_test_expired")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAuthMiddlewareErrorCode(t, w, http.StatusUnauthorized, exceptions.UNAUTHORIZED)
}

func (f *fakeAPIKeyVerifier) VerifyAPIKey(_ context.Context, _ string, _ string) (*apiKeySchemas.AuthenticatedAPIKey, error) {
	return f.apiKey, f.err
}

func newHybridAuthTestRouter(accessVerifier AccessTokenVerifier, apiKeyVerifier APIKeyVerifier) *gin.Engine {
	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())
	router.GET("/private", RequireJWTOrAPIKey(accessVerifier, apiKeyVerifier, apiKeySchemas.ScopeMatchesWrite), func(c *gin.Context) {
		if authUser, ok := CurrentAuthenticatedUser(c); ok && authUser != nil {
			sharedSchemas.Success(c, http.StatusOK, "Authorized", map[string]any{"kind": "jwt", "userId": authUser.UserID})
			return
		}
		if authAPIKey, ok := CurrentAuthenticatedAPIKey(c); ok && authAPIKey != nil {
			sharedSchemas.Success(c, http.StatusOK, "Authorized", map[string]any{"kind": "apiKey", "keyId": authAPIKey.KeyID})
			return
		}
		c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
	})
	return router
}

func decodeHybridAuthResponse(t *testing.T, recorder *httptest.ResponseRecorder) sharedSchemas.Response {
	t.Helper()

	var res sharedSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

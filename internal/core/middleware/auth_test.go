package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	authSchemas "sports-dashboard/internal/features/auth/schemas"
	sharedSchemas "sports-dashboard/internal/shared/schemas"
)

type fakeAccessTokenVerifier struct {
	authUser *authSchemas.AuthenticatedUser
	err      error
}

func TestAuthRequiredRejectsMissingAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthMiddlewareTestRouter(&fakeAccessTokenVerifier{})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAuthMiddlewareErrorCode(t, w, http.StatusUnauthorized, exceptions.UNAUTHORIZED)
}

func TestAuthRequiredRejectsInvalidAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthMiddlewareTestRouter(&fakeAccessTokenVerifier{})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Token nope")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertAuthMiddlewareErrorCode(t, w, http.StatusUnauthorized, exceptions.UNAUTHORIZED)
}

func TestAuthRequiredAllowsValidBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthMiddlewareTestRouter(&fakeAccessTokenVerifier{
		authUser: &authSchemas.AuthenticatedUser{
			UserID: 7,
			User: &authSchemas.UserResponse{
				ID:    7,
				Email: "user@example.com",
				Name:  "User",
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res sharedSchemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success=true")
	}
}

func (f *fakeAccessTokenVerifier) VerifyAccessToken(_ context.Context, _ string) (*authSchemas.AuthenticatedUser, error) {
	return f.authUser, f.err
}

func newAuthMiddlewareTestRouter(verifier AccessTokenVerifier) *gin.Engine {
	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())
	router.GET("/private", AuthRequired(verifier), func(c *gin.Context) {
		authUser, ok := CurrentAuthenticatedUser(c)
		if !ok || authUser == nil {
			c.Error(exceptions.NewUnauthorizedError("Unauthorized"))
			return
		}
		sharedSchemas.Success(c, http.StatusOK, "Authorized", map[string]any{"userId": authUser.UserID})
	})
	return router
}

func assertAuthMiddlewareErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
	}

	var res sharedSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error payload")
	}
	if res.Error.Code != expectedCode {
		t.Fatalf("expected error code %s, got %s", expectedCode, res.Error.Code)
	}
}

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/config"
	"sports-dashboard/internal/core/exceptions"
	authSchemas "sports-dashboard/internal/features/auth/schemas"
	authServices "sports-dashboard/internal/features/auth/services"
	globalSchemas "sports-dashboard/internal/shared/schemas"
)

type fakeAuthService struct {
	loginFn               func(ctx context.Context, req *authSchemas.LoginRequest, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error)
	getCurrentUserFn      func(ctx context.Context, authUser *authSchemas.AuthenticatedUser) (*authSchemas.UserResponse, error)
	refreshTokenFn        func(ctx context.Context, rawToken string, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error)
	logoutCurrentDeviceFn func(ctx context.Context, rawToken string) error
	logoutAllDevicesFn    func(ctx context.Context, userID int64) error
}

func TestAuthHandlerLoginSetsRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthHandlerTestRouter(t, &fakeAuthService{
		loginFn: func(_ context.Context, req *authSchemas.LoginRequest, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error) {
			if req.Email != "user@example.com" || req.Password != "secret123" {
				t.Fatalf("unexpected login request %#v", req)
			}
			if metadata.UserAgent != "Browser/1.0" {
				t.Fatalf("expected user-agent metadata, got %#v", metadata)
			}
			return &authSchemas.AuthTokenResponse{
				AccessToken: "access-token",
				User: &authSchemas.UserResponse{
					ID:    1,
					Email: req.Email,
					Name:  "User",
				},
			}, "refresh-token", nil
		},
	})

	body := []byte(`{"email":"user@example.com","password":"secret123"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Browser/1.0")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	res := decodeAuthHandlerResponse(t, w)
	if !res.Success || res.Message != "Login successful" {
		t.Fatalf("unexpected response %#v", res)
	}
	if cookie := w.Header().Get("Set-Cookie"); cookie == "" || !containsAll(cookie, "refresh_token=refresh-token", "HttpOnly") {
		t.Fatalf("expected refresh cookie header, got %q", cookie)
	}
}

func TestAuthHandlerLoginFailureReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthHandlerTestRouter(t, &fakeAuthService{
		loginFn: func(_ context.Context, _ *authSchemas.LoginRequest, _ authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error) {
			return nil, "", exceptions.NewUnauthorizedError("Invalid email or password")
		},
	})

	body := []byte(`{"email":"user@example.com","password":"wrong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}

	res := decodeAuthHandlerResponse(t, w)
	if res.Error == nil || res.Error.Code != exceptions.UNAUTHORIZED {
		t.Fatalf("expected unauthorized error payload, got %#v", res)
	}
}

func TestAuthHandlerRefreshTokenRotatesCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthHandlerTestRouter(t, &fakeAuthService{
		refreshTokenFn: func(_ context.Context, rawToken string, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error) {
			if rawToken != "old-refresh" {
				t.Fatalf("expected refresh cookie old-refresh, got %q", rawToken)
			}
			if metadata.IPAddress == "" {
				t.Fatal("expected client ip metadata")
			}
			return &authSchemas.AuthTokenResponse{
				AccessToken: "new-access",
				User:        &authSchemas.UserResponse{ID: 2, Email: "user@example.com", Name: "User"},
			}, "new-refresh", nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh-token", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-refresh"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	res := decodeAuthHandlerResponse(t, w)
	if !res.Success || res.Message != "Token refreshed successfully" {
		t.Fatalf("unexpected response %#v", res)
	}
	if cookie := w.Header().Get("Set-Cookie"); cookie == "" || !containsAll(cookie, "refresh_token=new-refresh", "HttpOnly") {
		t.Fatalf("expected rotated refresh cookie, got %q", cookie)
	}
}

func TestAuthHandlerLogoutClearsRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotToken string
	router := newAuthHandlerTestRouter(t, &fakeAuthService{
		logoutCurrentDeviceFn: func(_ context.Context, rawToken string) error {
			gotToken = rawToken
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "refresh-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if gotToken != "refresh-token" {
		t.Fatalf("expected logout service to receive refresh-token, got %q", gotToken)
	}
	if cookie := w.Header().Get("Set-Cookie"); cookie == "" || !containsAll(cookie, "refresh_token=", "Max-Age=0") {
		t.Fatalf("expected cleared refresh cookie, got %q", cookie)
	}
}

func TestAuthHandlerMeReturnsAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newAuthHandlerTestRouter(t, &fakeAuthService{
		getCurrentUserFn: func(_ context.Context, authUser *authSchemas.AuthenticatedUser) (*authSchemas.UserResponse, error) {
			if authUser.UserID != 99 {
				t.Fatalf("expected auth user id 99, got %d", authUser.UserID)
			}
			return authUser.User, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	res := decodeAuthHandlerResponse(t, w)
	if !res.Success || res.Message != "Current user retrieved successfully" {
		t.Fatalf("unexpected response %#v", res)
	}
}

func (f *fakeAuthService) Login(ctx context.Context, req *authSchemas.LoginRequest, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, req, metadata)
	}
	return nil, "", nil
}

func (f *fakeAuthService) GetCurrentUser(ctx context.Context, authUser *authSchemas.AuthenticatedUser) (*authSchemas.UserResponse, error) {
	if f.getCurrentUserFn != nil {
		return f.getCurrentUserFn(ctx, authUser)
	}
	return nil, nil
}

func (f *fakeAuthService) RefreshToken(ctx context.Context, rawToken string, metadata authServices.RequestMetadata) (*authSchemas.AuthTokenResponse, string, error) {
	if f.refreshTokenFn != nil {
		return f.refreshTokenFn(ctx, rawToken, metadata)
	}
	return nil, "", nil
}

func (f *fakeAuthService) LogoutCurrentDevice(ctx context.Context, rawToken string) error {
	if f.logoutCurrentDeviceFn != nil {
		return f.logoutCurrentDeviceFn(ctx, rawToken)
	}
	return nil
}

func (f *fakeAuthService) LogoutAllDevices(ctx context.Context, userID int64) error {
	if f.logoutAllDevicesFn != nil {
		return f.logoutAllDevicesFn(ctx, userID)
	}
	return nil
}

func newAuthHandlerTestRouter(t *testing.T, service AuthService) *gin.Engine {
	t.Helper()

	cfg := &config.Config{
		RefreshCookieName:     "refresh_token",
		RefreshCookiePath:     "/",
		RefreshCookieSameSite: "lax",
		RefreshTokenTTLDays:   30,
	}

	router := gin.New()
	router.Use(exceptions.ErrorHandlerMiddleware())

	handler := NewAuthHandler(service, cfg)
	router.POST("/login", handler.Login)
	router.POST("/refresh-token", handler.RefreshToken)
	router.POST("/logout", handler.Logout)
	router.GET("/me", func(c *gin.Context) {
		c.Set("auth_user", &authSchemas.AuthenticatedUser{
			UserID: 99,
			User: &authSchemas.UserResponse{
				ID:        99,
				Email:     "user@example.com",
				Name:      "User",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		})
		handler.Me(c)
	})

	return router
}

func decodeAuthHandlerResponse(t *testing.T, recorder *httptest.ResponseRecorder) globalSchemas.Response {
	t.Helper()

	var res globalSchemas.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return res
}

func containsAll(raw string, parts ...string) bool {
	for _, part := range parts {
		if !bytes.Contains([]byte(raw), []byte(part)) {
			return false
		}
	}
	return true
}

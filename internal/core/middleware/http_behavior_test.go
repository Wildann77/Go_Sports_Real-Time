package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"sports-dashboard/internal/core/exceptions"
	sharedSchemas "sports-dashboard/internal/shared/schemas"
)

func TestBodyLimitReturnsExpectedFailure(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(BodyLimit(4))
	router.POST("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/limited", strings.NewReader("12345"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", w.Code)
	}

	res := decodeMiddlewareResponse(t, w)
	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Request body too large" {
		t.Fatalf("expected request body too large message, got %q", res.Message)
	}
	if res.Error == nil {
		t.Fatal("expected error payload")
	}
	if res.Error.Code != exceptions.BAD_REQUEST {
		t.Fatalf("expected code %s, got %s", exceptions.BAD_REQUEST, res.Error.Code)
	}
}

func TestRecoverConvertsPanicToSanitizedError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Recover())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	res := decodeMiddlewareResponse(t, w)
	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Internal Server Error" {
		t.Fatalf("expected internal server error message, got %q", res.Message)
	}
	if res.Error == nil {
		t.Fatal("expected error payload")
	}
	if res.Error.Code != exceptions.INTERNAL_SERVER_ERROR {
		t.Fatalf("expected code %s, got %s", exceptions.INTERNAL_SERVER_ERROR, res.Error.Code)
	}
}

func TestCORSAllowlistBehavior(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	allowedOrigin := "https://app.example.com"

	router := gin.New()
	router.Use(CORS([]string{allowedOrigin}))
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("allowed origin gets cors headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/resource", nil)
		req.Header.Set("Origin", allowedOrigin)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
			t.Fatalf("expected allow origin header %q, got %q", allowedOrigin, got)
		}
	})

	t.Run("disallowed origin gets no cors allow origin header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/resource", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no allow origin header, got %q", got)
		}
	})

	t.Run("options request returns no content for allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
		req.Header.Set("Origin", allowedOrigin)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
			t.Fatalf("expected allow origin header %q, got %q", allowedOrigin, got)
		}
	})
}

func TestRateLimitMiddlewareReturnsRateLimitedEnvelope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NewIPRateLimiter(0, 0).Middleware())
	router.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}

	res := decodeMiddlewareResponse(t, w)
	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Too many requests" {
		t.Fatalf("expected too many requests message, got %q", res.Message)
	}
	if res.Error == nil {
		t.Fatal("expected error payload")
	}
	if res.Error.Code != exceptions.RATE_LIMITED {
		t.Fatalf("expected code %s, got %s", exceptions.RATE_LIMITED, res.Error.Code)
	}
}

func decodeMiddlewareResponse(t *testing.T, w *httptest.ResponseRecorder) sharedSchemas.Response {
	t.Helper()

	var res sharedSchemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	return res
}

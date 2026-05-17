package exceptions

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sharedSchemas "sports-dashboard/internal/shared/schemas"
)

func TestErrorHandlerMiddlewareConvertsAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ErrorHandlerMiddleware())
	router.GET("/app-error", func(c *gin.Context) {
		c.Error(NewAppError(VALIDATION_ERROR, "Validation failed", http.StatusBadRequest, []ValidationErrorDetail{
			{Field: "sport", Message: "required"},
		}))
	})

	req := httptest.NewRequest(http.MethodGet, "/app-error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var res sharedSchemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Validation failed" {
		t.Fatalf("expected validation failed message, got %q", res.Message)
	}
	if res.Error == nil {
		t.Fatal("expected error object")
	}
	if res.Error.Code != VALIDATION_ERROR {
		t.Fatalf("expected code %s, got %s", VALIDATION_ERROR, res.Error.Code)
	}
}

func TestErrorHandlerMiddlewareConvertsUnknownErrorToInternalServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ErrorHandlerMiddleware())
	router.GET("/unknown-error", func(c *gin.Context) {
		c.Error(errors.New("boom"))
	})

	req := httptest.NewRequest(http.MethodGet, "/unknown-error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var res sharedSchemas.Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Internal Server Error" {
		t.Fatalf("expected internal server error message, got %q", res.Message)
	}
	if res.Error == nil {
		t.Fatal("expected error object")
	}
	if res.Error.Code != INTERNAL_SERVER_ERROR {
		t.Fatalf("expected code %s, got %s", INTERNAL_SERVER_ERROR, res.Error.Code)
	}
}

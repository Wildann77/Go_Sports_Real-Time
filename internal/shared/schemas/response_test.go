package schemas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSuccessResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, http.StatusOK, "Test successful", map[string]string{"foo": "bar"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !res.Success {
		t.Fatal("expected success=true")
	}
	if res.Message != "Test successful" {
		t.Fatalf("expected message 'Test successful', got %q", res.Message)
	}

	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	if data["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %#v", data)
	}
	if res.Meta != nil {
		t.Fatalf("expected nil meta, got %#v", res.Meta)
	}
	if res.Error != nil {
		t.Fatalf("expected nil error, got %#v", res.Error)
	}
}

func TestSuccessWithMetaResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SuccessWithMeta(
		c,
		http.StatusCreated,
		"Created",
		[]string{"one", "two"},
		map[string]interface{}{"count": 2, "limit": 10},
	)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	var res Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !res.Success {
		t.Fatal("expected success=true")
	}
	if res.Error != nil {
		t.Fatalf("expected nil error, got %#v", res.Error)
	}

	data, ok := res.Data.([]interface{})
	if !ok || len(data) != 2 {
		t.Fatalf("expected 2-item slice data, got %#v", res.Data)
	}

	meta, ok := res.Meta.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map meta, got %T", res.Meta)
	}
	if meta["count"] != float64(2) || meta["limit"] != float64(10) {
		t.Fatalf("unexpected meta payload: %#v", meta)
	}
}

func TestErrorResponseShapeAndSanitizedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, http.StatusBadRequest, "Validation failed", "VALIDATION_ERROR", []map[string]string{
		{"field": "sport", "message": "required"},
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var res Response
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Success {
		t.Fatal("expected success=false")
	}
	if res.Message != "Validation failed" {
		t.Fatalf("expected validation message, got %q", res.Message)
	}
	if res.Data != nil {
		t.Fatalf("expected nil data, got %#v", res.Data)
	}
	if res.Meta != nil {
		t.Fatalf("expected nil meta, got %#v", res.Meta)
	}
	if res.Error == nil {
		t.Fatal("expected error object")
	}
	if res.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected error code VALIDATION_ERROR, got %s", res.Error.Code)
	}

	details, ok := res.Error.Details.([]interface{})
	if !ok || len(details) != 1 {
		t.Fatalf("expected sanitized details slice, got %#v", res.Error.Details)
	}
}

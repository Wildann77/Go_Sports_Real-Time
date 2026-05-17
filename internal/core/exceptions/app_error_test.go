package exceptions

import (
	"net/http"
	"testing"
)

func TestAppError(t *testing.T) {
	err := NewAppError("TEST_CODE", "Test message", http.StatusBadRequest, nil)

	if err.Error() != "Test message" {
		t.Errorf("Expected 'Test message', got '%s'", err.Error())
	}
	if err.Code != "TEST_CODE" {
		t.Errorf("Expected 'TEST_CODE', got '%s'", err.Code)
	}
	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", err.StatusCode)
	}
}

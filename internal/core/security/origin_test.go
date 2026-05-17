package security

import (
	"net/http/httptest"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	t.Parallel()

	allowedOrigins := []string{
		"https://app.example.com",
		"http://localhost:3000",
	}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{
			name:   "accepts allowed origin",
			origin: "https://app.example.com",
			want:   true,
		},
		{
			name:   "rejects disallowed origin",
			origin: "https://evil.example.com",
			want:   false,
		},
		{
			name:   "accepts empty origin for non-browser clients",
			origin: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			if got := CheckOrigin(req, allowedOrigins); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

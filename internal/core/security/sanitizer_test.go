package security

import (
	"testing"
)

func TestSanitizeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"  hello world  \n", "hello world"},
		{"\t  trimmed value \r\n", "trimmed value"},
		{"   \n\t  ", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if result := SanitizeString(tt.input); result != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, result)
		}
	}
}

func TestSanitizeSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normalizes whitespace and case", "  Hello World!  ", "helloworld"},
		{"keeps allowed separators", "soccer-123", "soccer-123"},
		{"strips malformed html-like characters", "<script>hello</script>", "scripthelloscript"},
		{"keeps underscores and dashes", "Valid_Slug-1", "valid_slug-1"},
		{"returns empty for symbol-only input", " !!! ", ""},
		{"returns empty for whitespace-only input", "   \n\t ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSlug(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q for %q, got %q", tt.expected, tt.input, result)
			}
		})
	}
}

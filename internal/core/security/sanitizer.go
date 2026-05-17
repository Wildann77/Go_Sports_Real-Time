package security

import (
	"strings"
	"unicode"
)

func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}

func SanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

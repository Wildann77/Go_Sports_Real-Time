package security

import "net/http"

func CheckOrigin(r *http.Request, allowedOrigins []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Allow non-browser clients, or change to false if strict
	}
	for _, o := range allowedOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

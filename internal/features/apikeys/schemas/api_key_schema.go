package schemas

import (
	"strings"
	"time"
)

const (
	ScopeMatchesWrite    = "matches:write"
	ScopeCommentaryWrite = "commentary:write"
)

var allowedScopes = map[string]struct{}{
	ScopeMatchesWrite:    {},
	ScopeCommentaryWrite: {},
}

type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required,max=100,non_empty_trimmed"`
	Scopes    []string   `json:"scopes" binding:"required,min=1,dive,required"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type APIKeyResponse struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"keyPrefix"`
	KeyLastFour string     `json:"keyLastFour"`
	Scopes      []string   `json:"scopes"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	RevokedAt   *time.Time `json:"revokedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type CreateAPIKeyResponse struct {
	APIKey string          `json:"apiKey"`
	Key    *APIKeyResponse `json:"key"`
}

type AuthenticatedAPIKey struct {
	KeyID      int64
	UserID     int64
	Name       string
	Scopes     []string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

func AllowedScopes() map[string]struct{} {
	copyMap := make(map[string]struct{}, len(allowedScopes))
	for scope := range allowedScopes {
		copyMap[scope] = struct{}{}
	}
	return copyMap
}

func IsAPIKeyToken(rawToken string) bool {
	token := strings.TrimSpace(rawToken)
	return strings.HasPrefix(token, "sk_live_") || strings.HasPrefix(token, "sk_test_")
}

package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	apiKeySchemas "sports-dashboard/internal/features/apikeys/schemas"
	authSchemas "sports-dashboard/internal/features/auth/schemas"
)

const authUserContextKey = "auth_user"
const apiKeyContextKey = "api_key"

type AccessTokenVerifier interface {
	VerifyAccessToken(ctx context.Context, rawToken string) (*authSchemas.AuthenticatedUser, error)
}

type APIKeyVerifier interface {
	VerifyAPIKey(ctx context.Context, rawToken, requiredScope string) (*apiKeySchemas.AuthenticatedAPIKey, error)
}

func AuthRequired(verifier AccessTokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}

		authUser, err := verifier.VerifyAccessToken(c.Request.Context(), token)
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}

		c.Set(authUserContextKey, authUser)
		c.Next()
	}
}

func RequireJWTOrAPIKey(accessVerifier AccessTokenVerifier, apiKeyVerifier APIKeyVerifier, requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.Error(err)
			c.Abort()
			return
		}

		if apiKeySchemas.IsAPIKeyToken(token) {
			authAPIKey, verifyErr := apiKeyVerifier.VerifyAPIKey(c.Request.Context(), token, requiredScope)
			if verifyErr != nil {
				c.Error(verifyErr)
				c.Abort()
				return
			}

			c.Set(apiKeyContextKey, authAPIKey)
			c.Next()
			return
		}

		authUser, verifyErr := accessVerifier.VerifyAccessToken(c.Request.Context(), token)
		if verifyErr != nil {
			c.Error(verifyErr)
			c.Abort()
			return
		}

		c.Set(authUserContextKey, authUser)
		c.Next()
	}
}

func CurrentAuthenticatedUser(c *gin.Context) (*authSchemas.AuthenticatedUser, bool) {
	value, ok := c.Get(authUserContextKey)
	if !ok {
		return nil, false
	}

	authUser, ok := value.(*authSchemas.AuthenticatedUser)
	return authUser, ok
}

func CurrentAuthenticatedAPIKey(c *gin.Context) (*apiKeySchemas.AuthenticatedAPIKey, bool) {
	value, ok := c.Get(apiKeyContextKey)
	if !ok {
		return nil, false
	}

	authAPIKey, ok := value.(*apiKeySchemas.AuthenticatedAPIKey)
	return authAPIKey, ok
}

func extractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", authSchemas.ErrMissingAuthorizationHeader
	}

	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return "", authSchemas.ErrInvalidAuthorizationHeader
	}

	return strings.TrimSpace(token), nil
}

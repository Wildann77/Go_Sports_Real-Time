package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token expired")
	ErrTokenTypeMismatch = errors.New("token type mismatch")
	ErrTokenConfig       = errors.New("token configuration error")
)

type Claims struct {
	Subject      string `json:"sub"`
	UserID       int64  `json:"uid"`
	TokenVersion int    `json:"tv"`
	Type         string `json:"typ"`
	JTI          string `json:"jti,omitempty"`
	FamilyID     string `json:"fid,omitempty"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
}

type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type TokenOptions struct {
	Secret       string
	TTL          time.Duration
	TokenType    string
	UserID       int64
	TokenVersion int
	JTI          string
	FamilyID     string
	Now          time.Time
}

func GenerateToken(opts TokenOptions) (string, *Claims, error) {
	if strings.TrimSpace(opts.Secret) == "" || opts.TTL <= 0 || opts.UserID <= 0 || opts.TokenType == "" {
		return "", nil, ErrTokenConfig
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	headerBytes, err := json.Marshal(tokenHeader{
		Alg: "HS256",
		Typ: "JWT",
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal token header: %w", err)
	}

	claims := &Claims{
		Subject:      fmt.Sprintf("%d", opts.UserID),
		UserID:       opts.UserID,
		TokenVersion: opts.TokenVersion,
		Type:         opts.TokenType,
		JTI:          opts.JTI,
		FamilyID:     opts.FamilyID,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(opts.TTL).Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", nil, fmt.Errorf("marshal token claims: %w", err)
	}

	unsignedToken := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := sign(unsignedToken, opts.Secret)

	return unsignedToken + "." + signature, claims, nil
}

func ParseAndVerifyToken(rawToken, secret, expectedType string, now time.Time) (*Claims, error) {
	return parseAndVerifyToken(rawToken, secret, expectedType, now, false)
}

func ParseAndVerifyTokenAllowExpired(rawToken, secret, expectedType string, now time.Time) (*Claims, error) {
	return parseAndVerifyToken(rawToken, secret, expectedType, now, true)
}

func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func CompareTokenHash(rawToken, storedHash string) bool {
	computedHash := HashToken(rawToken)
	if len(computedHash) != len(storedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedHash)) == 1
}

func parseAndVerifyToken(rawToken, secret, expectedType string, now time.Time, allowExpired bool) (*Claims, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrTokenConfig
	}

	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	unsignedToken := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(unsignedToken, secret)), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var header tokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}
	if header.Alg != "HS256" {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if claims.Type != expectedType {
		return nil, ErrTokenTypeMismatch
	}
	if claims.UserID <= 0 || claims.ExpiresAt <= 0 {
		return nil, ErrInvalidToken
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !allowExpired && now.Unix() >= claims.ExpiresAt {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func sign(unsignedToken, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsignedToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

package utils

import (
	"errors"
	"testing"
	"time"
)

func TestGenerateAndParseTokenRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	token, claims, err := GenerateToken(TokenOptions{
		Secret:       "secret",
		TTL:          15 * time.Minute,
		TokenType:    TokenTypeRefresh,
		UserID:       42,
		TokenVersion: 3,
		JTI:          "jti-1",
		FamilyID:     "family-1",
		Now:          now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	parsedClaims, err := ParseAndVerifyToken(token, "secret", TokenTypeRefresh, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if parsedClaims.UserID != 42 || parsedClaims.TokenVersion != 3 {
		t.Fatalf("unexpected parsed claims %#v", parsedClaims)
	}
	if parsedClaims.JTI != claims.JTI || parsedClaims.FamilyID != claims.FamilyID {
		t.Fatalf("expected matching jti/family, got %#v and %#v", parsedClaims, claims)
	}
}

func TestParseAndVerifyTokenRejectsExpiredAndTypeMismatch(t *testing.T) {
	now := time.Now().UTC()
	token, _, err := GenerateToken(TokenOptions{
		Secret:    "secret",
		TTL:       time.Minute,
		TokenType: TokenTypeAccess,
		UserID:    7,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = ParseAndVerifyToken(token, "secret", TokenTypeAccess, now.Add(2*time.Minute))
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}

	_, err = ParseAndVerifyToken(token, "secret", TokenTypeRefresh, now.Add(30*time.Second))
	if !errors.Is(err, ErrTokenTypeMismatch) {
		t.Fatalf("expected ErrTokenTypeMismatch, got %v", err)
	}
}

func TestHashTokenComparison(t *testing.T) {
	token := "refresh-token-value"
	hash := HashToken(token)

	if hash == token {
		t.Fatal("expected token hash to differ from raw token")
	}
	if !CompareTokenHash(token, hash) {
		t.Fatal("expected token hash comparison to succeed")
	}
	if CompareTokenHash("different-token", hash) {
		t.Fatal("expected token hash comparison to fail for different token")
	}
}

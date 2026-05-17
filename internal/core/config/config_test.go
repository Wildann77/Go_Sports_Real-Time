package config

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	restore := unsetEnvKeys(
		t,
		"PORT",
		"DATABASE_URL",
		"APP_ENV",
		"ALLOWED_ORIGINS",
		"RATE_LIMIT_RPS",
		"RATE_LIMIT_BURST",
		"WS_MAX_PAYLOAD_BYTES",
		"DB_MAX_CONNS",
		"DB_MIN_CONNS",
		"DB_QUERY_TIMEOUT_SECONDS",
		"DB_TX_TIMEOUT_SECONDS",
	)
	defer restore()

	cfg := LoadConfig()

	if cfg.Port != "8000" {
		t.Fatalf("expected default port 8000, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty default database url, got %s", cfg.DatabaseURL)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("expected default app env development, got %s", cfg.AppEnv)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"http://localhost:3000"}) {
		t.Fatalf("unexpected default allowed origins: %#v", cfg.AllowedOrigins)
	}
	if cfg.RateLimitRPS != 5.0 {
		t.Fatalf("expected default rate limit rps 5.0, got %v", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 10 {
		t.Fatalf("expected default rate limit burst 10, got %d", cfg.RateLimitBurst)
	}
	if cfg.WsMaxPayloadBytes != 1048576 {
		t.Fatalf("expected default ws payload 1048576, got %d", cfg.WsMaxPayloadBytes)
	}
	if cfg.DbMaxConns != 10 {
		t.Fatalf("expected default db max conns 10, got %d", cfg.DbMaxConns)
	}
	if cfg.DbMinConns != 2 {
		t.Fatalf("expected default db min conns 2, got %d", cfg.DbMinConns)
	}
	if cfg.DbQueryTimeoutSeconds != 5 {
		t.Fatalf("expected default query timeout 5, got %d", cfg.DbQueryTimeoutSeconds)
	}
	if cfg.DbTxTimeoutSeconds != 10 {
		t.Fatalf("expected default tx timeout 10, got %d", cfg.DbTxTimeoutSeconds)
	}
}

func TestLoadConfigInvalidNumericFallbacks(t *testing.T) {
	restore := unsetEnvKeys(
		t,
		"RATE_LIMIT_RPS",
		"RATE_LIMIT_BURST",
		"WS_MAX_PAYLOAD_BYTES",
		"DB_MAX_CONNS",
		"DB_MIN_CONNS",
		"DB_QUERY_TIMEOUT_SECONDS",
		"DB_TX_TIMEOUT_SECONDS",
	)
	defer restore()

	t.Setenv("RATE_LIMIT_RPS", "bad-float")
	t.Setenv("RATE_LIMIT_BURST", "bad-int")
	t.Setenv("WS_MAX_PAYLOAD_BYTES", "bad-int")
	t.Setenv("DB_MAX_CONNS", "bad-int")
	t.Setenv("DB_MIN_CONNS", "bad-int")
	t.Setenv("DB_QUERY_TIMEOUT_SECONDS", "bad-int")
	t.Setenv("DB_TX_TIMEOUT_SECONDS", "bad-int")

	cfg := LoadConfig()

	if cfg.RateLimitRPS != 5.0 {
		t.Fatalf("expected fallback rate limit rps 5.0, got %v", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 10 {
		t.Fatalf("expected fallback rate limit burst 10, got %d", cfg.RateLimitBurst)
	}
	if cfg.WsMaxPayloadBytes != 1048576 {
		t.Fatalf("expected fallback ws payload 1048576, got %d", cfg.WsMaxPayloadBytes)
	}
	if cfg.DbMaxConns != 10 {
		t.Fatalf("expected fallback db max conns 10, got %d", cfg.DbMaxConns)
	}
	if cfg.DbMinConns != 2 {
		t.Fatalf("expected fallback db min conns 2, got %d", cfg.DbMinConns)
	}
	if cfg.DbQueryTimeoutSeconds != 5 {
		t.Fatalf("expected fallback query timeout 5, got %d", cfg.DbQueryTimeoutSeconds)
	}
	if cfg.DbTxTimeoutSeconds != 10 {
		t.Fatalf("expected fallback tx timeout 10, got %d", cfg.DbTxTimeoutSeconds)
	}
}

func TestConfigTimeoutConversions(t *testing.T) {
	cfg := &Config{
		DbQueryTimeoutSeconds: 7,
		DbTxTimeoutSeconds:    11,
	}

	if cfg.DBQueryTimeout() != 7*time.Second {
		t.Fatalf("expected query timeout 7s, got %s", cfg.DBQueryTimeout())
	}
	if cfg.DBTxTimeout() != 11*time.Second {
		t.Fatalf("expected tx timeout 11s, got %s", cfg.DBTxTimeout())
	}
}

func unsetEnvKeys(t *testing.T, keys ...string) func() {
	t.Helper()

	previous := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, exists := os.LookupEnv(key); exists {
			valueCopy := value
			previous[key] = &valueCopy
		} else {
			previous[key] = nil
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset env %s: %v", key, err)
		}
	}

	return func() {
		for _, key := range keys {
			if previous[key] == nil {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *previous[key])
		}
	}
}

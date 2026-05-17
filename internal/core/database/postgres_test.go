package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"sports-dashboard/internal/core/config"
)

func TestNewPostgresDBFailsFastOnBootstrapError(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL:           "://bad-dsn",
		AppEnv:                "development",
		DbMaxConns:            3,
		DbMinConns:            1,
		DbQueryTimeoutSeconds: 1,
	}

	db, err := NewPostgresDB(cfg)
	if err == nil {
		t.Fatal("expected bootstrap error, got nil")
	}
	if db != nil {
		t.Fatal("expected nil db on bootstrap failure")
	}
}

func TestNewPostgresDBFailsFastOnPingFailure(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL:           "postgres://user:password@127.0.0.1:65432/sports_db?sslmode=disable",
		AppEnv:                "development",
		DbMaxConns:            4,
		DbMinConns:            2,
		DbQueryTimeoutSeconds: 1,
	}

	db, err := NewPostgresDB(cfg)
	if err == nil {
		if db != nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr == nil {
				_ = sqlDB.Close()
			}
		}
		t.Fatal("expected ping failure, got nil")
	}
	if db != nil {
		t.Fatal("expected nil db on ping failure")
	}
}

func TestNewPostgresDBAppliesPoolConfigWhenDatabaseAvailable(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL:           integrationDatabaseURL(),
		AppEnv:                "development",
		DbMaxConns:            6,
		DbMinConns:            3,
		DbQueryTimeoutSeconds: 2,
	}

	db, err := NewPostgresDB(cfg)
	if err != nil {
		if looksLikeConnError(err) {
			t.Skipf("database not available for integration bootstrap test: %v", err)
		}
		t.Fatalf("expected db bootstrap success, got %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expected sql db handle, got %v", err)
	}
	defer sqlDB.Close()

	if stats := sqlDB.Stats(); stats.MaxOpenConnections != cfg.DbMaxConns {
		t.Fatalf("expected max open conns %d, got %d", cfg.DbMaxConns, stats.MaxOpenConnections)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.DBQueryTimeout())
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		t.Fatalf("expected ping success, got %v", err)
	}
}

func TestTimeoutPolicyNilAndPositive(t *testing.T) {
	ctx := context.Background()

	nilPolicyCtx, nilPolicyCancel := (*TimeoutPolicy)(nil).WithQueryTimeout(ctx)
	defer nilPolicyCancel()
	if nilPolicyCtx != ctx {
		t.Fatal("expected nil query policy to return original context")
	}

	nilTxCtx, nilTxCancel := (*TimeoutPolicy)(nil).WithTransactionTimeout(ctx)
	defer nilTxCancel()
	if nilTxCtx != ctx {
		t.Fatal("expected nil tx policy to return original context")
	}

	policy := &TimeoutPolicy{
		queryTimeout: 25 * time.Millisecond,
		txTimeout:    40 * time.Millisecond,
	}

	queryCtx, queryCancel := policy.WithQueryTimeout(ctx)
	defer queryCancel()
	queryDeadline, queryOK := queryCtx.Deadline()
	if !queryOK {
		t.Fatal("expected query timeout deadline")
	}
	if remaining := time.Until(queryDeadline); remaining <= 0 || remaining > 100*time.Millisecond {
		t.Fatalf("unexpected query deadline remaining: %s", remaining)
	}

	txCtx, txCancel := policy.WithTransactionTimeout(ctx)
	defer txCancel()
	txDeadline, txOK := txCtx.Deadline()
	if !txOK {
		t.Fatal("expected tx timeout deadline")
	}
	if remaining := time.Until(txDeadline); remaining <= 0 || remaining > 120*time.Millisecond {
		t.Fatalf("unexpected tx deadline remaining: %s", remaining)
	}
}

func integrationDatabaseURL() string {
	return "postgres://user:password@127.0.0.1:5432/sports_db?sslmode=disable"
}

func looksLikeConnError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "timeout")
}

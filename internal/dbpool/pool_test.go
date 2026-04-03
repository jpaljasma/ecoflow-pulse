package dbpool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConfigurePGXAppliesEnvOverrides(t *testing.T) {
	t.Setenv("DB_POOL_MAX_OPEN_CONNS", "7")
	t.Setenv("DB_POOL_MAX_CONN_IDLE_TIME", "45s")
	t.Setenv("DB_POOL_MAX_CONN_LIFETIME", "15m")
	t.Setenv("DB_POOL_MAX_CONN_LIFETIME_JITTER", "90s")
	t.Setenv("DB_POOL_HEALTH_CHECK_PERIOD", "12s")

	cfg, err := pgxpool.ParseConfig("postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	ConfigurePGX(cfg)

	if got := cfg.MaxConns; got != 7 {
		t.Fatalf("MaxConns mismatch: got=%d want=7", got)
	}
	if got := cfg.MinConns; got != 0 {
		t.Fatalf("MinConns mismatch: got=%d want=0", got)
	}
	if got := cfg.MaxConnIdleTime; got != 45*time.Second {
		t.Fatalf("MaxConnIdleTime mismatch: got=%s want=45s", got)
	}
	if got := cfg.MaxConnLifetime; got != 15*time.Minute {
		t.Fatalf("MaxConnLifetime mismatch: got=%s want=15m", got)
	}
	if got := cfg.MaxConnLifetimeJitter; got != 90*time.Second {
		t.Fatalf("MaxConnLifetimeJitter mismatch: got=%s want=90s", got)
	}
	if got := cfg.HealthCheckPeriod; got != 12*time.Second {
		t.Fatalf("HealthCheckPeriod mismatch: got=%s want=12s", got)
	}
}

func TestRetryReadRetriesTransientPressureErrors(t *testing.T) {
	t.Setenv("DB_READ_RETRY_MAX_ATTEMPTS", "3")
	t.Setenv("DB_READ_RETRY_INITIAL_BACKOFF", "1ms")
	t.Setenv("DB_READ_RETRY_MAX_BACKOFF", "2ms")
	t.Setenv("DB_READ_RETRY_JITTER_FACTOR", "0")

	attempts := 0
	got, err := RetryRead(context.Background(), func(context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("FATAL: sorry, too many clients already (SQLSTATE 53300)")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("RetryRead failed: %v", err)
	}
	if got != "ok" {
		t.Fatalf("value mismatch: got=%q want=ok", got)
	}
	if attempts != 3 {
		t.Fatalf("attempt count mismatch: got=%d want=3", attempts)
	}
}

func TestIsTransientReadErrorRecognizesPgErrorCode(t *testing.T) {
	err := &pgconn.PgError{Code: pgTooManyClientsCode}
	if !IsTransientReadError(err) {
		t.Fatal("expected pg too-many-clients error to be retryable")
	}
}

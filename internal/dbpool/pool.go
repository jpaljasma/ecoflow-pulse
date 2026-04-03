package dbpool

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	// Keep service-level pools intentionally small because the local/dev cluster
	// fans out many replicas, each with its own store/reader pool.
	defaultMaxOpenConns            = 2
	defaultMaxIdleConns            = 1
	defaultConnMaxIdleTime         = 2 * time.Minute
	defaultConnMaxLifetime         = 30 * time.Minute
	defaultConnMaxLifetimeJitter   = 2 * time.Minute
	defaultPGXHealthCheckPeriod    = 30 * time.Second
	defaultReadRetryMaxAttempts    = 3
	defaultReadRetryInitialBackoff = 50 * time.Millisecond
	defaultReadRetryMaxBackoff     = 250 * time.Millisecond
	defaultReadRetryJitterFactor   = 0.2

	pgTooManyClientsCode = "53300"
)

func ConfigureSQL(db *sql.DB) {
	if db == nil {
		return
	}
	cfg := loadPoolConfig()
	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.maxConnIdleTime)
	db.SetConnMaxLifetime(cfg.maxConnLifetime)
}

func ConfigurePGX(cfg *pgxpool.Config) {
	if cfg == nil {
		return
	}
	poolCfg := loadPoolConfig()
	cfg.MaxConns = boundedInt32(poolCfg.maxOpenConns)
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = poolCfg.maxConnIdleTime
	cfg.MaxConnLifetime = poolCfg.maxConnLifetime
	cfg.MaxConnLifetimeJitter = poolCfg.maxConnLifetimeJitter
	cfg.HealthCheckPeriod = poolCfg.healthCheckPeriod
}

func RetryRead[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	opts := loadRetryConfig()
	backoff := opts.initialBackoff
	for attempt := 1; attempt <= opts.maxAttempts; attempt++ {
		value, err := fn(ctx)
		if err == nil {
			return value, nil
		}
		if attempt == opts.maxAttempts || !IsTransientReadError(err) || ctx.Err() != nil {
			return zero, err
		}
		if sleepErr := sleepWithContext(ctx, jitterDelay(backoff, opts.jitterFactor)); sleepErr != nil {
			return zero, err
		}
		backoff *= 2
		if backoff > opts.maxBackoff {
			backoff = opts.maxBackoff
		}
	}
	return zero, context.Cause(ctx)
}

func IsTransientReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		return pgErr.Code == pgTooManyClientsCode
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlstate 53300") ||
		strings.Contains(msg, "too many clients already") ||
		strings.Contains(msg, "remaining connection slots are reserved")
}

type poolConfig struct {
	maxOpenConns          int
	maxIdleConns          int
	maxConnIdleTime       time.Duration
	maxConnLifetime       time.Duration
	maxConnLifetimeJitter time.Duration
	healthCheckPeriod     time.Duration
}

func loadPoolConfig() poolConfig {
	return poolConfig{
		maxOpenConns:          runtimecfg.IntPositive("DB_POOL_MAX_OPEN_CONNS", defaultMaxOpenConns),
		maxIdleConns:          runtimecfg.IntMin("DB_POOL_MAX_IDLE_CONNS", defaultMaxIdleConns, 0),
		maxConnIdleTime:       runtimecfg.DurationNonNegative("DB_POOL_MAX_CONN_IDLE_TIME", defaultConnMaxIdleTime),
		maxConnLifetime:       runtimecfg.DurationPositive("DB_POOL_MAX_CONN_LIFETIME", defaultConnMaxLifetime),
		maxConnLifetimeJitter: runtimecfg.DurationNonNegative("DB_POOL_MAX_CONN_LIFETIME_JITTER", defaultConnMaxLifetimeJitter),
		healthCheckPeriod:     runtimecfg.DurationPositive("DB_POOL_HEALTH_CHECK_PERIOD", defaultPGXHealthCheckPeriod),
	}
}

type retryConfig struct {
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	jitterFactor   float64
}

func loadRetryConfig() retryConfig {
	initialBackoff := runtimecfg.DurationPositive("DB_READ_RETRY_INITIAL_BACKOFF", defaultReadRetryInitialBackoff)
	maxBackoff := runtimecfg.DurationPositive("DB_READ_RETRY_MAX_BACKOFF", defaultReadRetryMaxBackoff)
	if maxBackoff < initialBackoff {
		maxBackoff = initialBackoff
	}
	return retryConfig{
		maxAttempts:    runtimecfg.IntPositive("DB_READ_RETRY_MAX_ATTEMPTS", defaultReadRetryMaxAttempts),
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		jitterFactor:   runtimecfg.Float64NonNegative("DB_READ_RETRY_JITTER_FACTOR", defaultReadRetryJitterFactor),
	}
}

func boundedInt32(value int) int32 {
	switch {
	case value <= 0:
		return 0
	case value > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(value)
	}
}

func jitterDelay(backoff time.Duration, jitterFactor float64) time.Duration {
	if backoff <= 0 || jitterFactor <= 0 {
		return backoff
	}
	jitter := float64(backoff) * jitterFactor
	if jitter <= 0 {
		return backoff
	}
	minDelay := float64(backoff) - jitter
	maxDelay := float64(backoff) + jitter
	return time.Duration(minDelay + rand.Float64()*(maxDelay-minDelay))
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

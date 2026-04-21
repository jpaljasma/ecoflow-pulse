package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
)

type RecurringJob struct {
	JobKey      string
	JobType     string
	Interval    time.Duration
	PayloadJSON []byte
	Enabled     bool
	NextRunAt   time.Time
	LastRunAt   *time.Time
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("scheduler postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply scheduler postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open scheduler postgres: %w", err)
	}
	dbpool.ConfigureSQL(db)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping scheduler postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) EnsureJob(ctx context.Context, job RecurringJob, now time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("scheduler store is not configured")
	}
	if strings.TrimSpace(job.JobKey) == "" {
		return errors.New("scheduler job key is required")
	}
	if strings.TrimSpace(job.JobType) == "" {
		return errors.New("scheduler job type is required")
	}
	if job.Interval <= 0 {
		return errors.New("scheduler job interval must be positive")
	}
	payload := normalizeJSON(job.PayloadJSON)
	if job.NextRunAt.IsZero() {
		job.NextRunAt = now.UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scheduled_jobs (
	job_key,
	job_type,
	interval_seconds,
	payload_json,
	enabled,
	next_run_at,
	last_run_at,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
ON CONFLICT (job_key)
DO UPDATE SET
	job_type = EXCLUDED.job_type,
	interval_seconds = EXCLUDED.interval_seconds,
	payload_json = EXCLUDED.payload_json,
	enabled = EXCLUDED.enabled,
	updated_at = EXCLUDED.updated_at;
`,
		job.JobKey,
		job.JobType,
		int(job.Interval.Seconds()),
		payload,
		job.Enabled,
		job.NextRunAt.UTC(),
		nullTime(job.LastRunAt),
		now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("ensure scheduler job %q: %w", job.JobKey, err)
	}
	return nil
}

func (s *PostgresStore) ClaimDueJobs(ctx context.Context, now time.Time, limit int) ([]RecurringJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("scheduler store is not configured")
	}
	if limit <= 0 {
		limit = 8
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin scheduler claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
WITH due AS (
	SELECT job_key
	FROM scheduled_jobs
	WHERE enabled = TRUE
	  AND next_run_at <= $1
	ORDER BY next_run_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
UPDATE scheduled_jobs j
SET last_run_at = $1,
	next_run_at = $1 + (j.interval_seconds * INTERVAL '1 second'),
	updated_at = $1
FROM due
WHERE j.job_key = due.job_key
RETURNING j.job_key, j.job_type, j.interval_seconds, j.payload_json, j.enabled, j.next_run_at, j.last_run_at;
`, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("claim due scheduler jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]RecurringJob, 0, limit)
	for rows.Next() {
		var (
			job         RecurringJob
			seconds     int
			payloadJSON []byte
			lastRunAt   sql.NullTime
		)
		if err := rows.Scan(&job.JobKey, &job.JobType, &seconds, &payloadJSON, &job.Enabled, &job.NextRunAt, &lastRunAt); err != nil {
			return nil, fmt.Errorf("scan claimed scheduler job: %w", err)
		}
		job.Interval = time.Duration(seconds) * time.Second
		job.PayloadJSON = normalizeJSON(payloadJSON)
		if lastRunAt.Valid {
			value := lastRunAt.Time.UTC()
			job.LastRunAt = &value
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed scheduler jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit scheduler claim tx: %w", err)
	}
	return out, nil
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func normalizeJSON(value []byte) []byte {
	if len(value) == 0 {
		return []byte(`{}`)
	}
	var normalized any
	if err := json.Unmarshal(value, &normalized); err != nil {
		return []byte(`{}`)
	}
	out, err := json.Marshal(normalized)
	if err != nil {
		return []byte(`{}`)
	}
	return out
}

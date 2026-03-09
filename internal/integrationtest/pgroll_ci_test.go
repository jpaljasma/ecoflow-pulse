package integrationtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPgrollPlansCycleAndRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skip pgroll CI integration test in short mode")
	}

	pgrollBin := strings.TrimSpace(os.Getenv("PGROLL_BIN"))
	if pgrollBin == "" {
		pgrollBin = "pgroll"
	}
	if _, err := exec.LookPath(pgrollBin); err != nil {
		if os.Getenv("PGROLL_REQUIRED") == "1" {
			t.Fatalf("pgroll is required but not found: %v", err)
		}
		t.Skip("pgroll not installed; skipping pgroll integration validation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	root, err := resolveRepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	stack, postgresURL, err := startPgrollStack(DefaultStackOptions(), root, ctx)
	if err != nil {
		t.Fatalf("build pgroll postgres url: %v", err)
	}
	t.Cleanup(func() {
		_ = stack.Terminate(context.Background())
	})

	smokePlan := filepath.Join(root, "internal/integrationtest/testdata/pgroll/smoke_create_customers.json")
	if err := runPgrollCycle(ctx, pgrollBin, postgresURL, smokePlan); err != nil {
		t.Fatalf("pgroll smoke cycle failed: %v", err)
	}

	repoPlans, err := discoverRepoPgrollPlans(root)
	if err != nil {
		t.Fatalf("discover repo pgroll plans: %v", err)
	}
	if len(repoPlans) == 0 {
		t.Log("no repo pgroll plan files found; smoke cycle only")
		return
	}

	chainStack, chainURL, err := startPgrollStack(DefaultStackOptions(), root, ctx)
	if err != nil {
		t.Fatalf("build chained pgroll postgres url: %v", err)
	}
	t.Cleanup(func() {
		_ = chainStack.Terminate(context.Background())
	})
	if err := runPgrollInit(ctx, pgrollBin, chainURL); err != nil {
		t.Fatalf("pgroll init for repo plans failed: %v", err)
	}
	for _, plan := range repoPlans {
		if err := runPgrollCommand(ctx, pgrollBin, chainURL, "start", plan); err != nil {
			t.Fatalf("pgroll start %s failed: %v", plan, err)
		}
		if err := runPgrollCommand(ctx, pgrollBin, chainURL, "complete"); err != nil {
			t.Fatalf("pgroll complete %s failed: %v", plan, err)
		}
	}
}

func startPgrollStack(opts StackOptions, root string, ctx context.Context) (*Stack, string, error) {
	opts.MigrationsDir = filepath.Join(root, opts.MigrationsDir)
	stack, err := StartPostgresStack(ctx, opts)
	if err != nil {
		return nil, "", err
	}
	cfg, err := pgx.ParseConfig(stack.PostgresDSN)
	if err != nil {
		return nil, "", fmt.Errorf("parse postgres dsn: %w", err)
	}
	query := url.Values{}
	if sslmode := strings.TrimSpace(cfg.RuntimeParams["sslmode"]); sslmode != "" {
		query.Set("sslmode", sslmode)
	} else {
		query.Set("sslmode", "disable")
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" || host == "localhost" || host == "::1" || host == "[::1]" {
		host = "127.0.0.1"
	}
	return stack, (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", host, cfg.Port),
		Path:     cfg.Database,
		RawQuery: query.Encode(),
	}).String(), nil
}

func runPgrollCycle(ctx context.Context, pgrollBin string, postgresURL string, plan string) error {
	if err := runPgrollInit(ctx, pgrollBin, postgresURL); err != nil {
		return err
	}
	if err := runPgrollCommand(ctx, pgrollBin, postgresURL, "start", plan); err != nil {
		return err
	}
	if err := runPgrollCommand(ctx, pgrollBin, postgresURL, "rollback"); err != nil {
		if strings.Contains(err.Error(), "no active migration") {
			return nil
		}
		return err
	}
	return nil
}

func runPgrollInit(ctx context.Context, pgrollBin string, postgresURL string) error {
	cmd := exec.CommandContext(ctx, pgrollBin, "init", "--postgres-url", postgresURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pgroll init: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runPgrollCommand(ctx context.Context, pgrollBin string, postgresURL string, action string, extraArgs ...string) error {
	args := []string{"--postgres-url", postgresURL, action}
	args = append(args, extraArgs...)
	cmd := exec.CommandContext(ctx, pgrollBin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pgroll %s: %w\n%s", action, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func discoverRepoPgrollPlans(root string) ([]string, error) {
	dir := filepath.Join(root, "deploy/db/pgroll/plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read pgroll plans dir: %w", err)
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		switch {
		case strings.HasSuffix(name, ".json"), strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
			out = append(out, filepath.Join(dir, name))
		}
	}
	return out, nil
}

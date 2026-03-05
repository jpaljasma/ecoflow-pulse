package integrationtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Keep Timescale image aligned with the platform chart pin for parity.
	defaultTimescaleImage = "timescale/timescaledb:latest-pg18@sha256:42ffa71e12b69a7d8ff4fd04c177e4519f66d3c9d1a026e5c4adf296a8168240"
	defaultNATSImage      = "nats:2.11.0-alpine"
	defaultValkeyImage    = "valkey/valkey:8"
	defaultMinIOImage     = "minio/minio:latest"

	defaultPostgresUser = "pulse"
	defaultPostgresPass = "pulse-local-dev-password"
	defaultPostgresDB   = "pulse"

	defaultMinIOUser = "minio"
	defaultMinIOPass = "minio123"
	defaultMinIOZone = "us-east-1"
)

type StackOptions struct {
	PostgresImage string
	NATSImage     string
	ValkeyImage   string
	MinIOImage    string

	PostgresUser string
	PostgresPass string
	PostgresDB   string

	MinIOUser   string
	MinIOPass   string
	MinIORegion string

	MigrationsDir string
}

func DefaultStackOptions() StackOptions {
	return StackOptions{
		PostgresImage: defaultTimescaleImage,
		NATSImage:     defaultNATSImage,
		ValkeyImage:   defaultValkeyImage,
		MinIOImage:    defaultMinIOImage,
		PostgresUser:  defaultPostgresUser,
		PostgresPass:  defaultPostgresPass,
		PostgresDB:    defaultPostgresDB,
		MinIOUser:     defaultMinIOUser,
		MinIOPass:     defaultMinIOPass,
		MinIORegion:   defaultMinIOZone,
		MigrationsDir: "deploy/db/migrations",
	}
}

func (o StackOptions) normalized() StackOptions {
	out := o
	if strings.TrimSpace(out.PostgresImage) == "" {
		out.PostgresImage = defaultTimescaleImage
	}
	if strings.TrimSpace(out.NATSImage) == "" {
		out.NATSImage = defaultNATSImage
	}
	if strings.TrimSpace(out.ValkeyImage) == "" {
		out.ValkeyImage = defaultValkeyImage
	}
	if strings.TrimSpace(out.MinIOImage) == "" {
		out.MinIOImage = defaultMinIOImage
	}
	if strings.TrimSpace(out.PostgresUser) == "" {
		out.PostgresUser = defaultPostgresUser
	}
	if strings.TrimSpace(out.PostgresPass) == "" {
		out.PostgresPass = defaultPostgresPass
	}
	if strings.TrimSpace(out.PostgresDB) == "" {
		out.PostgresDB = defaultPostgresDB
	}
	if strings.TrimSpace(out.MinIOUser) == "" {
		out.MinIOUser = defaultMinIOUser
	}
	if strings.TrimSpace(out.MinIOPass) == "" {
		out.MinIOPass = defaultMinIOPass
	}
	if strings.TrimSpace(out.MinIORegion) == "" {
		out.MinIORegion = defaultMinIOZone
	}
	if strings.TrimSpace(out.MigrationsDir) == "" {
		out.MigrationsDir = "deploy/db/migrations"
	}
	return out
}

type Stack struct {
	PostgresDSN   string
	NATSURL       string
	ValkeyAddress string
	MinIOEndpoint string
	MinIOUser     string
	MinIOPass     string
	MinIORegion   string

	containers []tc.Container
}

func StartStack(ctx context.Context, opts StackOptions) (*Stack, error) {
	opts = opts.normalized()
	stack := &Stack{
		MinIOUser:   strings.TrimSpace(opts.MinIOUser),
		MinIOPass:   strings.TrimSpace(opts.MinIOPass),
		MinIORegion: strings.TrimSpace(opts.MinIORegion),
	}
	var err error
	defer func() {
		if err != nil {
			_ = stack.Terminate(context.Background())
		}
	}()

	stack.PostgresDSN, err = startTimescale(ctx, stack, opts)
	if err != nil {
		return nil, err
	}
	if err = waitForPostgres(ctx, stack.PostgresDSN); err != nil {
		return nil, err
	}
	if err = ApplyMigrations(ctx, stack.PostgresDSN, opts.MigrationsDir); err != nil {
		return nil, err
	}

	stack.NATSURL, err = startNATS(ctx, stack, opts)
	if err != nil {
		return nil, err
	}
	stack.ValkeyAddress, err = startValkey(ctx, stack, opts)
	if err != nil {
		return nil, err
	}
	stack.MinIOEndpoint, err = startMinIO(ctx, stack, opts)
	if err != nil {
		return nil, err
	}

	return stack, nil
}

func (s *Stack) Terminate(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var combined error
	for i := len(s.containers) - 1; i >= 0; i-- {
		if s.containers[i] == nil {
			continue
		}
		if err := s.containers[i].Terminate(ctx); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	s.containers = nil
	return combined
}

func startTimescale(ctx context.Context, stack *Stack, opts StackOptions) (string, error) {
	container, host, port, err := startContainer(ctx, tc.ContainerRequest{
		Image: opts.PostgresImage,
		Env: map[string]string{
			"POSTGRES_USER":     opts.PostgresUser,
			"POSTGRES_PASSWORD": opts.PostgresPass,
			"POSTGRES_DB":       opts.PostgresDB,
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithStartupTimeout(2 * time.Minute),
	})
	if err != nil {
		return "", fmt.Errorf("start timescaledb container: %w", err)
	}
	stack.containers = append(stack.containers, container)
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host,
		port,
		strings.TrimSpace(opts.PostgresUser),
		strings.TrimSpace(opts.PostgresPass),
		strings.TrimSpace(opts.PostgresDB),
	)
	return dsn, nil
}

func startNATS(ctx context.Context, stack *Stack, opts StackOptions) (string, error) {
	container, host, port, err := startContainer(ctx, tc.ContainerRequest{
		Image:        opts.NATSImage,
		Cmd:          []string{"-js", "-sd", "/data", "-m", "8222"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor: wait.ForLog("Server is ready").
			WithStartupTimeout(90 * time.Second),
	})
	if err != nil {
		return "", fmt.Errorf("start nats container: %w", err)
	}
	stack.containers = append(stack.containers, container)
	return fmt.Sprintf("nats://%s:%s", host, port), nil
}

func startValkey(ctx context.Context, stack *Stack, opts StackOptions) (string, error) {
	container, host, port, err := startContainer(ctx, tc.ContainerRequest{
		Image:        opts.ValkeyImage,
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForListeningPort("6379/tcp").
			WithStartupTimeout(90 * time.Second),
	})
	if err != nil {
		return "", fmt.Errorf("start valkey container: %w", err)
	}
	stack.containers = append(stack.containers, container)
	return fmt.Sprintf("%s:%s", host, port), nil
}

func startMinIO(ctx context.Context, stack *Stack, opts StackOptions) (string, error) {
	container, host, port, err := startContainer(ctx, tc.ContainerRequest{
		Image: opts.MinIOImage,
		Env: map[string]string{
			"MINIO_ROOT_USER":     opts.MinIOUser,
			"MINIO_ROOT_PASSWORD": opts.MinIOPass,
		},
		Cmd:          []string{"server", "/data", "--address=:9000"},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor: wait.ForListeningPort("9000/tcp").
			WithStartupTimeout(90 * time.Second),
	})
	if err != nil {
		return "", fmt.Errorf("start minio container: %w", err)
	}
	stack.containers = append(stack.containers, container)
	return fmt.Sprintf("%s:%s", host, port), nil
}

func startContainer(ctx context.Context, request tc.ContainerRequest) (tc.Container, string, string, error) {
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	})
	if err != nil {
		return nil, "", "", err
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", "", fmt.Errorf("resolve container host: %w", err)
	}
	if len(request.ExposedPorts) == 0 {
		_ = container.Terminate(ctx)
		return nil, "", "", errors.New("container request must expose at least one port")
	}
	port, err := container.MappedPort(ctx, nat.Port(request.ExposedPorts[0]))
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", "", fmt.Errorf("resolve container port: %w", err)
	}
	return container, host, port.Port(), nil
}

func waitForPostgres(ctx context.Context, dsn string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		db, err := sql.Open("pgx", strings.TrimSpace(dsn))
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			closeErr := db.Close()
			if closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("wait for postgres ready: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func ApplyMigrations(ctx context.Context, dsn string, migrationsDir string) error {
	root, err := resolveRepoRoot()
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(migrationsDir)
	if dir == "" {
		return errors.New("migrations directory is required")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if strings.HasSuffix(name, ".up.sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no up migrations found in %q", dir)
	}

	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return fmt.Errorf("open postgres for migrations: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()
	if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;"); err != nil {
		return fmt.Errorf("ensure timescaledb extension: %w", err)
	}
	for _, file := range files {
		body, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmt.Errorf("read migration %q: %w", file, readErr)
		}
		sqlBody := strings.TrimSpace(string(body))
		if sqlBody == "" {
			continue
		}
		if _, execErr := db.ExecContext(ctx, sqlBody); execErr != nil {
			return fmt.Errorf("apply migration %q: %w", file, execErr)
		}
	}
	return nil
}

func resolveRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	cur := wd
	for {
		candidate := filepath.Join(cur, "go.mod")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return cur, nil
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return "", fmt.Errorf("stat %q: %w", candidate, statErr)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", fmt.Errorf("repository root (go.mod) not found from %q", wd)
}

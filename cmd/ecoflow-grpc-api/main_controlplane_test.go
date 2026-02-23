package main

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

func TestNewControlPlaneStoreFromEnvFallback(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, cleanup, err := newControlPlaneStoreFromEnv(log)
	if err != nil {
		t.Fatalf("newControlPlaneStoreFromEnv returned error: %v", err)
	}
	defer cleanup()

	if _, ok := store.(*controlplane.MemoryStore); !ok {
		t.Fatalf("expected memory store fallback, got %T", store)
	}
}

func TestNewControlPlaneStoreFromEnvInvalidDSN(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "://bad-dsn")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, cleanup, err := newControlPlaneStoreFromEnv(log)
	if err == nil {
		cleanup()
		t.Fatalf("expected error for invalid postgres dsn, got store=%T", store)
	}
}

func TestMainWhitespaceDSNFallsBackToMemoryStore(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "   \n\t  ")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, cleanup, err := newControlPlaneStoreFromEnv(log)
	if err != nil {
		t.Fatalf("newControlPlaneStoreFromEnv returned error: %v", err)
	}
	defer cleanup()
	if _, ok := store.(*controlplane.MemoryStore); !ok {
		t.Fatalf("expected memory store fallback, got %T", store)
	}
}

func TestMainStoreFallbackDoesNotDependOnFilesystem(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir temp dir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	store, cleanup, err := newControlPlaneStoreFromEnv(log)
	if err != nil {
		t.Fatalf("newControlPlaneStoreFromEnv returned error: %v", err)
	}
	defer cleanup()
	if _, ok := store.(*controlplane.MemoryStore); !ok {
		t.Fatalf("expected memory store fallback, got %T", store)
	}
}

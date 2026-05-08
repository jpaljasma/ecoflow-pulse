package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
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

func TestNewAuthorizerFromEnvNoop(t *testing.T) {
	t.Setenv("GRPC_AUTH_MODE", "noop")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := newAuthorizerFromEnv(context.Background(), log)
	if err != nil {
		t.Fatalf("newAuthorizerFromEnv returned error: %v", err)
	}
	if _, ok := a.(grpcmw.NoopAuthorizer); !ok {
		t.Fatalf("expected NoopAuthorizer, got %T", a)
	}
}

func TestNewAuthorizerFromEnvKeycloakMissingIssuer(t *testing.T) {
	t.Setenv("GRPC_AUTH_MODE", "keycloak")
	t.Setenv("KEYCLOAK_ISSUER_URL", "")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := newAuthorizerFromEnv(context.Background(), log)
	if err == nil {
		t.Fatalf("expected error when issuer is missing")
	}
}

func TestGRPCServiceModeFromEnvDefaultsTelemetry(t *testing.T) {
	t.Setenv("GRPC_SERVICE_MODE", "")

	mode, err := grpcServiceModeFromEnv()
	if err != nil {
		t.Fatalf("grpcServiceModeFromEnv returned error: %v", err)
	}
	if mode != grpcServiceModeTelemetry {
		t.Fatalf("expected telemetry mode by default, got %q", mode)
	}
}

func TestGRPCServiceModeFromEnvSupportsEnergy(t *testing.T) {
	t.Setenv("GRPC_SERVICE_MODE", "energy")

	mode, err := grpcServiceModeFromEnv()
	if err != nil {
		t.Fatalf("grpcServiceModeFromEnv returned error: %v", err)
	}
	if mode != grpcServiceModeEnergy {
		t.Fatalf("expected energy mode, got %q", mode)
	}
}

func TestGRPCServiceModeFromEnvRejectsUnknownMode(t *testing.T) {
	t.Setenv("GRPC_SERVICE_MODE", "invalid")

	_, err := grpcServiceModeFromEnv()
	if err == nil {
		t.Fatal("expected error for unsupported service mode")
	}
}

func TestRegisterControlPlaneDiscoverersIncludesAnkerSolix(t *testing.T) {
	t.Parallel()

	registry := provideradapter.NewRegistry()
	registerControlPlaneDiscoverers(
		registry,
		staticDiscoverer{},
		staticDiscoverer{},
		staticDiscoverer{},
		nil,
	)

	for _, provider := range []string{
		controlplane.ProviderEcoFlow,
		controlplane.ProviderPecron,
		controlplane.ProviderAnkerSolix,
	} {
		if !registry.Supports(provider) {
			t.Fatalf("expected registry to support %s", provider)
		}
		if _, ok := registry.Discoverer(provider); !ok {
			t.Fatalf("expected discoverer for %s", provider)
		}
	}
	if registry.Supports(controlplane.ProviderPulseMQTT) {
		t.Fatal("did not expect pulse mqtt without adapter")
	}
}

func TestNewInferenceReaderFromEnvFallback(t *testing.T) {
	t.Setenv("VALKEY_ADDRS", "")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	reader, cleanup, err := newInferenceReaderFromEnv(log)
	if err != nil {
		t.Fatalf("newInferenceReaderFromEnv returned error: %v", err)
	}
	defer cleanup()
	if reader != nil {
		t.Fatalf("expected nil reader fallback, got %T", reader)
	}
}

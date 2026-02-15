package ecoflow

import (
	"context"
	"os"
	"testing"
)

func TestEnvironmentCredentialsProvider_UsesEnvironmentScopedVars(t *testing.T) {
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "dev-ak")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "dev-sk")
	t.Setenv("ECOFLOW_ACCESS_KEY", "shared-ak")
	t.Setenv("ECOFLOW_SECRET_KEY", "shared-sk")

	provider := NewEnvironmentCredentialsProvider(EnvironmentDev)
	credentials, err := provider.Credentials(context.Background())
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}

	if credentials.AccessKey != "dev-ak" {
		t.Fatalf("access key mismatch: got %q", credentials.AccessKey)
	}
	if credentials.SecretKey != "dev-sk" {
		t.Fatalf("secret key mismatch: got %q", credentials.SecretKey)
	}
}

func TestEnvironmentCredentialsProvider_FallsBackToSharedVars(t *testing.T) {
	t.Setenv("ECOFLOW_ACCESS_KEY", "shared-ak")
	t.Setenv("ECOFLOW_SECRET_KEY", "shared-sk")

	provider := NewEnvironmentCredentialsProvider(EnvironmentProd)
	credentials, err := provider.Credentials(context.Background())
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}

	if credentials.AccessKey != "shared-ak" {
		t.Fatalf("access key mismatch: got %q", credentials.AccessKey)
	}
	if credentials.SecretKey != "shared-sk" {
		t.Fatalf("secret key mismatch: got %q", credentials.SecretKey)
	}
}

func TestEnvironmentCredentialsProvider_ReturnsErrorWhenUnset(t *testing.T) {
	unsetIfPresent(t, "ECOFLOW_STAGING_ACCESS_KEY")
	unsetIfPresent(t, "ECOFLOW_STAGING_SECRET_KEY")
	unsetIfPresent(t, "ECOFLOW_ACCESS_KEY")
	unsetIfPresent(t, "ECOFLOW_SECRET_KEY")

	provider := NewEnvironmentCredentialsProvider(EnvironmentStaging)
	if _, err := provider.Credentials(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func unsetIfPresent(t *testing.T, key string) {
	t.Helper()
	_, exists := os.LookupEnv(key)
	if exists {
		t.Setenv(key, "")
	}
}

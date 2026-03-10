package ingestlease

import (
	"testing"

	valkey "github.com/valkey-io/valkey-go"
)

func TestConfigureSentinelFromEnv(t *testing.T) {
	t.Setenv("VALKEY_SENTINEL_MASTER_SET", "myprimary")
	t.Setenv("VALKEY_SENTINEL_USERNAME", "sentinel-user")
	t.Setenv("VALKEY_SENTINEL_PASSWORD", "sentinel-pass")

	cfg := ValkeyClientConfig{Sentinel: valkey.SentinelOption{}}
	ConfigureSentinelFromEnv(&cfg)

	if cfg.Sentinel.MasterSet != "myprimary" {
		t.Fatalf("master set = %q, want myprimary", cfg.Sentinel.MasterSet)
	}
	if cfg.Sentinel.Username != "sentinel-user" {
		t.Fatalf("sentinel username = %q, want sentinel-user", cfg.Sentinel.Username)
	}
	if cfg.Sentinel.Password != "sentinel-pass" {
		t.Fatalf("sentinel password = %q, want sentinel-pass", cfg.Sentinel.Password)
	}
}

package ingestlease

import (
	"slices"
	"testing"
)

func TestConfigureClientSideCacheFromEnvDefaultsAndOverrides(t *testing.T) {
	t.Setenv("VALKEY_CACHE_CLIENT_SIDE_CACHE_ENABLED", "")
	t.Setenv("VALKEY_CACHE_SIZE_EACH_CONN", "")
	t.Setenv("VALKEY_CACHE_CLIENT_TRACKING_OPTIONS", "")

	var cfg ValkeyClientConfig
	ConfigureClientSideCacheFromEnv(&cfg)
	if !cfg.ClientSideCacheEnabled {
		t.Fatal("client-side cache should default on for shared cache clients")
	}
	if cfg.CacheSizeEachConn != 0 {
		t.Fatalf("cache size = %d, want 0", cfg.CacheSizeEachConn)
	}
	if got, want := cfg.ClientTrackingOptions, []string{"OPTIN"}; !slices.Equal(got, want) {
		t.Fatalf("tracking options = %#v, want %#v", got, want)
	}

	t.Setenv("VALKEY_CACHE_CLIENT_SIDE_CACHE_ENABLED", "false")
	t.Setenv("VALKEY_CACHE_SIZE_EACH_CONN", "4096")
	t.Setenv("VALKEY_CACHE_CLIENT_TRACKING_OPTIONS", "OPTIN,NOLOOP")
	ConfigureClientSideCacheFromEnv(&cfg)
	if cfg.ClientSideCacheEnabled {
		t.Fatal("client-side cache override should disable shared cache clients")
	}
	if cfg.CacheSizeEachConn != 4096 {
		t.Fatalf("cache size override = %d, want 4096", cfg.CacheSizeEachConn)
	}
	if got, want := cfg.ClientTrackingOptions, []string{"OPTIN", "NOLOOP"}; !slices.Equal(got, want) {
		t.Fatalf("tracking options override = %#v, want %#v", got, want)
	}
}

func TestConfigureClientSideCacheFromEnvNilSafe(t *testing.T) {
	ConfigureClientSideCacheFromEnv(nil)
}

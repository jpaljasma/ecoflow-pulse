package ingestlease

import "testing"

func TestBuildValkeyClientOptionDisablesClientSideCacheByDefault(t *testing.T) {
	opt, err := buildValkeyClientOption(DefaultValkeyClientConfig([]string{"127.0.0.1:6379"}))
	if err != nil {
		t.Fatalf("build option: %v", err)
	}
	if !opt.DisableCache {
		t.Fatal("default valkey client unexpectedly enables client-side caching")
	}
}

func TestBuildValkeyClientOptionEnablesClientSideCacheExplicitly(t *testing.T) {
	cfg := DefaultValkeyClientConfig([]string{"127.0.0.1:6379"})
	cfg.ClientSideCacheEnabled = true
	cfg.CacheSizeEachConn = 8 << 20
	cfg.ClientTrackingOptions = []string{"OPTIN"}

	opt, err := buildValkeyClientOption(cfg)
	if err != nil {
		t.Fatalf("build option: %v", err)
	}
	if opt.DisableCache {
		t.Fatal("cache client did not enable client-side caching")
	}
	if opt.CacheSizeEachConn != 8<<20 {
		t.Fatalf("cache size = %d", opt.CacheSizeEachConn)
	}
	if len(opt.ClientTrackingOptions) != 1 || opt.ClientTrackingOptions[0] != "OPTIN" {
		t.Fatalf("tracking options = %#v", opt.ClientTrackingOptions)
	}
}

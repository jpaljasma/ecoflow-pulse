package ingestlease

import "github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"

const defaultClientTrackingOptions = "OPTIN"

// ConfigureClientSideCacheFromEnv applies the shared cache-client knobs used by
// read-through cache clients. Lease/script clients should keep cache disabled.
func ConfigureClientSideCacheFromEnv(cfg *ValkeyClientConfig) {
	if cfg == nil {
		return
	}
	cfg.ClientSideCacheEnabled = runtimecfg.Bool("VALKEY_CACHE_CLIENT_SIDE_CACHE_ENABLED", true)
	cfg.CacheSizeEachConn = runtimecfg.IntMin("VALKEY_CACHE_SIZE_EACH_CONN", 0, 0)
	cfg.ClientTrackingOptions = runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("VALKEY_CACHE_CLIENT_TRACKING_OPTIONS", defaultClientTrackingOptions))
}

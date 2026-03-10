package ingestlease

import (
	"os"
	"strings"
)

func ConfigureSentinelFromEnv(cfg *ValkeyClientConfig) {
	if cfg == nil {
		return
	}
	masterSet := strings.TrimSpace(os.Getenv("VALKEY_SENTINEL_MASTER_SET"))
	if masterSet == "" {
		return
	}
	cfg.Sentinel.MasterSet = masterSet
	cfg.Sentinel.Username = strings.TrimSpace(os.Getenv("VALKEY_SENTINEL_USERNAME"))
	cfg.Sentinel.Password = os.Getenv("VALKEY_SENTINEL_PASSWORD")
}

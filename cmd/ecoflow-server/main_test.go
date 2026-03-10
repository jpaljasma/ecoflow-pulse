package main

import "testing"

func TestLoadServerConfigFromEnv(t *testing.T) {
	t.Setenv("ECOFLOW_SERVER_ADDR", "127.0.0.1:18080")
	t.Setenv("ECOFLOW_SERVER_COMPRESSION_MIN_BYTES", "2048")
	t.Setenv("ECOFLOW_SERVER_GZIP_LEVEL", "3")
	t.Setenv("ECOFLOW_SERVER_DEFLATE_LEVEL", "2")
	t.Setenv("ECOFLOW_SERVER_BROTLI_LEVEL", "5")
	t.Setenv("ECOFLOW_SERVER_ZSTD_LEVEL", "7")

	cfg := loadServerConfigFromEnv()
	if cfg.Address != "127.0.0.1:18080" {
		t.Fatalf("address=%q", cfg.Address)
	}
	if cfg.Compression.MinResponseBytes != 2048 || cfg.Compression.GzipLevel != 3 || cfg.Compression.DeflateLevel != 2 || cfg.Compression.BrotliLevel != 5 || cfg.Compression.ZstdLevel != 7 {
		t.Fatalf("compression config mismatch: %+v", cfg.Compression)
	}
}

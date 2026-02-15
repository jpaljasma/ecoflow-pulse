package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jpaljasma/ecoflow-api-playground/pkg/ecoflowserver"
)

func main() {
	cfg := ecoflowserver.DefaultConfig()
	if addr := os.Getenv("ECOFLOW_SERVER_ADDR"); addr != "" {
		cfg.Address = addr
	}
	if min := os.Getenv("ECOFLOW_SERVER_COMPRESSION_MIN_BYTES"); min != "" {
		if value, err := strconv.Atoi(min); err == nil && value >= 0 {
			cfg.Compression.MinResponseBytes = value
		}
	}
	if gzipLevel := os.Getenv("ECOFLOW_SERVER_GZIP_LEVEL"); gzipLevel != "" {
		if value, err := strconv.Atoi(gzipLevel); err == nil {
			cfg.Compression.GzipLevel = value
		}
	}
	if deflateLevel := os.Getenv("ECOFLOW_SERVER_DEFLATE_LEVEL"); deflateLevel != "" {
		if value, err := strconv.Atoi(deflateLevel); err == nil {
			cfg.Compression.DeflateLevel = value
		}
	}
	if brotliLevel := os.Getenv("ECOFLOW_SERVER_BROTLI_LEVEL"); brotliLevel != "" {
		if value, err := strconv.Atoi(brotliLevel); err == nil {
			cfg.Compression.BrotliLevel = value
		}
	}
	if zstdLevel := os.Getenv("ECOFLOW_SERVER_ZSTD_LEVEL"); zstdLevel != "" {
		if value, err := strconv.Atoi(zstdLevel); err == nil {
			cfg.Compression.ZstdLevel = value
		}
	}

	server, err := ecoflowserver.New(cfg)
	if err != nil {
		log.Fatalf("server initialization failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("ecoflow server listening on %s", cfg.Address)
	if err := server.ListenAndServe(ctx); err != nil && err != context.Canceled {
		log.Fatalf("server stopped with error: %v", err)
	}
}

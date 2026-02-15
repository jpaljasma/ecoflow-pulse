package ecoflowserver

import (
	"errors"
	"net/http"
	"time"
)

// Config controls server runtime behavior and compression policy.
type Config struct {
	Address string

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	Compression CompressionConfig
	Handler     http.Handler
}

// CompressionConfig configures request decoding and response compression.
type CompressionConfig struct {
	Enabled bool

	MinResponseBytes int
	GzipLevel        int
	DeflateLevel     int
	BrotliLevel      int
	ZstdLevel        int
}

// DefaultConfig returns throughput-oriented server defaults.
func DefaultConfig() Config {
	return Config{
		Address:           ":8080",
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 * 1024,
		Compression: CompressionConfig{
			Enabled:          true,
			MinResponseBytes: 1024,
			GzipLevel:        5,
			DeflateLevel:     5,
			BrotliLevel:      5,
			ZstdLevel:        3,
		},
	}
}

// Validate ensures the server configuration is internally consistent.
func (c Config) Validate() error {
	if c.Address == "" {
		return errors.New("address is required")
	}
	if c.ReadHeaderTimeout <= 0 {
		return errors.New("read header timeout must be > 0")
	}
	if c.ReadTimeout < 0 {
		return errors.New("read timeout must be >= 0")
	}
	if c.WriteTimeout < 0 {
		return errors.New("write timeout must be >= 0")
	}
	if c.IdleTimeout < 0 {
		return errors.New("idle timeout must be >= 0")
	}
	if c.MaxHeaderBytes <= 0 {
		return errors.New("max header bytes must be > 0")
	}
	if c.Compression.MinResponseBytes < 0 {
		return errors.New("compression min response bytes must be >= 0")
	}
	return nil
}

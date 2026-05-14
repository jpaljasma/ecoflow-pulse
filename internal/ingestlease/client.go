package ingestlease

import (
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	valkey "github.com/valkey-io/valkey-go"
)

const (
	defaultDialTimeout         = 3 * time.Second
	defaultTCPKeepAlive        = 15 * time.Second
	defaultConnWriteTimeout    = 6 * time.Second
	defaultClusterRefresh      = 5 * time.Second
	defaultMovedRedirections   = 8
	defaultReadBufferEachConn  = 1 << 20
	defaultWriteBufferEachConn = 1 << 20
	defaultPipelineMultiplex   = 1
	defaultBlockingPoolSize    = 128
	defaultBlockingPoolCleanup = 2 * time.Minute
	defaultRetryMinBackoff     = 20 * time.Millisecond
	defaultRetryMaxBackoff     = 500 * time.Millisecond
	defaultRetryJitterFraction = 0.20
)

// ValkeyClientConfig controls high-throughput resilient valkey-go client behavior.
type ValkeyClientConfig struct {
	InitAddresses []string
	Username      string
	Password      string
	TLSConfig     *tls.Config
	Sentinel      valkey.SentinelOption

	DialTimeout      time.Duration
	TCPKeepAlive     time.Duration
	ConnWriteTimeout time.Duration

	ShardsRefreshInterval time.Duration
	MaxMovedRedirects     int

	ReadBufferEachConn  int
	WriteBufferEachConn int
	PipelineMultiplex   int
	BlockingPoolSize    int
	BlockingPoolCleanup time.Duration

	ClientSideCacheEnabled bool
	CacheSizeEachConn      int
	ClientTrackingOptions  []string

	DisableRetry bool
	RetryDelay   valkey.RetryDelayFn
}

// DefaultValkeyClientConfig returns production-leaning defaults for a cluster-capable client.
func DefaultValkeyClientConfig(initAddresses []string) ValkeyClientConfig {
	return ValkeyClientConfig{
		InitAddresses:         append([]string(nil), initAddresses...),
		DialTimeout:           defaultDialTimeout,
		TCPKeepAlive:          defaultTCPKeepAlive,
		ConnWriteTimeout:      defaultConnWriteTimeout,
		ShardsRefreshInterval: defaultClusterRefresh,
		MaxMovedRedirects:     defaultMovedRedirections,
		ReadBufferEachConn:    defaultReadBufferEachConn,
		WriteBufferEachConn:   defaultWriteBufferEachConn,
		PipelineMultiplex:     defaultPipelineMultiplex,
		BlockingPoolSize:      defaultBlockingPoolSize,
		BlockingPoolCleanup:   defaultBlockingPoolCleanup,
		RetryDelay:            defaultClientRetryDelay(),
	}
}

// NewValkeyClient constructs a cluster-aware valkey-go client.
func NewValkeyClient(cfg ValkeyClientConfig) (valkey.Client, error) {
	opt, err := buildValkeyClientOption(cfg)
	if err != nil {
		return nil, err
	}
	return valkey.NewClient(opt)
}

func buildValkeyClientOption(cfg ValkeyClientConfig) (valkey.ClientOption, error) {
	if len(cfg.InitAddresses) == 0 {
		return valkey.ClientOption{}, fmt.Errorf("at least one valkey init address is required")
	}

	addrs := make([]string, 0, len(cfg.InitAddresses))
	for _, addr := range cfg.InitAddresses {
		if trimmed := strings.TrimSpace(addr); trimmed != "" {
			addrs = append(addrs, trimmed)
		}
	}
	if len(addrs) == 0 {
		return valkey.ClientOption{}, fmt.Errorf("at least one non-empty valkey init address is required")
	}

	opt := valkey.ClientOption{
		InitAddress:         addrs,
		Username:            strings.TrimSpace(cfg.Username),
		Password:            cfg.Password,
		TLSConfig:           cfg.TLSConfig,
		Sentinel:            cfg.Sentinel,
		Dialer:              net.Dialer{Timeout: chooseDuration(cfg.DialTimeout, defaultDialTimeout), KeepAlive: chooseDuration(cfg.TCPKeepAlive, defaultTCPKeepAlive)},
		ConnWriteTimeout:    chooseDuration(cfg.ConnWriteTimeout, defaultConnWriteTimeout),
		ReadBufferEachConn:  chooseInt(cfg.ReadBufferEachConn, defaultReadBufferEachConn),
		WriteBufferEachConn: chooseInt(cfg.WriteBufferEachConn, defaultWriteBufferEachConn),
		PipelineMultiplex:   chooseInt(cfg.PipelineMultiplex, defaultPipelineMultiplex),
		BlockingPoolSize:    chooseInt(cfg.BlockingPoolSize, defaultBlockingPoolSize),
		BlockingPoolCleanup: chooseDuration(cfg.BlockingPoolCleanup, defaultBlockingPoolCleanup),
		ShuffleInit:         true,
		DisableRetry:        cfg.DisableRetry,
		RetryDelay:          chooseRetryDelay(cfg.RetryDelay),
		DisableCache:        !cfg.ClientSideCacheEnabled,
		ClusterOption: valkey.ClusterOption{
			ShardsRefreshInterval: chooseDuration(cfg.ShardsRefreshInterval, defaultClusterRefresh),
			MaxMovedRedirections:  chooseInt(cfg.MaxMovedRedirects, defaultMovedRedirections),
		},
	}
	if cfg.ClientSideCacheEnabled {
		opt.CacheSizeEachConn = cfg.CacheSizeEachConn
		opt.ClientTrackingOptions = append([]string(nil), cfg.ClientTrackingOptions...)
	}
	return opt, nil
}

func defaultClientRetryDelay() valkey.RetryDelayFn {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var rngMu sync.Mutex
	return func(attempts int, _ valkey.Completed, _ error) time.Duration {
		if attempts < 1 {
			attempts = 1
		}
		base := float64(defaultRetryMinBackoff) * math.Pow(2, float64(attempts-1))
		if base > float64(defaultRetryMaxBackoff) {
			base = float64(defaultRetryMaxBackoff)
		}
		jitterSpan := base * defaultRetryJitterFraction
		rngMu.Lock()
		jitter := (rng.Float64()*2 - 1) * jitterSpan
		rngMu.Unlock()
		delay := time.Duration(base + jitter)
		if delay < defaultRetryMinBackoff {
			return defaultRetryMinBackoff
		}
		if delay > defaultRetryMaxBackoff {
			return defaultRetryMaxBackoff
		}
		return delay
	}
}

func chooseDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func chooseInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func chooseRetryDelay(delay valkey.RetryDelayFn) valkey.RetryDelayFn {
	if delay != nil {
		return delay
	}
	return defaultClientRetryDelay()
}

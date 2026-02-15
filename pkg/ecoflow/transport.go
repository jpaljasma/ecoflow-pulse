package ecoflow

import (
	"net"
	"net/http"
	"time"
)

// TransportTuning controls HTTP transport performance characteristics.
type TransportTuning struct {
	DialTimeout           time.Duration
	DialKeepAlive         time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	ExpectContinueTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
}

// DefaultTransportTuning returns a high-throughput client transport profile.
func DefaultTransportTuning() TransportTuning {
	return TransportTuning{
		DialTimeout:           3 * time.Second,
		DialKeepAlive:         30 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       120 * time.Second,
		MaxIdleConns:          10_000,
		MaxIdleConnsPerHost:   4_096,
		MaxConnsPerHost:       0,
	}
}

// NewTunedTransport builds an http.Transport from TransportTuning.
func NewTunedTransport(tuning TransportTuning) *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
		MaxIdleConns:          tuning.MaxIdleConns,
		MaxIdleConnsPerHost:   tuning.MaxIdleConnsPerHost,
		MaxConnsPerHost:       tuning.MaxConnsPerHost,
		IdleConnTimeout:       tuning.IdleConnTimeout,
		TLSHandshakeTimeout:   tuning.TLSHandshakeTimeout,
		ExpectContinueTimeout: tuning.ExpectContinueTimeout,
		ResponseHeaderTimeout: tuning.ResponseHeaderTimeout,
		DialContext: (&net.Dialer{
			Timeout:   tuning.DialTimeout,
			KeepAlive: tuning.DialKeepAlive,
		}).DialContext,
	}
}

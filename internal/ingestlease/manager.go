package ingestlease

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	valkey "github.com/valkey-io/valkey-go"
)

const (
	defaultLeaseTTL          = 45 * time.Second
	defaultHeartbeatInterval = 15 * time.Second
	defaultHeartbeatJitter   = 0.10
	defaultMaxAttempts       = 3
	defaultMinBackoff        = 20 * time.Millisecond
	defaultMaxBackoff        = 300 * time.Millisecond
	defaultDrainTimeout      = 5 * time.Second
)

// Config controls lease timings and retry behavior.
type Config struct {
	LeaseTTL          time.Duration
	HeartbeatInterval time.Duration
	HeartbeatJitter   float64
	RetryPolicy       RetryPolicy
}

// RetryPolicy defines retry behavior for lease script calls.
type RetryPolicy struct {
	MaxAttempts int
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
}

// CallOptions allows per-call retry overrides.
type CallOptions struct {
	MaxAttempts int
}

// Lease is the owned lease metadata returned by successful Acquire.
type Lease struct {
	Ref      LeaseRef
	Token    string
	WorkerID string
	Fence    int64
	TTL      time.Duration
	keys     LeaseKeys
}

// AcquireResult contains successful or denied ownership details.
type AcquireResult struct {
	Acquired      bool
	Lease         Lease
	ExistingToken string
	ExistingFence int64
}

// RenewResult indicates token-checked heartbeat renewal status.
type RenewResult struct {
	Renewed bool
	Fence   int64
	Reason  string
}

// ReleaseResult indicates token-checked release status.
type ReleaseResult struct {
	Released bool
	Reason   string
}

// HeartbeatOptions configures lease heartbeat lifecycle behavior.
type HeartbeatOptions struct {
	Interval         time.Duration
	Jitter           float64
	GracefulDrain    bool
	DrainTimeout     time.Duration
	CallOptions      CallOptions
	StateDuringRenew string
}

// Manager owns token-checked lease operations backed by Lua scripts.
type Manager struct {
	client valkey.Client
	cfg    Config
	nowFn  func() time.Time
	ttlMs  string

	retry RetryPolicy

	randMu sync.Mutex
	rng    *mathrand.Rand
}

// DefaultConfig returns ADR-0014 lease defaults.
func DefaultConfig() Config {
	return Config{
		LeaseTTL:          defaultLeaseTTL,
		HeartbeatInterval: defaultHeartbeatInterval,
		HeartbeatJitter:   defaultHeartbeatJitter,
		RetryPolicy: RetryPolicy{
			MaxAttempts: defaultMaxAttempts,
			MinBackoff:  defaultMinBackoff,
			MaxBackoff:  defaultMaxBackoff,
		},
	}
}

// NewManager builds a lease manager with sane defaults and validation.
func NewManager(client valkey.Client, cfg Config) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("valkey client is required")
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = defaultLeaseTTL
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	if cfg.HeartbeatJitter < 0 {
		cfg.HeartbeatJitter = defaultHeartbeatJitter
	}
	if cfg.HeartbeatJitter > 1 {
		return nil, fmt.Errorf("heartbeat_jitter must be <= 1")
	}
	if cfg.HeartbeatInterval >= cfg.LeaseTTL {
		return nil, fmt.Errorf("heartbeat interval must be smaller than lease ttl")
	}

	retry := cfg.RetryPolicy
	if retry.MaxAttempts <= 0 {
		retry.MaxAttempts = defaultMaxAttempts
	}
	if retry.MinBackoff <= 0 {
		retry.MinBackoff = defaultMinBackoff
	}
	if retry.MaxBackoff <= 0 {
		retry.MaxBackoff = defaultMaxBackoff
	}
	if retry.MinBackoff > retry.MaxBackoff {
		retry.MinBackoff, retry.MaxBackoff = retry.MaxBackoff, retry.MinBackoff
	}

	return &Manager{
		client: client,
		cfg:    cfg,
		nowFn:  time.Now,
		ttlMs:  strconv.FormatInt(cfg.LeaseTTL.Milliseconds(), 10),
		retry:  retry,
		rng:    mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Acquire attempts to own the lease for a provider-device.
func (m *Manager) Acquire(ctx context.Context, ref LeaseRef, workerID string, token string, opts CallOptions) (AcquireResult, error) {
	keys, err := KeysForRef(ref)
	if err != nil {
		return AcquireResult{}, err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = defaultWorkerID()
	}
	token = strings.TrimSpace(token)
	if token == "" {
		token, err = newToken()
		if err != nil {
			return AcquireResult{}, fmt.Errorf("generate lease token: %w", err)
		}
	}

	resp, err := m.execScript(ctx, leaseAcquireScript, []string{keys.Lease, keys.Session, keys.Fence}, []string{
		token,
		workerID,
		m.ttlMs,
		strconv.FormatInt(m.nowFn().UTC().UnixMilli(), 10),
		strings.TrimSpace(ref.Provider),
		strings.TrimSpace(ref.ProviderDeviceID),
	}, opts)
	if err != nil {
		return AcquireResult{}, err
	}
	values, err := resp.ToArray()
	if err != nil {
		return AcquireResult{}, fmt.Errorf("parse acquire response: %w", err)
	}
	if len(values) < 3 {
		return AcquireResult{}, fmt.Errorf("invalid acquire response tuple length=%d", len(values))
	}

	acquired, err := values[0].ToInt64()
	if err != nil {
		return AcquireResult{}, fmt.Errorf("parse acquire status: %w", err)
	}
	fence, err := messageToInt64(values[1])
	if err != nil {
		return AcquireResult{}, fmt.Errorf("parse acquire fence: %w", err)
	}
	currentToken, err := values[2].ToString()
	if err != nil {
		return AcquireResult{}, fmt.Errorf("parse acquire token: %w", err)
	}
	if acquired == 1 {
		normalizedRef := LeaseRef{
			Provider:         strings.TrimSpace(ref.Provider),
			ProviderDeviceID: strings.TrimSpace(ref.ProviderDeviceID),
		}
		return AcquireResult{
			Acquired: true,
			Lease: Lease{
				Ref:      normalizedRef,
				Token:    token,
				WorkerID: workerID,
				Fence:    fence,
				TTL:      m.cfg.LeaseTTL,
				keys:     keys,
			},
		}, nil
	}
	return AcquireResult{
		Acquired:      false,
		ExistingToken: currentToken,
		ExistingFence: fence,
	}, nil
}

// Renew extends the lease TTL if token ownership is still valid.
func (m *Manager) Renew(ctx context.Context, lease Lease, state string, opts CallOptions) (RenewResult, error) {
	keys, err := keysForLease(lease)
	if err != nil {
		return RenewResult{}, err
	}
	lease.Token = strings.TrimSpace(lease.Token)
	lease.WorkerID = strings.TrimSpace(lease.WorkerID)
	if lease.Token == "" {
		return RenewResult{}, fmt.Errorf("token is required")
	}
	if lease.WorkerID == "" {
		return RenewResult{}, fmt.Errorf("worker_id is required")
	}
	state = normalizeState(state)
	resp, err := m.execScript(ctx, leaseRenewScript, []string{keys.Lease, keys.Session}, []string{
		lease.Token,
		lease.WorkerID,
		m.ttlMs,
		strconv.FormatInt(m.nowFn().UTC().UnixMilli(), 10),
		state,
	}, opts)
	if err != nil {
		return RenewResult{}, err
	}
	values, err := resp.ToArray()
	if err != nil {
		return RenewResult{}, fmt.Errorf("parse renew response: %w", err)
	}
	if len(values) < 3 {
		return RenewResult{}, fmt.Errorf("invalid renew response tuple length=%d", len(values))
	}
	ok, err := values[0].ToInt64()
	if err != nil {
		return RenewResult{}, fmt.Errorf("parse renew status: %w", err)
	}
	fence, err := messageToInt64(values[1])
	if err != nil {
		return RenewResult{}, fmt.Errorf("parse renew fence: %w", err)
	}
	reason, err := values[2].ToString()
	if err != nil {
		return RenewResult{}, fmt.Errorf("parse renew reason: %w", err)
	}
	return RenewResult{
		Renewed: ok == 1,
		Fence:   fence,
		Reason:  reason,
	}, nil
}

// DrainAndRelease marks lease as draining then releases ownership.
func (m *Manager) DrainAndRelease(ctx context.Context, lease Lease, opts CallOptions) (ReleaseResult, error) {
	drainCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		drainCtx, cancel = context.WithTimeout(ctx, defaultDrainTimeout)
		defer cancel()
	}
	if _, err := m.Renew(drainCtx, lease, leaseStateDraining, opts); err != nil {
		return ReleaseResult{}, err
	}
	return m.Release(drainCtx, lease.Ref, lease.Token, opts)
}

// Release removes a lease if token ownership is valid.
func (m *Manager) Release(ctx context.Context, ref LeaseRef, token string, opts CallOptions) (ReleaseResult, error) {
	keys, err := KeysForRef(ref)
	if err != nil {
		return ReleaseResult{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ReleaseResult{}, fmt.Errorf("token is required")
	}
	resp, err := m.execScript(ctx, leaseReleaseScript, []string{keys.Lease, keys.Session}, []string{token}, opts)
	if err != nil {
		return ReleaseResult{}, err
	}
	values, err := resp.ToArray()
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("parse release response: %w", err)
	}
	if len(values) < 2 {
		return ReleaseResult{}, fmt.Errorf("invalid release response tuple length=%d", len(values))
	}
	ok, err := values[0].ToInt64()
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("parse release status: %w", err)
	}
	reason, err := values[1].ToString()
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("parse release reason: %w", err)
	}
	return ReleaseResult{
		Released: ok == 1,
		Reason:   reason,
	}, nil
}

// RunHeartbeat keeps a lease alive until ctx is done or ownership is lost.
// If GracefulDrain is enabled, it drains+releases lease on context cancellation.
func (m *Manager) RunHeartbeat(ctx context.Context, lease Lease, options HeartbeatOptions) error {
	interval := options.Interval
	if interval <= 0 {
		interval = m.cfg.HeartbeatInterval
	}
	jitter := options.Jitter
	if jitter < 0 {
		jitter = 0
	}
	if jitter == 0 {
		jitter = m.cfg.HeartbeatJitter
	}
	state := normalizeState(options.StateDuringRenew)

	timer := time.NewTimer(m.jitteredInterval(interval, jitter))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			drainTimeout := options.DrainTimeout
			if drainTimeout <= 0 {
				drainTimeout = defaultDrainTimeout
			}
			drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
			defer cancel()
			if options.GracefulDrain {
				_, err := m.DrainAndRelease(drainCtx, lease, options.CallOptions)
				return err
			}
			if strings.TrimSpace(lease.Token) == "" {
				return nil
			}
			released, err := m.Release(drainCtx, lease.Ref, lease.Token, options.CallOptions)
			if err != nil {
				return err
			}
			if !released.Released && released.Reason != "not_found" && released.Reason != "token_mismatch" {
				return fmt.Errorf("lease release rejected: %s", released.Reason)
			}
			return nil
		case <-timer.C:
			result, err := m.Renew(ctx, lease, state, options.CallOptions)
			if err != nil {
				return err
			}
			if !result.Renewed {
				// On transient backend churn the lease key can momentarily disappear.
				// Try one immediate self-heal reacquire with the same token before failing.
				if result.Reason == "missing" {
					reacquired, reacquireErr := m.Acquire(ctx, lease.Ref, lease.WorkerID, lease.Token, options.CallOptions)
					if reacquireErr != nil {
						return fmt.Errorf("lease reacquire after missing failed: %w", reacquireErr)
					}
					if reacquired.Acquired {
						lease.Fence = reacquired.Lease.Fence
						timer.Reset(m.jitteredInterval(interval, jitter))
						continue
					}
				}
				return NewLeaseRejectedError("renew", result.Reason)
			}
			lease.Fence = result.Fence
			timer.Reset(m.jitteredInterval(interval, jitter))
		}
	}
}

func (m *Manager) jitteredInterval(base time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return base
	}
	m.randMu.Lock()
	random := m.rng.Float64()
	m.randMu.Unlock()
	shift := ((random * 2) - 1) * jitter
	delay := float64(base) * (1 + shift)
	if delay < float64(time.Millisecond) {
		return time.Millisecond
	}
	return time.Duration(delay)
}

func (m *Manager) execScript(ctx context.Context, script *valkey.Lua, keys []string, args []string, opts CallOptions) (valkey.ValkeyResult, error) {
	maxAttempts := m.retry.MaxAttempts
	if opts.MaxAttempts > 0 {
		maxAttempts = opts.MaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var result valkey.ValkeyResult
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result = script.Exec(ctx, m.client, keys, args)
		if err := result.Error(); err != nil {
			lastErr = err
			if attempt == maxAttempts || !isRetryableErr(err) {
				return result, err
			}
			if sleepErr := sleepWithContext(ctx, m.retryDelay(attempt)); sleepErr != nil {
				return result, sleepErr
			}
			continue
		}
		return result, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown lease script error")
	}
	return result, lastErr
}

func (m *Manager) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := float64(m.retry.MinBackoff) * math.Pow(2, float64(attempt-1))
	if backoff > float64(m.retry.MaxBackoff) {
		backoff = float64(m.retry.MaxBackoff)
	}
	m.randMu.Lock()
	random := m.rng.Float64()
	m.randMu.Unlock()
	jitter := backoff * 0.2 * ((random * 2) - 1)
	delay := time.Duration(backoff + jitter)
	if delay < m.retry.MinBackoff {
		return m.retry.MinBackoff
	}
	if delay > m.retry.MaxBackoff {
		return m.retry.MaxBackoff
	}
	return delay
}

func normalizeState(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "", leaseStateActive:
		return leaseStateActive
	case leaseStateDraining:
		return leaseStateDraining
	default:
		return leaseStateActive
	}
}

func isRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, valkey.ErrClosing) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if vkErr, ok := valkey.IsValkeyErr(err); ok {
		if _, moved := vkErr.IsMoved(); moved {
			return true
		}
		if _, ask := vkErr.IsAsk(); ask {
			return true
		}
		if vkErr.IsTryAgain() || vkErr.IsLoading() || vkErr.IsClusterDown() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") || strings.Contains(msg, "broken pipe")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func defaultWorkerID() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

func messageToInt64(msg valkey.ValkeyMessage) (int64, error) {
	if value, err := msg.ToInt64(); err == nil {
		return value, nil
	}
	raw, err := msg.ToString()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

func keysForLease(lease Lease) (LeaseKeys, error) {
	if lease.keys.HashTag != "" && lease.keys.Lease != "" && lease.keys.Session != "" && lease.keys.Fence != "" {
		return lease.keys, nil
	}
	return KeysForRef(lease.Ref)
}

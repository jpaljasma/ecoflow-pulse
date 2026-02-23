package ingestlease

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	valkey "github.com/valkey-io/valkey-go"
)

func TestAcquireRenewRelease(t *testing.T) {
	t.Parallel()

	_, client, manager := setupTestManager(t, Config{
		LeaseTTL:          2 * time.Second,
		HeartbeatInterval: 500 * time.Millisecond,
		HeartbeatJitter:   0,
	})
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "R351ZABAPH331057"}

	acquired, err := manager.Acquire(ctx, ref, "worker-a", "token-a", CallOptions{})
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if !acquired.Acquired {
		t.Fatalf("expected acquired lease")
	}
	if acquired.Lease.Fence < 1 {
		t.Fatalf("expected fence >= 1, got %d", acquired.Lease.Fence)
	}

	renew, err := manager.Renew(ctx, acquired.Lease, leaseStateActive, CallOptions{})
	if err != nil {
		t.Fatalf("Renew error: %v", err)
	}
	if !renew.Renewed {
		t.Fatalf("expected renew success, reason=%s", renew.Reason)
	}

	release, err := manager.Release(ctx, ref, acquired.Lease.Token, CallOptions{})
	if err != nil {
		t.Fatalf("Release error: %v", err)
	}
	if !release.Released {
		t.Fatalf("expected release success, reason=%s", release.Reason)
	}

	keys, err := KeysForRef(ref)
	if err != nil {
		t.Fatalf("KeysForRef error: %v", err)
	}
	exists, err := existsKey(ctx, client, keys.Lease)
	if err != nil {
		t.Fatalf("existsKey error: %v", err)
	}
	if exists {
		t.Fatalf("expected lease key deleted")
	}
}

func TestAcquireFenceIncrementsAfterReacquire(t *testing.T) {
	t.Parallel()

	_, _, manager := setupTestManager(t, DefaultConfig())
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "Y711ZABA9H2P0294"}

	first, err := manager.Acquire(ctx, ref, "worker-a", "token-a", CallOptions{})
	if err != nil {
		t.Fatalf("first acquire error: %v", err)
	}
	if !first.Acquired {
		t.Fatalf("first acquire should succeed")
	}
	if _, err := manager.Release(ctx, ref, "token-a", CallOptions{}); err != nil {
		t.Fatalf("release error: %v", err)
	}
	second, err := manager.Acquire(ctx, ref, "worker-b", "token-b", CallOptions{})
	if err != nil {
		t.Fatalf("second acquire error: %v", err)
	}
	if !second.Acquired {
		t.Fatalf("second acquire should succeed")
	}
	if second.Lease.Fence != first.Lease.Fence+1 {
		t.Fatalf("fence mismatch: got=%d want=%d", second.Lease.Fence, first.Lease.Fence+1)
	}
}

func TestTokenMismatchRejected(t *testing.T) {
	t.Parallel()

	_, _, manager := setupTestManager(t, DefaultConfig())
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "R634ZABAWH2G1008"}

	acquired, err := manager.Acquire(ctx, ref, "worker-a", "token-a", CallOptions{})
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if !acquired.Acquired {
		t.Fatalf("expected acquire success")
	}

	releaseWrong, err := manager.Release(ctx, ref, "token-wrong", CallOptions{})
	if err != nil {
		t.Fatalf("Release wrong token error: %v", err)
	}
	if releaseWrong.Released {
		t.Fatalf("release should fail with mismatched token")
	}
	if releaseWrong.Reason != "token_mismatch" {
		t.Fatalf("unexpected reason: %s", releaseWrong.Reason)
	}

	mismatchLease := acquired.Lease
	mismatchLease.Token = "token-wrong"
	renewWrong, err := manager.Renew(ctx, mismatchLease, leaseStateActive, CallOptions{})
	if err != nil {
		t.Fatalf("Renew wrong token error: %v", err)
	}
	if renewWrong.Renewed {
		t.Fatalf("renew should fail with mismatched token")
	}
	if renewWrong.Reason != "token_mismatch" {
		t.Fatalf("unexpected renew reason: %s", renewWrong.Reason)
	}
}

func TestConcurrentAcquireSingleWinner(t *testing.T) {
	t.Parallel()

	_, _, manager := setupTestManager(t, Config{
		LeaseTTL:          5 * time.Second,
		HeartbeatInterval: 1 * time.Second,
		HeartbeatJitter:   0,
	})

	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "SN-concurrent"}

	const workers = 64
	var acquiredCount int32
	var winner Lease
	var winnerSet atomic.Bool
	var mu sync.Mutex
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			token := fmt.Sprintf("token-%d", i)
			workerID := fmt.Sprintf("worker-%d", i)
			result, err := manager.Acquire(ctx, ref, workerID, token, CallOptions{})
			if err != nil {
				t.Errorf("acquire error worker=%d: %v", i, err)
				return
			}
			if result.Acquired {
				atomic.AddInt32(&acquiredCount, 1)
				if winnerSet.CompareAndSwap(false, true) {
					mu.Lock()
					winner = result.Lease
					mu.Unlock()
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&acquiredCount); got != 1 {
		t.Fatalf("expected exactly one winner, got=%d", got)
	}

	mu.Lock()
	defer mu.Unlock()
	release, err := manager.Release(ctx, ref, winner.Token, CallOptions{})
	if err != nil {
		t.Fatalf("winner release error: %v", err)
	}
	if !release.Released {
		t.Fatalf("winner release failed: %s", release.Reason)
	}
}

func TestRunHeartbeatRenewsAndGracefullyDrains(t *testing.T) {
	t.Parallel()

	_, client, manager := setupTestManager(t, Config{
		LeaseTTL:          300 * time.Millisecond,
		HeartbeatInterval: 90 * time.Millisecond,
		HeartbeatJitter:   0,
		RetryPolicy: RetryPolicy{
			MaxAttempts: 1,
			MinBackoff:  5 * time.Millisecond,
			MaxBackoff:  10 * time.Millisecond,
		},
	})

	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "SN-heartbeat"}
	acquired, err := manager.Acquire(ctx, ref, "worker-heartbeat", "token-heartbeat", CallOptions{})
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}
	if !acquired.Acquired {
		t.Fatalf("expected acquire success")
	}

	beatCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- manager.RunHeartbeat(beatCtx, acquired.Lease, HeartbeatOptions{
			GracefulDrain: true,
		})
	}()

	time.Sleep(700 * time.Millisecond)
	keys, err := KeysForRef(ref)
	if err != nil {
		t.Fatalf("KeysForRef error: %v", err)
	}
	leaseExists, err := existsKey(ctx, client, keys.Lease)
	if err != nil {
		t.Fatalf("existsKey error: %v", err)
	}
	if !leaseExists {
		t.Fatalf("expected lease key to exist while heartbeat is active")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("heartbeat loop error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for heartbeat loop exit")
	}

	time.Sleep(50 * time.Millisecond)
	leaseExists, err = existsKey(ctx, client, keys.Lease)
	if err != nil {
		t.Fatalf("existsKey lease error: %v", err)
	}
	sessionExists, err := existsKey(ctx, client, keys.Session)
	if err != nil {
		t.Fatalf("existsKey session error: %v", err)
	}
	if leaseExists || sessionExists {
		t.Fatalf("expected lease/session deleted after graceful drain, lease=%v session=%v", leaseExists, sessionExists)
	}
}

func TestRunHeartbeatReleasesOnCancelWithoutDrain(t *testing.T) {
	t.Parallel()

	_, client, manager := setupTestManager(t, Config{
		LeaseTTL:          450 * time.Millisecond,
		HeartbeatInterval: 90 * time.Millisecond,
		HeartbeatJitter:   0,
		RetryPolicy: RetryPolicy{
			MaxAttempts: 1,
			MinBackoff:  5 * time.Millisecond,
			MaxBackoff:  10 * time.Millisecond,
		},
	})

	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "SN-heartbeat-release"}
	acquired, err := manager.Acquire(ctx, ref, "worker-heartbeat", "token-heartbeat", CallOptions{})
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}
	if !acquired.Acquired {
		t.Fatalf("expected acquire success")
	}

	beatCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- manager.RunHeartbeat(beatCtx, acquired.Lease, HeartbeatOptions{
			GracefulDrain: false,
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("heartbeat loop error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for heartbeat loop exit")
	}

	keys, err := KeysForRef(ref)
	if err != nil {
		t.Fatalf("KeysForRef error: %v", err)
	}
	leaseExists, err := existsKey(ctx, client, keys.Lease)
	if err != nil {
		t.Fatalf("existsKey lease error: %v", err)
	}
	sessionExists, err := existsKey(ctx, client, keys.Session)
	if err != nil {
		t.Fatalf("existsKey session error: %v", err)
	}
	if leaseExists || sessionExists {
		t.Fatalf("expected lease/session deleted after cancel cleanup, lease=%v session=%v", leaseExists, sessionExists)
	}
}

func TestIsRetryableErr(t *testing.T) {
	t.Parallel()

	if !isRetryableErr(valkey.ErrClosing) {
		t.Fatalf("expected valkey.ErrClosing retryable")
	}
	if !isRetryableErr(io.EOF) {
		t.Fatalf("expected EOF retryable")
	}
	if isRetryableErr(context.Canceled) {
		t.Fatalf("context canceled must not be retryable")
	}
}

func setupTestManager(tb testing.TB, cfg Config) (*miniredis.Miniredis, valkey.Client, *Manager) {
	tb.Helper()
	mini, err := miniredis.Run()
	if err != nil {
		tb.Fatalf("start miniredis: %v", err)
	}
	tb.Cleanup(mini.Close)

	client, err := NewValkeyClient(DefaultValkeyClientConfig([]string{mini.Addr()}))
	if err != nil {
		tb.Fatalf("NewValkeyClient: %v", err)
	}
	tb.Cleanup(client.Close)

	manager, err := NewManager(client, cfg)
	if err != nil {
		tb.Fatalf("NewManager: %v", err)
	}
	return mini, client, manager
}

func existsKey(ctx context.Context, client valkey.Client, key string) (bool, error) {
	result := client.Do(ctx, client.B().Exists().Key(key).Build())
	if err := result.Error(); err != nil {
		return false, err
	}
	count, err := result.ToInt64()
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

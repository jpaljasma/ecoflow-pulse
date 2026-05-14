package ingestlease

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	valkey "github.com/valkey-io/valkey-go"
)

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

func TestBuildValkeyClientOptionKeepsRetryBackoffEnabled(t *testing.T) {
	opt, err := buildValkeyClientOption(DefaultValkeyClientConfig([]string{"127.0.0.1:6379"}))
	if err != nil {
		t.Fatalf("build option: %v", err)
	}
	if opt.DisableRetry {
		t.Fatal("default valkey client disabled retries")
	}
	if opt.RetryDelay == nil {
		t.Fatal("default valkey client did not configure retry backoff")
	}
	first := opt.RetryDelay(1, valkey.Completed{}, errors.New("retry"))
	second := opt.RetryDelay(2, valkey.Completed{}, errors.New("retry"))
	if first < defaultRetryMinBackoff || first > defaultRetryMaxBackoff {
		t.Fatalf("first retry delay = %s, outside [%s,%s]", first, defaultRetryMinBackoff, defaultRetryMaxBackoff)
	}
	if second <= first || second > defaultRetryMaxBackoff {
		t.Fatalf("second retry delay = %s, want > %s and <= %s", second, first, defaultRetryMaxBackoff)
	}
}

func TestDefaultRetryDelayConcurrentSafe(t *testing.T) {
	delay := defaultClientRetryDelay()
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempt := 1; attempt <= 64; attempt++ {
				if got := delay(attempt, valkey.Completed{}, errors.New("retry")); got < defaultRetryMinBackoff || got > defaultRetryMaxBackoff {
					t.Errorf("retry delay = %s, outside [%s,%s]", got, defaultRetryMinBackoff, defaultRetryMaxBackoff)
				}
			}
		}()
	}
	wg.Wait()
}

func TestValkeyClientReconnectsAfterDisconnect(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := DefaultValkeyClientConfig([]string{server.Addr()})
	cfg.DialTimeout = 100 * time.Millisecond
	cfg.ConnWriteTimeout = 100 * time.Millisecond
	cfg.RetryDelay = func(int, valkey.Completed, error) time.Duration {
		return 10 * time.Millisecond
	}
	client, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}
	t.Cleanup(client.Close)

	ctx := context.Background()
	if err := client.Do(ctx, client.B().Set().Key("before").Value("ok").Build()).Error(); err != nil {
		t.Fatalf("initial set failed: %v", err)
	}
	server.Close()
	if err := server.Restart(); err != nil {
		t.Fatalf("restart miniredis: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		attemptCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		lastErr = client.Do(attemptCtx, client.B().Set().Key("after").Value("ok").Build()).Error()
		cancel()
		if lastErr == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("valkey client did not reconnect after server restart: %v", lastErr)
}

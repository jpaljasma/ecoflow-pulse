package workermetrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestStartServerShutdown(t *testing.T) {
	t.Parallel()

	metrics := New("test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartServer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.Registry(), "127.0.0.1:19111")
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19111/readyz")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("metrics server did not become ready")
}

func TestStartServerMarksReadyFalseOnDrain(t *testing.T) {
	t.Parallel()

	metrics := New("test")
	ctx, cancel := context.WithCancel(context.Background())
	stop := StartServer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.Registry(), "127.0.0.1:19112")
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19112/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	cancel()

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19112/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusServiceUnavailable {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("metrics server did not enter draining readiness state")
}

func TestStartServerDrainEndpointMarksReadyFalse(t *testing.T) {
	t.Parallel()

	metrics := New("test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartServer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.Registry(), "127.0.0.1:19113")
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19113/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:19113/drainz", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /drainz error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /drainz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Get("http://127.0.0.1:19113/readyz")
	if err != nil {
		t.Fatalf("GET /readyz after drain error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz after drain status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestMetricsConsumerSubscriptionReadiness(t *testing.T) {
	t.Parallel()

	metrics := New("test")
	if ok, reason := metrics.ReadyStatus(); !ok || reason != "ok" {
		t.Fatalf("ReadyStatus() = %v, %q; want ready ok", ok, reason)
	}

	metrics.RequireConsumerSubscription()
	if ok, reason := metrics.ReadyStatus(); ok || reason != "consumer_not_subscribed" {
		t.Fatalf("ReadyStatus() after requiring consumer = %v, %q; want not ready consumer_not_subscribed", ok, reason)
	}

	metrics.ObserveConsumerSubscribed("PULSE_TELEMETRY_INGEST", "rollup-timeseries-v1")
	if ok, reason := metrics.ReadyStatus(); !ok || reason != "ok" {
		t.Fatalf("ReadyStatus() after subscribe = %v, %q; want ready ok", ok, reason)
	}

	metrics.ObserveConsumerSubscribeFailure("PULSE_TELEMETRY_INGEST", "rollup-timeseries-v1", errors.New("boom"))
	if ok, reason := metrics.ReadyStatus(); ok || reason != "consumer_not_subscribed" {
		t.Fatalf("ReadyStatus() after subscribe failure = %v, %q; want not ready consumer_not_subscribed", ok, reason)
	}
}

func TestStartServerWithReadinessMarksReadyFalse(t *testing.T) {
	t.Parallel()

	metrics := New("test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartServerWithReadiness(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.Registry(), "127.0.0.1:19114", func() (bool, string) {
		return false, "consumer_not_subscribed"
	})
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19114/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusServiceUnavailable {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("metrics server did not expose readiness failure")
}

func BenchmarkMetricsStartMessage(b *testing.B) {
	metrics := New("bench")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := metrics.StartMessage()
		done("acked")
	}
}

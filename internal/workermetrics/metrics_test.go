package workermetrics

import (
	"context"
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

func BenchmarkMetricsStartMessage(b *testing.B) {
	metrics := New("bench")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := metrics.StartMessage()
		done("acked")
	}
}

package startupretry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRetryEventuallySucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	out, err := Retry(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", Options{
		Timeout:        100 * time.Millisecond,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
	}, func(_ context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("not ready")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if out != "ok" {
		t.Fatalf("output=%q want ok", out)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d want 3", attempts)
	}
}

func TestRetryStopsAfterTimeout(t *testing.T) {
	t.Parallel()

	_, err := Retry(context.Background(), nil, "test", Options{
		Timeout:        5 * time.Millisecond,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
	}, func(_ context.Context) (string, error) {
		return "", errors.New("still down")
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

package ecoflow

import (
	"net/http"
	"testing"
	"time"
)

func TestRetryPolicy_DelayForAttempt_UsesJitteredExponentialBackoff(t *testing.T) {
	policy := DefaultRetryPolicy()
	policy.BaseDelay = 100 * time.Millisecond
	policy.MaxDelay = 2 * time.Second
	policy.randomFloat64 = func() float64 { return 0.5 }

	delay := policy.delayForAttempt(time.Now(), nil, 3)
	// Attempt 3 => base * 2^(3-1) = 400ms; jitter 0.5 => 200ms.
	if delay != 200*time.Millisecond {
		t.Fatalf("delay mismatch: got %s", delay)
	}
}

func TestRetryPolicy_DelayForAttempt_PrefersRetryAfterHeader(t *testing.T) {
	policy := DefaultRetryPolicy()
	policy.MaxDelay = 10 * time.Second
	response := &http.Response{
		Header: make(http.Header),
	}
	response.Header.Set("Retry-After", "3")

	delay := policy.delayForAttempt(time.Now(), response, 1)
	if delay != 3*time.Second {
		t.Fatalf("delay mismatch: got %s", delay)
	}
}

func TestRetryPolicy_ShouldRetry_OnRetryableStatus(t *testing.T) {
	policy := DefaultRetryPolicy()
	response := &http.Response{StatusCode: http.StatusTooManyRequests}

	if !policy.shouldRetry(http.MethodGet, response, nil) {
		t.Fatal("expected retry on 429")
	}
	if policy.shouldRetry(http.MethodPost, response, nil) {
		t.Fatal("did not expect retry for POST by default")
	}
}

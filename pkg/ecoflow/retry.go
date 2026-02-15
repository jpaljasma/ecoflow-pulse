package ecoflow

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy defines retry eligibility and backoff behavior for outbound
// requests.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration

	RetryableStatusCodes map[int]struct{}
	RetryableMethods     map[string]struct{}

	randomFloat64 func() float64
}

// DefaultRetryPolicy returns pragmatic defaults for high-volume API clients.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 4,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		RetryableStatusCodes: map[int]struct{}{
			http.StatusTooManyRequests:     {},
			http.StatusRequestTimeout:      {},
			http.StatusBadGateway:          {},
			http.StatusServiceUnavailable:  {},
			http.StatusGatewayTimeout:      {},
			http.StatusInternalServerError: {},
		},
		RetryableMethods: map[string]struct{}{
			http.MethodGet:     {},
			http.MethodHead:    {},
			http.MethodPut:     {},
			http.MethodDelete:  {},
			http.MethodOptions: {},
		},
		randomFloat64: rand.Float64,
	}
}

func (p RetryPolicy) shouldRetry(method string, response *http.Response, requestErr error) bool {
	if p.MaxAttempts <= 1 {
		return false
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	if _, ok := p.RetryableMethods[method]; !ok {
		return false
	}

	if requestErr != nil {
		if errors.Is(requestErr, context.Canceled) || errors.Is(requestErr, context.DeadlineExceeded) {
			return false
		}
		var netErr interface{ Timeout() bool }
		if errors.As(requestErr, &netErr) {
			return true
		}
		return true
	}
	if response == nil {
		return false
	}
	_, ok := p.RetryableStatusCodes[response.StatusCode]
	return ok
}

func (p RetryPolicy) delayForAttempt(now time.Time, response *http.Response, retryNumber int) time.Duration {
	if retryNumber < 1 {
		retryNumber = 1
	}
	if retryAfterDelay, ok := parseRetryAfter(response, now); ok {
		if retryAfterDelay < 0 {
			return 0
		}
		if retryAfterDelay > p.MaxDelay {
			return p.MaxDelay
		}
		return retryAfterDelay
	}

	pow := math.Pow(2, float64(retryNumber-1))
	raw := float64(p.BaseDelay) * pow
	if raw > float64(p.MaxDelay) {
		raw = float64(p.MaxDelay)
	}

	randomFloat := p.randomFloat64
	if randomFloat == nil {
		randomFloat = rand.Float64
	}

	delay := time.Duration(raw * randomFloat())
	if delay < 0 {
		return 0
	}
	return delay
}

func parseRetryAfter(response *http.Response, now time.Time) (time.Duration, bool) {
	if response == nil {
		return 0, false
	}
	value := strings.TrimSpace(response.Header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}

	retryTime, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return retryTime.Sub(now), true
}

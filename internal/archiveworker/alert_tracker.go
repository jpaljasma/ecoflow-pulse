package archiveworker

import (
	"sync"
	"time"
)

type failureRateTracker struct {
	window    time.Duration
	threshold int
	cooldown  time.Duration

	mu        sync.Mutex
	lastAlert time.Time
	events    []time.Time
}

func newFailureRateTracker(window time.Duration, threshold int, cooldown time.Duration) *failureRateTracker {
	return &failureRateTracker{
		window:    window,
		threshold: threshold,
		cooldown:  cooldown,
		events:    make([]time.Time, 0, threshold*2),
	}
}

func (t *failureRateTracker) Record(now time.Time) (count int, perMinute float64, spike bool) {
	if t == nil || t.window <= 0 {
		return 0, 0, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := now.Add(-t.window)
	keep := t.events[:0]
	for _, ts := range t.events {
		if !ts.Before(cutoff) {
			keep = append(keep, ts)
		}
	}
	t.events = append(keep, now)
	count = len(t.events)
	perMinute = float64(count) / t.window.Minutes()
	if t.threshold <= 0 || count < t.threshold {
		return count, perMinute, false
	}
	if !t.lastAlert.IsZero() && now.Sub(t.lastAlert) < t.cooldown {
		return count, perMinute, false
	}
	t.lastAlert = now
	return count, perMinute, true
}

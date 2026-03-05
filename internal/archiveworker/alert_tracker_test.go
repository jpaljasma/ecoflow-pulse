package archiveworker

import (
	"testing"
	"time"
)

func TestFailureRateTrackerAlertingAndCooldown(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_700_000_000, 0).UTC()
	tracker := newFailureRateTracker(10*time.Minute, 3, 5*time.Minute)
	if tracker == nil {
		t.Fatal("tracker should not be nil")
	}

	count, perMin, spike := tracker.Record(base)
	if count != 1 || spike {
		t.Fatalf("first event mismatch: count=%d spike=%v", count, spike)
	}
	if perMin <= 0 {
		t.Fatalf("per-minute rate should be positive, got=%v", perMin)
	}

	count, _, spike = tracker.Record(base.Add(30 * time.Second))
	if count != 2 || spike {
		t.Fatalf("second event mismatch: count=%d spike=%v", count, spike)
	}

	count, _, spike = tracker.Record(base.Add(60 * time.Second))
	if count != 3 || !spike {
		t.Fatalf("third event should trigger spike: count=%d spike=%v", count, spike)
	}

	// Cooldown suppresses duplicate alerts while still tracking count.
	count, _, spike = tracker.Record(base.Add(90 * time.Second))
	if count != 4 || spike {
		t.Fatalf("cooldown event mismatch: count=%d spike=%v", count, spike)
	}

	// After cooldown expires and threshold still exceeded, alert should fire again.
	count, _, spike = tracker.Record(base.Add(7 * time.Minute))
	if count < 3 || !spike {
		t.Fatalf("post-cooldown event should trigger spike: count=%d spike=%v", count, spike)
	}

	// Old events should age out of the window.
	count, _, spike = tracker.Record(base.Add(20 * time.Minute))
	if count != 1 || spike {
		t.Fatalf("aged-out window mismatch: count=%d spike=%v", count, spike)
	}
}

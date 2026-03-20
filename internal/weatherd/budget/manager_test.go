package budget

import (
	"testing"
	"time"
)

func TestManagerEnforcesLimitsAndResetsWindows(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 4, 0, 0, time.UTC)
	nowRef := now
	manager := New(Config{
		DailyLimit:     4,
		PerMinuteLimit: 2,
		NowFn:          func() time.Time { return nowRef },
	})

	if !manager.Allow(2) {
		t.Fatal("first allowance should succeed")
	}
	if manager.Allow(1) {
		t.Fatal("per-minute budget should be exhausted")
	}

	nowRef = now.Add(time.Minute)
	if !manager.Allow(2) {
		t.Fatal("new minute should reset burst budget")
	}
	if manager.Allow(1) {
		t.Fatal("daily budget should now be exhausted")
	}

	nowRef = now.Add(24 * time.Hour)
	if !manager.Allow(2) {
		t.Fatal("new day should reset daily budget")
	}
}

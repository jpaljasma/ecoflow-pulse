package energydashboard

import (
	"testing"
	"time"
)

func TestResolveWindowTodayAcrossSpringForward(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetToday)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-03-09T04:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-09T16:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-03-09T04:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowYesterdayAcrossFallBack(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.November, 2, 12, 0, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetYesterday)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-11-01T04:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-11-02T05:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-10-31T04:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-11-01T04:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowPast24HoursUsesRollingTwentyFourHourWindow(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetPast24Hours)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-03-10T17:45:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-11T17:45:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-03-09T17:45:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-03-10T17:45:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowThisWeekUsesSundayStart(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetThisWeek)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-03-01T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowLastSevenDaysUsesCalendarDayStart(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetLast7Days)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-03-04T05:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-11T04:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-02-25T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-03-04T05:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowLastThirtyDaysUsesCalendarDayStart(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetLast30Days)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-02-09T05:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-11T04:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-01-10T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-02-09T05:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowLastMonthUsesCalendarMonthStart(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetLastMonth)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-02-01T05:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-01T05:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-01-01T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-02-01T05:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveWindowLastTwelveMonthsUsesCalendarMonthStart(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 11, 13, 45, 0, 0, loc)

	window, err := ResolveWindow(now, loc, PresetLast12Months)
	if err != nil {
		t.Fatalf("ResolveWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2025-04-01T04:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2024-04-01T04:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2025-04-01T04:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestParsePresetRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	if _, err := ParsePreset("quarter"); err == nil {
		t.Fatalf("expected ParsePreset to reject unknown value")
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()

	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) failed: %v", name, err)
	}
	return loc
}

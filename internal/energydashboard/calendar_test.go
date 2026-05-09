package energydashboard

import (
	"math"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

func TestResolveSelectedDateWindowUsesFullLocalDayForHistoricalDate(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, loc)

	window, err := ResolveSelectedDateWindow(now, loc, 2026, 3, 8)
	if err != nil {
		t.Fatalf("ResolveSelectedDateWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-03-09T04:00:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-03-07T05:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveSelectedDateWindowUsesMidnightToNowForCurrentDate(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.May, 3, 20, 15, 0, 0, loc)

	window, err := ResolveSelectedDateWindow(now, loc, 2026, 5, 3)
	if err != nil {
		t.Fatalf("ResolveSelectedDateWindow failed: %v", err)
	}

	if got, want := window.From.Format(time.RFC3339), "2026-05-03T04:00:00Z"; got != want {
		t.Fatalf("from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.To.Format(time.RFC3339), "2026-05-04T00:15:00Z"; got != want {
		t.Fatalf("to mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousFrom.Format(time.RFC3339), "2026-05-02T04:00:00Z"; got != want {
		t.Fatalf("previous from mismatch: got=%s want=%s", got, want)
	}
	if got, want := window.PreviousTo.Format(time.RFC3339), "2026-05-03T00:15:00Z"; got != want {
		t.Fatalf("previous to mismatch: got=%s want=%s", got, want)
	}
}

func TestResolveSelectedDateWindowHandlesDSTSensitiveBoundaries(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	cases := []struct {
		name     string
		now      time.Time
		year     int
		month    int
		day      int
		wantFrom string
		wantTo   string
	}{
		{
			name:     "spring-forward",
			now:      time.Date(2026, time.March, 9, 12, 0, 0, 0, loc),
			year:     2026,
			month:    3,
			day:      8,
			wantFrom: "2026-03-08T05:00:00Z",
			wantTo:   "2026-03-09T04:00:00Z",
		},
		{
			name:     "fall-back",
			now:      time.Date(2026, time.November, 2, 12, 0, 0, 0, loc),
			year:     2026,
			month:    11,
			day:      1,
			wantFrom: "2026-11-01T04:00:00Z",
			wantTo:   "2026-11-02T05:00:00Z",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			window, err := ResolveSelectedDateWindow(tc.now, loc, tc.year, tc.month, tc.day)
			if err != nil {
				t.Fatalf("ResolveSelectedDateWindow failed: %v", err)
			}
			if got := window.From.Format(time.RFC3339); got != tc.wantFrom {
				t.Fatalf("from mismatch: got=%s want=%s", got, tc.wantFrom)
			}
			if got := window.To.Format(time.RFC3339); got != tc.wantTo {
				t.Fatalf("to mismatch: got=%s want=%s", got, tc.wantTo)
			}
		})
	}
}

func TestBuildCalendarMonthReturnsSundayStartVisibleGridWithAdjacentDays(t *testing.T) {
	t.Parallel()

	loc := mustLoadLocation(t, "America/New_York")
	now := time.Date(2026, time.May, 15, 12, 0, 0, 0, loc)
	var queried []struct {
		from string
		to   string
	}

	calendar, err := BuildCalendarMonth(now, loc, 2026, 5, 0.30, "USD", func(from, to time.Time) (telemetryquery.Series, error) {
		queried = append(queried, struct {
			from string
			to   string
		}{
			from: from.In(loc).Format(time.RFC3339),
			to:   to.In(loc).Format(time.RFC3339),
		})
		points := make([]telemetryquery.Point, 0, 24)
		for day := from.In(loc); day.Before(to.In(loc)); day = day.AddDate(0, 0, 1) {
			dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).UTC()
			dayEnd := dayStart.AddDate(0, 0, 1)
			if dayEnd.After(to) {
				dayEnd = to
			}
			points = append(points, telemetryquery.Point{
				BucketStart: dayStart,
				BucketEnd:   dayEnd,
				Metrics: telemetryquery.Metrics{
					SolarGeneratedWh: floatPtr(1000),
				},
			})
		}
		return telemetryquery.Series{
			From:   from,
			To:     to,
			Points: points,
		}, nil
	})
	if err != nil {
		t.Fatalf("BuildCalendarMonth failed: %v", err)
	}

	if got, want := len(calendar.VisibleDays), 42; got != want {
		t.Fatalf("visible day count mismatch: got=%d want=%d", got, want)
	}
	if got, want := calendar.VisibleDays[0].Date, "2026-04-26"; got != want {
		t.Fatalf("first visible date mismatch: got=%s want=%s", got, want)
	}
	if calendar.VisibleDays[0].InSelectedMonth {
		t.Fatal("expected leading adjacent day to be outside the selected month")
	}
	if got, want := calendar.VisibleDays[len(calendar.VisibleDays)-1].Date, "2026-06-06"; got != want {
		t.Fatalf("last visible date mismatch: got=%s want=%s", got, want)
	}
	if calendar.VisibleDays[len(calendar.VisibleDays)-1].InSelectedMonth {
		t.Fatal("expected trailing adjacent day to be outside the selected month")
	}

	may15 := findCalendarDay(t, calendar.VisibleDays, "2026-05-15")
	if !may15.HasData {
		t.Fatal("expected current selected day to have data")
	}
	if may15.IsFuture {
		t.Fatal("expected current selected day not to be future")
	}
	if got, want := may15.SolarGeneratedKWh, 1.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("selected-day solar mismatch: got=%v want=%v", got, want)
	}
	if got, want := may15.EstimatedValue, 0.30; math.Abs(got-want) > 1e-9 {
		t.Fatalf("selected-day estimated value mismatch: got=%v want=%v", got, want)
	}
	if got, want := may15.Currency, "USD"; got != want {
		t.Fatalf("selected-day currency mismatch: got=%s want=%s", got, want)
	}

	futureDay := findCalendarDay(t, calendar.VisibleDays, "2026-05-20")
	if !futureDay.IsFuture {
		t.Fatal("expected future day to be flagged future")
	}
	if futureDay.HasData {
		t.Fatal("expected future day to skip data fetches")
	}
	if futureDay.SolarGeneratedKWh != 0 || futureDay.EstimatedValue != 0 {
		t.Fatalf("expected future day values to stay zero, got=%+v", futureDay)
	}

	if got, want := calendar.SelectedMonthTotals.SolarGeneratedKWh, 15.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("selected-month solar total mismatch: got=%v want=%v", got, want)
	}
	if got, want := calendar.SelectedMonthTotals.EstimatedValue, 4.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("selected-month estimated value mismatch: got=%v want=%v", got, want)
	}
	if got, want := calendar.SelectedMonthTotals.Currency, "USD"; got != want {
		t.Fatalf("selected-month currency mismatch: got=%s want=%s", got, want)
	}

	if got, want := len(queried), 1; got != want {
		t.Fatalf("calendar query count mismatch: got=%d want=%d", got, want)
	}
	if got, want := queried[0].from, "2026-04-26T00:00:00-04:00"; got != want {
		t.Fatalf("calendar query from mismatch: got=%s want=%s", got, want)
	}
	if got, want := queried[0].to, "2026-05-15T12:00:00-04:00"; got != want {
		t.Fatalf("calendar query to mismatch: got=%s want=%s", got, want)
	}
}

func findCalendarDay(t *testing.T, days []CalendarDay, date string) CalendarDay {
	t.Helper()

	for _, day := range days {
		if day.Date == date {
			return day
		}
	}
	t.Fatalf("calendar day %s not found", date)
	return CalendarDay{}
}

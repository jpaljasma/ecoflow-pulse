package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/scheduler"
)

func TestDefaultSchedulerJobsIncludesDailySolarActiveForecastRefresh(t *testing.T) {
	t.Setenv("SCHEDULER_SOLAR_REFRESH_INTERVAL", "")
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	jobs := defaultSchedulerJobs(now)

	var found *scheduler.RecurringJob
	for i := range jobs {
		if jobs[i].JobKey == "solar.refresh_active_forecasts" {
			found = &jobs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("defaultSchedulerJobs() missing solar.refresh_active_forecasts")
	}
	if got, want := found.JobType, "solar.refresh_active_forecasts"; got != want {
		t.Fatalf("solar refresh job type = %q, want %q", got, want)
	}
	if got, want := found.Interval, 24*time.Hour; got != want {
		t.Fatalf("solar refresh interval = %s, want %s", got, want)
	}
	if !found.Enabled {
		t.Fatal("solar refresh job should be enabled")
	}
	if !found.NextRunAt.Equal(now) {
		t.Fatalf("solar refresh next run = %s, want %s", found.NextRunAt, now)
	}
	if string(found.PayloadJSON) != `{}` {
		t.Fatalf("solar refresh payload = %s, want {}", string(found.PayloadJSON))
	}
}

func TestDefaultSchedulerJobsAllowsSolarRefreshIntervalOverride(t *testing.T) {
	t.Setenv("SCHEDULER_SOLAR_REFRESH_INTERVAL", "36h")
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	jobs := defaultSchedulerJobs(now)

	for _, job := range jobs {
		if job.JobKey == "solar.refresh_active_forecasts" {
			if got, want := job.Interval, 36*time.Hour; got != want {
				t.Fatalf("solar refresh interval = %s, want %s", got, want)
			}
			return
		}
	}
	t.Fatal("defaultSchedulerJobs() missing solar.refresh_active_forecasts")
}

func TestRunJobRefreshesActiveSolarForecasts(t *testing.T) {
	t.Setenv("SCHEDULER_SOLAR_REFRESH_BATCH_LIMIT", "7")
	refresher := &fakeActiveSolarForecastRefresher{}
	job := scheduler.RecurringJob{
		JobKey:  "solar.refresh_active_forecasts",
		JobType: "solar.refresh_active_forecasts",
	}

	err := runJob(context.Background(), slog.Default(), job, nil, nil, nil, refresher, nil)
	if err != nil {
		t.Fatalf("runJob() error = %v", err)
	}
	if refresher.calls != 1 {
		t.Fatalf("RefreshActiveSolarForecasts calls = %d, want 1", refresher.calls)
	}
	if refresher.limit != 7 {
		t.Fatalf("RefreshActiveSolarForecasts limit = %d, want 7", refresher.limit)
	}
	if refresher.now.IsZero() {
		t.Fatal("RefreshActiveSolarForecasts now was not set")
	}
}

type fakeActiveSolarForecastRefresher struct {
	calls int
	now   time.Time
	limit int
}

func (f *fakeActiveSolarForecastRefresher) RefreshActiveSolarForecasts(_ context.Context, now time.Time, limit int) (solarForecastRefreshStats, error) {
	f.calls++
	f.now = now
	f.limit = limit
	return solarForecastRefreshStats{Candidates: 1, Refreshed: 1}, nil
}

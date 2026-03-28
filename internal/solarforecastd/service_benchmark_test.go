package solarforecastd

import (
	"testing"
	"time"
)

func BenchmarkSummarizeVerificationRollups(b *testing.B) {
	records := benchmarkSolarVerificationRecords()
	nowUTC := time.Date(2026, 3, 19, 20, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = summarizeVerificationRollups("grid:42.61:-77.40:290|tilt:45|az:0|dev-a", records, nowUTC)
	}
}

func BenchmarkBuildRecentSiteCalibration(b *testing.B) {
	records := benchmarkSolarVerificationRecords()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildRecentSiteCalibration(records, "deterministic_baseline_v1")
	}
}

func benchmarkSolarVerificationRecords() []VerificationRecord {
	loc := mustLocationForBenchmark("America/New_York")
	issuedAt := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	dayStart := time.Date(2026, 3, 18, 0, 0, 0, 0, loc)
	deviceID := "device-1"
	records := make([]VerificationRecord, 0, 24)
	for hour := 0; hour < 24; hour++ {
		targetLocal := dayStart.Add(time.Duration(hour) * time.Hour)
		targetUTC := targetLocal.UTC()
		record := VerificationRecord{
			HourlyTrainingRecord: HourlyTrainingRecord{
				RunID:                        "run-1",
				SiteKey:                      "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				DeviceID:                     &deviceID,
				IssuedAt:                     issuedAt,
				TargetTime:                   targetUTC,
				TargetLocalDate:              parseDateISO("2026-03-18"),
				TargetLocalHour:              targetLocal.Hour(),
				TargetUTCOffsetMinutes:       -240,
				HorizonHours:                 24 - hour,
				HorizonBucket:                HorizonBucketDay1,
				ForecastGenerationWh:         100,
				BaselineForecastGenerationWh: float64Ptr(120),
				ActualGenerationWh:           float64Ptr(80),
				VerificationStatus:           VerificationStatusVerified,
				AbsoluteErrorWh:              float64Ptr(20),
				SquaredErrorWh2:              float64Ptr(400),
				BaselineAbsoluteErrorWh:      float64Ptr(40),
				BaselineSquaredErrorWh2:      float64Ptr(1600),
				VerifiedAt:                   timePtr(issuedAt.Add(2 * time.Hour)),
				UpdatedAt:                    issuedAt.Add(2 * time.Hour),
			},
			ForecastVersion: "deterministic_baseline_v1",
			ServedVariant:   "site_calibrated",
			Timezone:        "America/New_York",
		}
		records = append(records, record)
	}
	return records
}

func mustLocationForBenchmark(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMinuteTelemetryHistoryAggregatesByMinute(t *testing.T) {
	history := newMinuteTelemetryHistory(32)
	snapshot := newEnergySnapshot()

	base := time.Date(2026, time.February, 15, 10, 1, 5, 0, time.Local)
	snapshot.HasInPV = true
	snapshot.InPVWatts = 50
	snapshot.HasInAC = true
	snapshot.InACWatts = 800
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 100
	snapshot.HasOutDC = true
	snapshot.OutDCWatts = 20
	history.AddSample(base, snapshot)

	snapshot.InPVWatts = 70
	snapshot.InACWatts = 900
	snapshot.OutACWatts = 120
	snapshot.OutDCWatts = 40
	history.AddSample(base.Add(30*time.Second), snapshot)

	snapshot.InPVWatts = 30
	snapshot.InACWatts = 700
	snapshot.OutACWatts = 90
	snapshot.OutDCWatts = 10
	history.AddSample(base.Add(1*time.Minute), snapshot)

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 5, NewestFirst: true})
	if len(rows) != 2 {
		t.Fatalf("row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0][0] != "2026-02-15 10:02" {
		t.Fatalf("newest row time mismatch: got=%q want=%q", rows[0][0], "2026-02-15 10:02")
	}
	if rows[0][1] != "n/a" || rows[0][2] != "0.1" || rows[0][3] != "1.2" || rows[0][4] != "0.2" || rows[0][5] != "0.1" || rows[0][6] != "n/a" || rows[0][7] != "1.3" || rows[0][8] != "0.2" || rows[0][9] != "n/a" {
		t.Fatalf("newest row metrics mismatch: got=%v", rows[0])
	}
	if rows[1][0] != "2026-02-15 10:01" {
		t.Fatalf("older row time mismatch: got=%q want=%q", rows[1][0], "2026-02-15 10:01")
	}
	if rows[1][1] != "n/a" || rows[1][2] != "0.9" || rows[1][3] != "12.9" || rows[1][4] != "1.7" || rows[1][5] != "0.4" || rows[1][6] != "n/a" || rows[1][7] != "13.8" || rows[1][8] != "2.1" || rows[1][9] != "n/a" {
		t.Fatalf("older row averages mismatch: got=%v", rows[1])
	}
}

func TestMinuteTelemetryHistorySortAndLimit(t *testing.T) {
	history := newMinuteTelemetryHistory(32)
	snapshot := newEnergySnapshot()
	snapshot.HasInPV = true
	snapshot.InPVWatts = 10

	base := time.Date(2026, time.February, 15, 10, 0, 5, 0, time.Local)
	for i := 0; i < 3; i++ {
		snapshot.InPVWatts = float64(10 + i)
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 2, NewestFirst: false})
	if len(rows) != 2 {
		t.Fatalf("row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0][0] != "2026-02-15 10:00" || rows[1][0] != "2026-02-15 10:01" {
		t.Fatalf("oldest-first ordering mismatch: got=%v", rows)
	}
}

func TestMinuteTelemetryRowsUseBatteryNetForChargeAndNetWh(t *testing.T) {
	history := newMinuteTelemetryHistory(32)
	snapshot := newEnergySnapshot()

	at := time.Date(2026, time.February, 15, 12, 34, 10, 0, time.Local)
	snapshot.HasBatteryIn = true
	snapshot.BatteryInWatts = 120
	snapshot.HasBatteryOut = true
	snapshot.BatteryOutWatts = 20
	history.AddSample(at, snapshot)
	history.AddSample(at.Add(time.Minute), snapshot)

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 1, NewestFirst: false})
	if len(rows) != 1 {
		t.Fatalf("row count mismatch: got=%d want=1", len(rows))
	}
	// Battery net is +100W over 50s of elapsed time in the 12:34 bucket.
	if rows[0][6] != "1.4" {
		t.Fatalf("battery charge wh mismatch: got=%s want=1.4 row=%v", rows[0][6], rows[0])
	}
	if rows[0][9] != "1.4" {
		t.Fatalf("net wh mismatch: got=%s want=1.4 row=%v", rows[0][9], rows[0])
	}
}

func TestMinuteTelemetryStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_history.jsonl")
	store, err := newMinuteTelemetryStore(path)
	if err != nil {
		t.Fatalf("new minute telemetry store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	minuteA := minuteTelemetryBucket{
		MinuteStartUnix:       time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local).Unix(),
		SolarSumWatts:         1000,
		SolarSamples:          4,
		ACInSumWatts:          3600,
		ACInSamples:           4,
		ACOutSumWatts:         480,
		ACOutSamples:          4,
		DCOutSumWatts:         120,
		DCOutSamples:          4,
		BatteryChargeSumWatts: 3000,
		BatteryChargeSamples:  4,
	}
	minuteAUpdated := minuteA
	minuteAUpdated.SolarSumWatts = 1200
	minuteAUpdated.SolarSamples = 5

	minuteB := minuteTelemetryBucket{
		MinuteStartUnix:       time.Date(2026, time.February, 15, 10, 1, 0, 0, time.Local).Unix(),
		SolarSumWatts:         800,
		SolarSamples:          4,
		ACInSumWatts:          2400,
		ACInSamples:           4,
		ACOutSumWatts:         360,
		ACOutSamples:          4,
		DCOutSumWatts:         60,
		DCOutSamples:          4,
		BatteryChargeSumWatts: 2000,
		BatteryChargeSamples:  4,
	}

	if err := store.AppendBucket("SN-1", minuteA); err != nil {
		t.Fatalf("append minuteA: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteAUpdated); err != nil {
		t.Fatalf("append minuteAUpdated: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteB); err != nil {
		t.Fatalf("append minuteB: %v", err)
	}
	if err := store.AppendBucket("SN-2", minuteB); err != nil {
		t.Fatalf("append minuteB SN-2: %v", err)
	}

	loaded := newMinuteTelemetryHistory(32)
	loadedCount, err := store.LoadInto("SN-1", loaded)
	if err != nil {
		t.Fatalf("load history SN-1: %v", err)
	}
	if loadedCount != 2 {
		t.Fatalf("loadedCount mismatch: got=%d want=2", loadedCount)
	}
	if len(loaded.buckets) != 2 {
		t.Fatalf("loaded bucket count mismatch: got=%d want=2", len(loaded.buckets))
	}

	gotMinuteA, ok := loaded.Bucket(minuteA.MinuteStartUnix)
	if !ok {
		t.Fatalf("missing minuteA bucket")
	}
	if gotMinuteA.SolarSumWatts != minuteAUpdated.SolarSumWatts || gotMinuteA.SolarSamples != minuteAUpdated.SolarSamples {
		t.Fatalf("minuteA upsert mismatch: got=%+v want=%+v", gotMinuteA, minuteAUpdated)
	}

	other := newMinuteTelemetryHistory(32)
	otherCount, err := store.LoadInto("SN-2", other)
	if err != nil {
		t.Fatalf("load history SN-2: %v", err)
	}
	if otherCount != 1 || len(other.buckets) != 1 {
		t.Fatalf("SN-2 filtering mismatch: count=%d buckets=%d", otherCount, len(other.buckets))
	}
}

func TestMinuteTelemetryStoreLoadWindowFiltersOlderBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_history_window.jsonl")
	store, err := newMinuteTelemetryStore(path)
	if err != nil {
		t.Fatalf("new minute telemetry store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	oldMinute := time.Date(2026, time.February, 15, 9, 0, 0, 0, time.Local).Unix()
	newMinute := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local).Unix()

	if err := store.AppendBucket("SN-1", minuteTelemetryBucket{
		MinuteStartUnix: oldMinute,
		SolarSumWatts:   100,
		SolarSamples:    2,
	}); err != nil {
		t.Fatalf("append old minute: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteTelemetryBucket{
		MinuteStartUnix: newMinute,
		SolarSumWatts:   200,
		SolarSamples:    2,
	}); err != nil {
		t.Fatalf("append new minute: %v", err)
	}

	loaded := newMinuteTelemetryHistory(32)
	loadedCount, err := store.LoadIntoWindow("SN-1", loaded, newMinute)
	if err != nil {
		t.Fatalf("load history with window: %v", err)
	}
	if loadedCount != 1 {
		t.Fatalf("loadedCount mismatch: got=%d want=1", loadedCount)
	}
	if len(loaded.buckets) != 1 {
		t.Fatalf("loaded bucket count mismatch: got=%d want=1", len(loaded.buckets))
	}
	if _, ok := loaded.Bucket(oldMinute); ok {
		t.Fatalf("old bucket should be filtered out")
	}
	if _, ok := loaded.Bucket(newMinute); !ok {
		t.Fatalf("new bucket should be loaded")
	}
}

func TestMinuteTelemetryTracksAverageSOCPerMinute(t *testing.T) {
	history := newMinuteTelemetryHistory(16)
	snapshot := newEnergySnapshot()
	snapshot.HasDeviceSOC = true

	at := time.Date(2026, time.February, 15, 11, 15, 0, 0, time.Local)
	snapshot.DeviceSOC = 10
	history.AddSample(at, snapshot)
	snapshot.DeviceSOC = 20
	history.AddSample(at.Add(25*time.Second), snapshot)

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 1, NewestFirst: true})
	if len(rows) != 1 {
		t.Fatalf("row count mismatch: got=%d want=1", len(rows))
	}
	if rows[0][1] != "15.00" {
		t.Fatalf("soc minute average mismatch: got=%s want=15.00", rows[0][1])
	}
}

func TestMinuteTelemetryIntegratesSolarEnergyByElapsedTime(t *testing.T) {
	history := newMinuteTelemetryHistory(16)
	snapshot := newEnergySnapshot()
	snapshot.InPVWatts = 60
	snapshot.HasInPV = true

	start := time.Date(2026, time.February, 15, 12, 0, 0, 0, time.Local)
	history.AddSample(start, snapshot)

	snapshot.InPVWatts = 120
	history.AddSample(start.Add(30*time.Second), snapshot)
	history.AddSample(start.Add(time.Minute), snapshot)

	bucket, ok := history.Bucket(start.Unix())
	if !ok {
		t.Fatalf("missing bucket for integrated minute")
	}
	if bucket.SolarWattSeconds != 5400 {
		t.Fatalf("solar watt-seconds mismatch: got=%f want=5400", bucket.SolarWattSeconds)
	}

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 1, NewestFirst: false})
	if len(rows) != 1 {
		t.Fatalf("row count mismatch: got=%d want=1", len(rows))
	}
	if rows[0][2] != "1.5" {
		t.Fatalf("solar Wh mismatch: got=%s want=1.5", rows[0][2])
	}
}

func TestMinuteTelemetryStoreBackfillsLegacyEnergyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_history_legacy.jsonl")
	record := `{"version":1,"device_sn":"SN-1","minute_start_unix":1739631600,"solar_sum_watts":1200,"solar_samples":5}` + "\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatalf("write legacy history: %v", err)
	}

	store, err := newMinuteTelemetryStore(path)
	if err != nil {
		t.Fatalf("new minute telemetry store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	loaded := newMinuteTelemetryHistory(32)
	loadedCount, err := store.LoadInto("SN-1", loaded)
	if err != nil {
		t.Fatalf("load legacy history: %v", err)
	}
	if loadedCount != 1 {
		t.Fatalf("loadedCount mismatch: got=%d want=1", loadedCount)
	}

	bucket, ok := loaded.Bucket(1739631600)
	if !ok {
		t.Fatalf("missing legacy bucket")
	}
	if bucket.SolarWattSeconds != 14400 {
		t.Fatalf("legacy solar watt-seconds mismatch: got=%f want=14400", bucket.SolarWattSeconds)
	}
}

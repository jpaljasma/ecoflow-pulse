package main

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func TestTrainingTelemetryCSVStoreWritesHeaderAndRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_training.csv")
	store, err := newTrainingTelemetryCSVStore(path)
	if err != nil {
		t.Fatalf("new training telemetry csv store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	rowA := make([]string, len(trainingTelemetryCSVHeaders))
	rowB := make([]string, len(trainingTelemetryCSVHeaders))
	rowA[0] = "1"
	rowB[0] = "2"

	if err := store.AppendRow(rowA); err != nil {
		t.Fatalf("append rowA: %v", err)
	}
	if err := store.AppendRow(rowB); err != nil {
		t.Fatalf("append rowB: %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush rows: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read csv file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count mismatch: got=%d want=3 lines=%q", len(lines), string(raw))
	}
	if !strings.Contains(lines[0], "timestamp_utc") {
		t.Fatalf("missing header row: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "1,") {
		t.Fatalf("unexpected first data row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "2,") {
		t.Fatalf("unexpected second data row: %q", lines[2])
	}
}

func TestBuildTrainingTelemetryCSVRowIncludesRequestedFields(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 42.5
	snapshot.HasWattsIn = true
	snapshot.WattsIn = 240
	snapshot.HasWattsOut = true
	snapshot.WattsOut = 40
	snapshot.HasInAC = true
	snapshot.InACWatts = 220
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 35
	snapshot.HasOutDC = true
	snapshot.OutDCWatts = 5
	snapshot.HasChargeRemainTime = true
	snapshot.ChargeRemainTimeRaw = 480
	snapshot.HasDischargeRemainTime = true
	snapshot.DischargeRemainTimeRaw = 300
	snapshot.HasRemainTime = true
	snapshot.RemainTimeRaw = 420
	snapshot.HasPVLowChgState = true
	snapshot.PVLowChgStateRaw = 1
	snapshot.HasPVHighChgState = true
	snapshot.PVHighChgStateRaw = 0
	snapshot.HasACOn = true
	snapshot.ACOn = true
	snapshot.HasFanOn = true
	snapshot.FanOn = true
	snapshot.HasSolarLVVolts = true
	snapshot.SolarLVVolts = 33.8
	snapshot.HasSolarLVAmp = true
	snapshot.SolarLVAmp = 0.78
	snapshot.HasInPVLow = true
	snapshot.InPVLowWatts = 26
	snapshot.HasInPVHigh = true
	snapshot.InPVHighWatts = 12
	snapshot.HasXT150 = true
	snapshot.XT150Watts = -45
	snapshot.MQTTQueueDepth = 2
	snapshot.MQTTQueueDroppedOldest = 7
	snapshot.Packs[1] = &packSnapshot{SOC: 40, HasSOC: true, PowerW: -10, HasPower: true, TempC: 20, HasTemp: true}
	snapshot.Packs[2] = &packSnapshot{SOC: 45, HasSOC: true, PowerW: -8, HasPower: true, TempC: 21, HasTemp: true}

	at := time.Date(2026, time.February, 16, 11, 45, 0, 0, time.UTC)
	device := ecoflow.GeneralInfoDevice{
		SN:         "DEMOD2M00001057",
		DeviceName: "Kitchen Delta 2 Max",
	}
	envelope := telemetryEnvelope{
		TypeCode: "pdStatus",
		CmdID:    1,
		CmdFunc:  2,
		ID:       12345,
		Time:     67890,
	}
	row := buildTrainingTelemetryCSVRow(at, device, "/open/test/sn/quota", envelope, snapshot)
	if len(row) != len(trainingTelemetryCSVHeaders) {
		t.Fatalf("row width mismatch: got=%d want=%d", len(row), len(trainingTelemetryCSVHeaders))
	}

	byCol := make(map[string]string, len(trainingTelemetryCSVHeaders))
	for i, col := range trainingTelemetryCSVHeaders {
		byCol[col] = row[i]
	}

	if byCol["system_state"] != string(systemStateCharging) {
		t.Fatalf("system_state mismatch: got=%q want=%q", byCol["system_state"], systemStateCharging)
	}
	if byCol["ac_in_w"] != "220.000000" {
		t.Fatalf("ac_in_w mismatch: got=%q", byCol["ac_in_w"])
	}
	solarLow, err := strconv.ParseFloat(byCol["solar_low_in_w"], 64)
	if err != nil {
		t.Fatalf("parse solar_low_in_w=%q: %v", byCol["solar_low_in_w"], err)
	}
	if math.Abs(solarLow-26.364) > 0.001 {
		t.Fatalf("solar_low_in_w mismatch: got=%f want~26.364", solarLow)
	}
	if byCol["estimate_mode"] != string(systemStateCharging) {
		t.Fatalf("estimate_mode mismatch: got=%q want=%q", byCol["estimate_mode"], systemStateCharging)
	}
	if byCol["estimate_eta_min"] != "480" {
		t.Fatalf("estimate_eta_min mismatch: got=%q want=%q", byCol["estimate_eta_min"], "480")
	}
	if byCol["mppt_low_state"] != "charging" {
		t.Fatalf("mppt_low_state mismatch: got=%q", byCol["mppt_low_state"])
	}
	if byCol["mppt_high_state"] != "idle" {
		t.Fatalf("mppt_high_state mismatch: got=%q", byCol["mppt_high_state"])
	}
	if byCol["bp1_soc"] != "40.000000" || byCol["bp2_soc"] != "45.000000" {
		t.Fatalf("bp soc mismatch: bp1=%q bp2=%q", byCol["bp1_soc"], byCol["bp2_soc"])
	}
	if byCol["ac_on"] != "1" {
		t.Fatalf("ac_on mismatch: got=%q want=1", byCol["ac_on"])
	}
	if byCol["mqtt_queue_depth"] != "2" {
		t.Fatalf("mqtt_queue_depth mismatch: got=%q want=2", byCol["mqtt_queue_depth"])
	}
}

func TestTelemetryCaptureSchedulerEveryMessageWhenIntervalZero(t *testing.T) {
	scheduler := newTelemetryCaptureScheduler(0, 0.2)
	base := time.Date(2026, time.February, 16, 12, 0, 0, 0, time.UTC)
	if !scheduler.ShouldCapture(base) {
		t.Fatal("expected capture at base time")
	}
	if !scheduler.ShouldCapture(base.Add(250 * time.Millisecond)) {
		t.Fatal("expected capture at subsequent time with zero interval")
	}
}

func TestTelemetryCaptureSchedulerHonorsInterval(t *testing.T) {
	scheduler := newTelemetryCaptureScheduler(10*time.Second, 0)
	base := time.Date(2026, time.February, 16, 12, 0, 0, 0, time.UTC)
	if !scheduler.ShouldCapture(base) {
		t.Fatal("expected first capture")
	}
	if scheduler.ShouldCapture(base.Add(9 * time.Second)) {
		t.Fatal("did not expect capture before interval elapsed")
	}
	if !scheduler.ShouldCapture(base.Add(10 * time.Second)) {
		t.Fatal("expected capture after interval elapsed")
	}
}

package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

var trainingTelemetryCSVHeaders = []string{
	"ts_unix_ms",
	"timestamp_utc",
	"device_sn",
	"product_name",
	"topic",
	"type_code",
	"cmd_id",
	"cmd_func",
	"message_id",
	"message_time",
	"system_state",
	"soc_pct",
	"ac_in_w",
	"ac_out_w",
	"dc_in_w",
	"dc_out_w",
	"solar_in_w",
	"solar_low_in_w",
	"solar_high_in_w",
	"solar_low_v",
	"solar_low_a",
	"solar_high_v",
	"solar_high_a",
	"battery_in_w",
	"battery_out_w",
	"battery_net_w",
	"xt150_in_w",
	"xt150_out_w",
	"pack_charge_w",
	"pack_discharge_w",
	"estimate_mode",
	"estimate_eta_min",
	"estimate_source",
	"remain_charge_min",
	"remain_discharge_min",
	"remain_global_min",
	"mppt_low_state",
	"mppt_low_state_raw",
	"mppt_high_state",
	"mppt_high_state_raw",
	"bp_count",
	"bp1_soc",
	"bp2_soc",
	"bp3_soc",
	"bp4_soc",
	"bp5_soc",
	"bp1_power_w",
	"bp2_power_w",
	"bp3_power_w",
	"bp4_power_w",
	"bp5_power_w",
	"bp1_temp_c",
	"bp2_temp_c",
	"bp3_temp_c",
	"bp4_temp_c",
	"bp5_temp_c",
	"ac_on",
	"dc_on",
	"usb_on",
	"dc12v_on",
	"ev_charging_on",
	"fan_on",
	"solar_charging_on",
	"mqtt_queue_depth",
	"mqtt_queue_dropped_oldest",
}

type trainingTelemetryCSVStore struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	writer *csv.Writer
}

func newTrainingTelemetryCSVStore(path string) (*trainingTelemetryCSVStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("training telemetry csv path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create training telemetry csv directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open training telemetry csv file: %w", err)
	}
	store := &trainingTelemetryCSVStore{
		path:   path,
		file:   file,
		writer: csv.NewWriter(file),
	}
	shouldWriteHeader := true
	if info, statErr := file.Stat(); statErr == nil && info.Size() > 0 {
		shouldWriteHeader = false
	}
	if shouldWriteHeader {
		if err := store.writer.Write(trainingTelemetryCSVHeaders); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("write training telemetry csv header: %w", err)
		}
		store.writer.Flush()
		if err := store.writer.Error(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("flush training telemetry csv header: %w", err)
		}
	}
	return store, nil
}

func (s *trainingTelemetryCSVStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *trainingTelemetryCSVStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer != nil {
		s.writer.Flush()
	}
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	s.writer = nil
	return err
}

func (s *trainingTelemetryCSVStore) AppendRow(row []string) error {
	if s == nil {
		return nil
	}
	if len(row) != len(trainingTelemetryCSVHeaders) {
		return fmt.Errorf("training telemetry csv row width mismatch: got=%d want=%d", len(row), len(trainingTelemetryCSVHeaders))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil || s.writer == nil {
		return errors.New("training telemetry csv file is closed")
	}
	if err := s.writer.Write(row); err != nil {
		return fmt.Errorf("append training telemetry csv row: %w", err)
	}
	s.writer.Flush()
	if err := s.writer.Error(); err != nil {
		return fmt.Errorf("flush training telemetry csv row: %w", err)
	}
	return nil
}

type telemetryCaptureScheduler struct {
	interval     time.Duration
	jitterFactor float64
	nextDue      time.Time
}

func newTelemetryCaptureScheduler(interval time.Duration, jitterFactor float64) *telemetryCaptureScheduler {
	if interval < 0 {
		interval = 0
	}
	if jitterFactor < 0 {
		jitterFactor = 0
	}
	return &telemetryCaptureScheduler{
		interval:     interval,
		jitterFactor: jitterFactor,
	}
}

func (s *telemetryCaptureScheduler) ShouldCapture(now time.Time) bool {
	if s == nil || s.interval <= 0 {
		return true
	}
	if s.nextDue.IsZero() || !now.Before(s.nextDue) {
		wait := applyJitter(s.interval, s.jitterFactor)
		if wait <= 0 {
			wait = s.interval
		}
		s.nextDue = now.Add(wait)
		return true
	}
	return false
}

func isPowerRelatedTelemetry(envelope telemetryEnvelope, quota map[string]any) bool {
	typeCode := strings.ToLower(strings.TrimSpace(envelope.TypeCode))
	switch {
	case typeCode == "pdstatus",
		typeCode == "invstatus",
		typeCode == "mpptstatus",
		typeCode == "emsstatus",
		typeCode == "kitinfo",
		typeCode == "bpinfo",
		typeCode == "daddr":
		return true
	case strings.HasPrefix(typeCode, "bmsstatus"),
		strings.HasPrefix(typeCode, "bmsslavestatus"):
		return true
	}

	addr := strings.ToLower(strings.TrimSpace(envelope.Addr))
	if strings.Contains(addr, "pd_appshow") ||
		strings.Contains(addr, "pd_backend") ||
		strings.Contains(addr, "bms_slave") ||
		strings.Contains(addr, "bms_ems") ||
		strings.Contains(addr, "pd_bp_addr") ||
		strings.Contains(addr, "d_addr") {
		return true
	}

	for key := range quota {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "" {
			continue
		}
		if strings.Contains(lower, "watt") ||
			strings.Contains(lower, "pwr") ||
			strings.Contains(lower, "amp") ||
			strings.Contains(lower, "vol") ||
			strings.Contains(lower, "remaintime") {
			return true
		}
	}
	return false
}

func buildTrainingTelemetryCSVRow(
	at time.Time,
	device ecoflow.GeneralInfoDevice,
	topic string,
	envelope telemetryEnvelope,
	snapshot *energySnapshot,
) []string {
	if snapshot == nil {
		snapshot = newEnergySnapshot()
	}

	packChargeW, packDischargeW := packPowerTotals(snapshot.Packs)
	effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut :=
		snapshot.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	flow := snapshot.batteryFlowForDisplay(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	systemState := snapshot.detectSystemState(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	soc, hasSOC := snapshot.displaySOC()
	pvTotalW, hasPVTotal, pvLowW, hasPVLow, pvHighW, hasPVHigh := snapshot.effectivePVInputChannels()

	xt150InW, hasXT150In, xt150OutW, hasXT150Out := xt150Directional(snapshot.HasXT150, snapshot.XT150Watts)

	estimateETAMin, estimateSource, hasEstimateETA := snapshot.selectRemainForState(systemState)
	estimateMode := deriveEstimateMode(systemState, estimateSource, hasEstimateETA)

	packSOCCols, packPowerCols, packTempCols, packCount := buildPackCSVColumns(snapshot.Packs)
	solarChargingKnown, solarChargingOn := snapshot.solarChargingStatus()

	row := []string{
		strconv.FormatInt(at.UnixMilli(), 10),
		at.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(device.SN),
		strings.TrimSpace(device.ProductName),
		strings.TrimSpace(topic),
		strings.TrimSpace(envelope.TypeCode),
		strconv.FormatInt(envelope.CmdID, 10),
		strconv.FormatInt(envelope.CmdFunc, 10),
		strconv.FormatInt(envelope.ID, 10),
		strconv.FormatInt(envelope.Time, 10),
		string(systemState),
		csvOptionalFloat(hasSOC, soc),
		csvOptionalFloat(snapshot.HasInAC, snapshot.InACWatts),
		csvOptionalFloat(snapshot.HasOutAC, snapshot.OutACWatts),
		csvOptionalFloat(flow.hasIn, flow.inWatts),
		csvOptionalFloat(snapshot.HasOutDC, snapshot.OutDCWatts),
		csvOptionalFloat(hasPVTotal, pvTotalW),
		csvOptionalFloat(hasPVLow, pvLowW),
		csvOptionalFloat(hasPVHigh, pvHighW),
		csvOptionalFloat(snapshot.HasSolarLVVolts, snapshot.SolarLVVolts),
		csvOptionalFloat(snapshot.HasSolarLVAmp, snapshot.SolarLVAmp),
		csvOptionalFloat(snapshot.HasSolarHVVolts, snapshot.SolarHVVolts),
		csvOptionalFloat(snapshot.HasSolarHVAmp, snapshot.SolarHVAmp),
		csvOptionalFloat(flow.hasIn, flow.inWatts),
		csvOptionalFloat(flow.hasOut, flow.outWatts),
		csvOptionalFloat(flow.hasNet, flow.netWatts),
		csvOptionalFloat(hasXT150In, xt150InW),
		csvOptionalFloat(hasXT150Out, xt150OutW),
		csvFloat(packChargeW),
		csvFloat(packDischargeW),
		estimateMode,
		csvOptionalInt64(hasEstimateETA, estimateETAMin),
		estimateSource,
		csvOptionalInt64(snapshot.HasChargeRemainTime, snapshot.ChargeRemainTimeRaw),
		csvOptionalInt64(snapshot.HasDischargeRemainTime, snapshot.DischargeRemainTimeRaw),
		csvOptionalInt64(snapshot.HasRemainTime, snapshot.RemainTimeRaw),
		mpptStateLabel(snapshot.HasPVLowChgState, snapshot.PVLowChgStateRaw),
		csvOptionalInt64(snapshot.HasPVLowChgState, snapshot.PVLowChgStateRaw),
		mpptStateLabel(snapshot.HasPVHighChgState, snapshot.PVHighChgStateRaw),
		csvOptionalInt64(snapshot.HasPVHighChgState, snapshot.PVHighChgStateRaw),
		strconv.Itoa(packCount),
	}

	row = append(row, packSOCCols[:]...)
	row = append(row, packPowerCols[:]...)
	row = append(row, packTempCols[:]...)
	row = append(row,
		csvOptionalBool(snapshot.HasACOn, snapshot.ACOn),
		csvOptionalBool(snapshot.HasDCOn, snapshot.DCOn),
		csvOptionalBool(snapshot.HasUSBOn, snapshot.USBOn),
		csvOptionalBool(snapshot.HasDC12VOn, snapshot.DC12VOn),
		csvOptionalBool(snapshot.HasEVChargingOn, snapshot.EVChargingOn),
		csvOptionalBool(snapshot.HasFanOn, snapshot.FanOn),
		csvOptionalBool(solarChargingKnown, solarChargingOn),
		strconv.Itoa(snapshot.MQTTQueueDepth),
		strconv.FormatUint(snapshot.MQTTQueueDroppedOldest, 10),
	)
	return row
}

func buildPackCSVColumns(packs map[int]*packSnapshot) (soc [5]string, power [5]string, temp [5]string, count int) {
	for packNo, pack := range packs {
		if pack == nil {
			continue
		}
		count++
		if packNo < 1 || packNo > len(soc) {
			continue
		}
		idx := packNo - 1
		soc[idx] = csvOptionalFloat(pack.HasSOC, pack.SOC)
		power[idx] = csvOptionalFloat(pack.HasPower, pack.PowerW)
		temp[idx] = csvOptionalFloat(pack.HasTemp, pack.TempC)
	}
	return soc, power, temp, count
}

func csvOptionalFloat(has bool, value float64) string {
	if !has {
		return ""
	}
	return csvFloat(value)
}

func csvFloat(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ""
	}
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func csvOptionalInt64(has bool, value int64) string {
	if !has {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func csvOptionalBool(has bool, value bool) string {
	if !has {
		return ""
	}
	if value {
		return "1"
	}
	return "0"
}

func xt150Directional(hasXT150 bool, totalWatts float64) (inW float64, hasIn bool, outW float64, hasOut bool) {
	if !hasXT150 {
		return 0, false, 0, false
	}
	// XT150 sign convention: negative means battery -> inverter (device input).
	if totalWatts < 0 {
		return -totalWatts, true, 0, true
	}
	return 0, true, totalWatts, true
}

func mpptStateLabel(hasRaw bool, raw int64) string {
	if !hasRaw {
		return ""
	}
	if isMPPTChargeStateActive(raw) {
		return "charging"
	}
	if raw == 0 {
		return "idle"
	}
	return fmt.Sprintf("state_%d", raw)
}

func deriveEstimateMode(state systemStateKind, source string, hasEstimate bool) string {
	if hasEstimate {
		switch source {
		case "charge":
			return string(systemStateCharging)
		case "discharge":
			return string(systemStateDischarging)
		}
	}
	return string(state)
}

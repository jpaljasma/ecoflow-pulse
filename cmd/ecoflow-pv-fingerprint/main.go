package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultInputCSV  = "logs/telemetry_training.csv"
	defaultOutputCSV = "logs/pv_fingerprint.csv"
)

type pvCapability struct {
	MinVolts float64
	MaxVolts float64
	MaxAmps  float64
	MaxWatts float64
}

type panelHint struct {
	DeviceSN      string  `json:"device_sn"`
	ProductName   string  `json:"product_name"`
	Port          string  `json:"port"`
	PanelSetup    string  `json:"panel_setup"`
	PanelCount    int     `json:"panel_count"`
	NominalTotalW float64 `json:"nominal_total_w"`
}

type panelHintIndex struct {
	bySNPort      map[string]panelHint
	byProductPort map[string]panelHint
}

type portStats struct {
	SN   string
	Name string
	Port string

	Samples       int
	ActiveSamples int
	FirstTSUnixMS int64
	LastTSUnixMS  int64

	Watts        []float64
	ActiveWatts  []float64
	VoltsNonZero []float64
	AmpsNonZero  []float64

	States map[string]int
}

type fingerprintRow struct {
	SN                string
	Name              string
	Port              string
	PanelSetup        string
	PanelCount        int
	NominalTotalW     float64
	Samples           int
	ActiveSamples     int
	ActivePct         float64
	AvgW              float64
	MedianW           float64
	AvgActiveW        float64
	MedianActiveW     float64
	MaxW              float64
	AvgVNonZero       float64
	MedianVNonZero    float64
	MaxV              float64
	AvgANonZero       float64
	MedianANonZero    float64
	MaxA              float64
	CapW              float64
	CapVRange         string
	CapA              float64
	MaxWUtilPct       float64
	CapHeadroomW      float64
	StateEmptyPct     float64
	StateIdlePct      float64
	StateChargingPct  float64
	DurationHours     float64
	EstimatedEnergyWh float64
}

func main() {
	var (
		inputCSV  string
		outputCSV string
		panelMap  string
	)

	flag.StringVar(&inputCSV, "csv", defaultInputCSV, "Path to telemetry training CSV")
	flag.StringVar(&outputCSV, "out", defaultOutputCSV, "Output CSV path (or '-' for stdout)")
	flag.StringVar(&panelMap, "panel-map", "", "Optional JSON file with panel mapping hints")
	flag.Parse()

	hints, err := loadPanelHints(panelMap)
	if err != nil {
		fatalf("load panel hints: %v", err)
	}

	rows, err := analyzePVFingerprints(inputCSV, hints)
	if err != nil {
		fatalf("analyze pv fingerprints: %v", err)
	}

	var writer io.Writer
	if strings.TrimSpace(outputCSV) == "-" {
		writer = os.Stdout
	} else {
		if err := os.MkdirAll(filepath.Dir(outputCSV), 0o755); err != nil {
			fatalf("create output directory: %v", err)
		}
		file, err := os.Create(outputCSV)
		if err != nil {
			fatalf("create output file: %v", err)
		}
		defer func() {
			_ = file.Close()
		}()
		writer = file
	}

	if err := writeFingerprintCSV(writer, rows); err != nil {
		fatalf("write output: %v", err)
	}
	if strings.TrimSpace(outputCSV) != "-" {
		fmt.Printf("wrote %d fingerprint rows to %s\n", len(rows), outputCSV)
	}
}

func analyzePVFingerprints(path string, hints panelHintIndex) ([]fingerprintRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	allRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(allRows) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	header := mapHeaderIndex(allRows[0])
	required := []string{
		"ts_unix_ms",
		"device_sn",
		"product_name",
		"solar_low_in_w",
		"solar_high_in_w",
		"solar_low_v",
		"solar_high_v",
		"solar_low_a",
		"solar_high_a",
		"mppt_low_state",
		"mppt_high_state",
	}
	for _, key := range required {
		if _, ok := header[key]; !ok {
			return nil, fmt.Errorf("missing required column %q", key)
		}
	}

	get := func(row []string, key string) string {
		idx := header[key]
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	stats := map[string]*portStats{}
	for _, row := range allRows[1:] {
		ts := parseInt64(get(row, "ts_unix_ms"))
		sn := get(row, "device_sn")
		name := get(row, "product_name")
		if sn == "" || name == "" {
			continue
		}

		low := ensurePortStats(stats, sn, name, "low")
		updatePortStats(
			low,
			ts,
			parseFloat(get(row, "solar_low_in_w")),
			parseFloat(get(row, "solar_low_v")),
			parseFloat(get(row, "solar_low_a")),
			get(row, "mppt_low_state"),
		)

		high := ensurePortStats(stats, sn, name, "high")
		updatePortStats(
			high,
			ts,
			parseFloat(get(row, "solar_high_in_w")),
			parseFloat(get(row, "solar_high_v")),
			parseFloat(get(row, "solar_high_a")),
			get(row, "mppt_high_state"),
		)
	}

	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ai := strings.Split(keys[i], "|")
		aj := strings.Split(keys[j], "|")
		if ai[0] != aj[0] {
			return ai[0] < aj[0]
		}
		if ai[1] != aj[1] {
			return ai[1] < aj[1]
		}
		// low before high
		return ai[2] < aj[2]
	})

	out := make([]fingerprintRow, 0, len(keys))
	for _, key := range keys {
		s := stats[key]
		if s.Samples == 0 {
			continue
		}
		capability, hasCap := estimatePVInputCapability(s.Name, s.Port)
		hint, hasHint := hints.Resolve(s.SN, s.Name, s.Port)

		row := fingerprintRow{
			SN:               s.SN,
			Name:             s.Name,
			Port:             s.Port,
			PanelSetup:       "(unknown)",
			PanelCount:       0,
			NominalTotalW:    0,
			Samples:          s.Samples,
			ActiveSamples:    s.ActiveSamples,
			ActivePct:        ratioPercent(float64(s.ActiveSamples), float64(s.Samples)),
			AvgW:             average(s.Watts),
			MedianW:          median(s.Watts),
			AvgActiveW:       average(s.ActiveWatts),
			MedianActiveW:    median(s.ActiveWatts),
			MaxW:             maxValue(s.Watts),
			AvgVNonZero:      average(s.VoltsNonZero),
			MedianVNonZero:   median(s.VoltsNonZero),
			MaxV:             maxValue(s.VoltsNonZero),
			AvgANonZero:      average(s.AmpsNonZero),
			MedianANonZero:   median(s.AmpsNonZero),
			MaxA:             maxValue(s.AmpsNonZero),
			StateEmptyPct:    ratioPercent(float64(s.States["(empty)"]), float64(s.Samples)),
			StateIdlePct:     ratioPercent(float64(s.States["idle"]), float64(s.Samples)),
			StateChargingPct: ratioPercent(float64(s.States["charging"]), float64(s.Samples)),
		}

		if s.LastTSUnixMS > s.FirstTSUnixMS {
			row.DurationHours = float64(s.LastTSUnixMS-s.FirstTSUnixMS) / 3_600_000.0
		}
		row.EstimatedEnergyWh = row.AvgW * row.DurationHours

		if hasCap {
			row.CapW = capability.MaxWatts
			row.CapVRange = fmt.Sprintf("%.0f-%.0f", capability.MinVolts, capability.MaxVolts)
			row.CapA = capability.MaxAmps
			if row.CapW > 0 {
				row.MaxWUtilPct = ratioPercent(row.MaxW, row.CapW)
				row.CapHeadroomW = row.CapW - row.MaxW
				if row.CapHeadroomW < 0 {
					row.CapHeadroomW = 0
				}
			}
		}

		if hasHint {
			if strings.TrimSpace(hint.PanelSetup) != "" {
				row.PanelSetup = hint.PanelSetup
			}
			if hint.PanelCount > 0 {
				row.PanelCount = hint.PanelCount
			}
			if hint.NominalTotalW > 0 {
				row.NominalTotalW = hint.NominalTotalW
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func writeFingerprintCSV(w io.Writer, rows []fingerprintRow) error {
	writer := csv.NewWriter(w)
	header := []string{
		"sn",
		"name",
		"port",
		"panel_setup",
		"panel_count",
		"nominal_total_w",
		"samples",
		"active_samples",
		"active_pct",
		"avg_w",
		"median_w",
		"avg_active_w",
		"median_active_w",
		"max_w",
		"avg_v_nonzero",
		"median_v_nonzero",
		"max_v",
		"avg_a_nonzero",
		"median_a_nonzero",
		"max_a",
		"cap_w",
		"max_w_util_pct",
		"cap_v_range",
		"cap_a",
		"cap_headroom_w",
		"state_empty_pct",
		"state_idle_pct",
		"state_charging_pct",
		"duration_h",
		"est_wh",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, row := range rows {
		record := []string{
			row.SN,
			row.Name,
			row.Port,
			row.PanelSetup,
			strconv.Itoa(row.PanelCount),
			floatFormat(row.NominalTotalW),
			strconv.Itoa(row.Samples),
			strconv.Itoa(row.ActiveSamples),
			floatFormat(row.ActivePct),
			floatFormat(row.AvgW),
			floatFormat(row.MedianW),
			floatFormat(row.AvgActiveW),
			floatFormat(row.MedianActiveW),
			floatFormat(row.MaxW),
			floatFormat(row.AvgVNonZero),
			floatFormat(row.MedianVNonZero),
			floatFormat(row.MaxV),
			floatFormat(row.AvgANonZero),
			floatFormat(row.MedianANonZero),
			floatFormat(row.MaxA),
			floatFormat(row.CapW),
			floatFormat(row.MaxWUtilPct),
			row.CapVRange,
			floatFormat(row.CapA),
			floatFormat(row.CapHeadroomW),
			floatFormat(row.StateEmptyPct),
			floatFormat(row.StateIdlePct),
			floatFormat(row.StateChargingPct),
			floatFormat(row.DurationHours),
			floatFormat(row.EstimatedEnergyWh),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush writer: %w", err)
	}
	return nil
}

func ensurePortStats(stats map[string]*portStats, sn, name, port string) *portStats {
	k := portKey(sn, name, port)
	if existing, ok := stats[k]; ok {
		return existing
	}
	created := &portStats{
		SN:     sn,
		Name:   name,
		Port:   normalizePort(port),
		States: map[string]int{},
	}
	stats[k] = created
	return created
}

func updatePortStats(s *portStats, ts int64, watts, volts, amps float64, state string) {
	if s == nil {
		return
	}
	s.Samples++
	s.Watts = append(s.Watts, watts)
	if watts > 0 {
		s.ActiveSamples++
		s.ActiveWatts = append(s.ActiveWatts, watts)
	}
	if volts > 0 {
		s.VoltsNonZero = append(s.VoltsNonZero, volts)
	}
	if amps > 0 {
		s.AmpsNonZero = append(s.AmpsNonZero, amps)
	}
	state = strings.TrimSpace(strings.ToLower(state))
	if state == "" {
		state = "(empty)"
	}
	s.States[state]++

	if s.FirstTSUnixMS == 0 || (ts > 0 && ts < s.FirstTSUnixMS) {
		s.FirstTSUnixMS = ts
	}
	if ts > s.LastTSUnixMS {
		s.LastTSUnixMS = ts
	}
}

func estimatePVInputCapability(productName, port string) (pvCapability, bool) {
	lower := strings.ToLower(strings.TrimSpace(productName))
	port = normalizePort(port)
	switch {
	case strings.Contains(lower, "delta 2 max"):
		return pvCapability{MinVolts: 11, MaxVolts: 60, MaxAmps: 15, MaxWatts: 500}, true
	case strings.Contains(lower, "delta pro ultra"):
		if port == "high" {
			return pvCapability{MinVolts: 80, MaxVolts: 450, MaxAmps: 15, MaxWatts: 4000}, true
		}
		return pvCapability{MinVolts: 30, MaxVolts: 150, MaxAmps: 15, MaxWatts: 1600}, true
	default:
		return pvCapability{}, false
	}
}

func loadPanelHints(path string) (panelHintIndex, error) {
	index := newPanelHintIndex()
	for _, hint := range defaultPanelHints() {
		index.Upsert(hint)
	}
	if strings.TrimSpace(path) == "" {
		return index, nil
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return panelHintIndex{}, fmt.Errorf("read panel-map: %w", err)
	}
	var hints []panelHint
	if err := json.Unmarshal(file, &hints); err != nil {
		var wrapped struct {
			Panels []panelHint `json:"panels"`
		}
		if err2 := json.Unmarshal(file, &wrapped); err2 != nil {
			return panelHintIndex{}, fmt.Errorf("parse panel-map json: %w", err)
		}
		hints = wrapped.Panels
	}
	for _, hint := range hints {
		index.Upsert(hint)
	}
	return index, nil
}

func defaultPanelHints() []panelHint {
	return []panelHint{
		{
			DeviceSN:      "Y711ZABA9H2P0294",
			Port:          "low",
			PanelSetup:    "2x400W JA Solar bifacial",
			PanelCount:    2,
			NominalTotalW: 800,
		},
		{
			DeviceSN:      "Y711ZABA9H2P0294",
			Port:          "high",
			PanelSetup:    "(none connected)",
			PanelCount:    0,
			NominalTotalW: 0,
		},
		{
			DeviceSN:      "R351ZABAPH331057",
			Port:          "high",
			PanelSetup:    "4x125W EcoFlow Bifacial Modular",
			PanelCount:    4,
			NominalTotalW: 500,
		},
		{
			DeviceSN:      "R351ZABAPH331057",
			Port:          "low",
			PanelSetup:    "EcoFlow 220W Bifacial Portable",
			PanelCount:    1,
			NominalTotalW: 220,
		},
	}
}

func newPanelHintIndex() panelHintIndex {
	return panelHintIndex{
		bySNPort:      map[string]panelHint{},
		byProductPort: map[string]panelHint{},
	}
}

func (p *panelHintIndex) Upsert(hint panelHint) {
	hint.Port = normalizePort(hint.Port)
	if hint.Port == "" {
		hint.Port = "low"
	}
	if strings.TrimSpace(hint.DeviceSN) != "" {
		p.bySNPort[strings.ToLower(strings.TrimSpace(hint.DeviceSN))+"|"+hint.Port] = hint
	}
	if strings.TrimSpace(hint.ProductName) != "" {
		p.byProductPort[strings.ToLower(strings.TrimSpace(hint.ProductName))+"|"+hint.Port] = hint
	}
}

func (p panelHintIndex) Resolve(sn, productName, port string) (panelHint, bool) {
	port = normalizePort(port)
	if hint, ok := p.bySNPort[strings.ToLower(strings.TrimSpace(sn))+"|"+port]; ok {
		return hint, true
	}
	if hint, ok := p.byProductPort[strings.ToLower(strings.TrimSpace(productName))+"|"+port]; ok {
		return hint, true
	}
	return panelHint{}, false
}

func mapHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, col := range header {
		index[strings.TrimSpace(col)] = i
	}
	return index
}

func normalizePort(port string) string {
	switch strings.ToLower(strings.TrimSpace(port)) {
	case "high":
		return "high"
	default:
		return "low"
	}
}

func portKey(sn, name, port string) string {
	return strings.TrimSpace(sn) + "|" + strings.TrimSpace(name) + "|" + normalizePort(port)
}

func parseFloat(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func parseInt64(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	n := len(copyValues)
	mid := n / 2
	if n%2 == 0 {
		return (copyValues[mid-1] + copyValues[mid]) / 2.0
	}
	return copyValues[mid]
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maximum := values[0]
	for _, value := range values[1:] {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

func ratioPercent(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return (numerator / denominator) * 100.0
}

func floatFormat(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

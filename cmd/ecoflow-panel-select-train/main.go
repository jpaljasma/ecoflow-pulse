package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

const (
	defaultInputCSV       = "logs/telemetry_training.csv"
	defaultOutputModel    = "data/solar_panels/panel_select_model.json"
	defaultReplayEveryN   = 10
	defaultReplayMinCount = 20
)

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

type trainingSample struct {
	TSUnixMS    int64
	DeviceSN    string
	ProductName string
	Profile     string
	Port        string
	Watts       float64
	Volts       float64
	Amps        float64
	State       string
	Hint        panelHint
}

type classKey struct {
	Profile string
	Port    string
	Setup   string
}

type classAggregate struct {
	Key         classKey
	PanelCount  int
	NominalW    float64
	DeviceSNs   map[string]struct{}
	Tracker     *panelselect.Tracker
	SampleCount int
}

type replayStats struct {
	Rows              int
	Predictions       int
	Correct           int
	AvgConfidence     float64
	AvgCorrectConf    float64
	AvgWrongConf      float64
	ProfilePortCounts map[string]int
}

func main() {
	var (
		inputCSV     string
		outputModel  string
		panelMapPath string
		replay       bool
		replayEveryN int
		replayMin    int
	)

	flag.StringVar(&inputCSV, "csv", defaultInputCSV, "Path to telemetry training CSV")
	flag.StringVar(&outputModel, "out", defaultOutputModel, "Output model JSON path")
	flag.StringVar(&panelMapPath, "panel-map", "", "Optional JSON panel map for labels")
	flag.BoolVar(&replay, "replay", true, "Replay telemetry rows against trained model")
	flag.IntVar(&replayEveryN, "replay-every", defaultReplayEveryN, "Replay prediction cadence (rows)")
	flag.IntVar(&replayMin, "replay-min-samples", defaultReplayMinCount, "Minimum samples before replay predictions")
	flag.Parse()

	hints, err := loadPanelHints(panelMapPath)
	if err != nil {
		fatalf("load panel hints: %v", err)
	}

	samples, err := loadLabeledSamples(inputCSV, hints)
	if err != nil {
		fatalf("load labeled telemetry samples: %v", err)
	}
	if len(samples) == 0 {
		fatalf("no labeled telemetry samples found")
	}

	model := trainModel(samples, inputCSV)
	if err := model.Save(outputModel); err != nil {
		fatalf("save model: %v", err)
	}

	fmt.Printf("source: %s\n", inputCSV)
	fmt.Printf("samples: %d\n", len(samples))
	fmt.Printf("classes: %d\n", len(model.Classes))
	fmt.Printf("model: %s\n", outputModel)
	fmt.Println("classes:")
	for _, class := range model.Classes {
		fmt.Printf(
			"  - %s | profile=%s port=%s samples=%d panel_count=%d nominal=%.0fW\n",
			class.PanelSetup,
			class.Profile,
			class.Port,
			class.SampleCount,
			class.PanelCount,
			class.NominalTotalW,
		)
	}

	if replay {
		stats := replayModel(model, samples, replayEveryN, replayMin)
		fmt.Println()
		fmt.Printf("replay_rows: %d\n", stats.Rows)
		fmt.Printf("predictions: %d\n", stats.Predictions)
		if stats.Predictions > 0 {
			accuracy := float64(stats.Correct) / float64(stats.Predictions)
			fmt.Printf("accuracy: %.4f\n", accuracy)
			fmt.Printf("avg_confidence: %.4f\n", stats.AvgConfidence/float64(stats.Predictions))
			if stats.Correct > 0 {
				fmt.Printf("avg_confidence_correct: %.4f\n", stats.AvgCorrectConf/float64(stats.Correct))
			}
			wrong := stats.Predictions - stats.Correct
			if wrong > 0 {
				fmt.Printf("avg_confidence_wrong: %.4f\n", stats.AvgWrongConf/float64(wrong))
			}
		}
	}
}

func loadPanelHints(path string) (panelHintIndex, error) {
	index := panelHintIndex{
		bySNPort:      map[string]panelHint{},
		byProductPort: map[string]panelHint{},
	}
	for _, hint := range defaultPanelHints() {
		index.Upsert(hint)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return index, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return panelHintIndex{}, fmt.Errorf("read panel map: %w", err)
	}

	var hints []panelHint
	if err := json.Unmarshal(content, &hints); err != nil {
		var wrapped struct {
			Panels []panelHint `json:"panels"`
		}
		if err2 := json.Unmarshal(content, &wrapped); err2 != nil {
			return panelHintIndex{}, fmt.Errorf("parse panel map json: %w", err)
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

func (i *panelHintIndex) Upsert(h panelHint) {
	h.Port = panelselect.NormalizePort(h.Port)
	if sn := strings.TrimSpace(h.DeviceSN); sn != "" {
		i.bySNPort[strings.ToLower(sn)+"|"+h.Port] = h
	}
	if name := strings.TrimSpace(h.ProductName); name != "" {
		i.byProductPort[strings.ToLower(name)+"|"+h.Port] = h
	}
}

func (i panelHintIndex) Resolve(sn, productName, port string) (panelHint, bool) {
	port = panelselect.NormalizePort(port)
	if hint, ok := i.bySNPort[strings.ToLower(strings.TrimSpace(sn))+"|"+port]; ok {
		return hint, true
	}
	if hint, ok := i.byProductPort[strings.ToLower(strings.TrimSpace(productName))+"|"+port]; ok {
		return hint, true
	}
	return panelHint{}, false
}

func loadLabeledSamples(path string, hints panelHintIndex) ([]trainingSample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}
	header := mapHeaderIndex(rows[0])
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
			return nil, fmt.Errorf("training csv missing %q", key)
		}
	}

	get := func(row []string, key string) string {
		idx := header[key]
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	out := make([]trainingSample, 0, len(rows))
	for _, row := range rows[1:] {
		sn := get(row, "device_sn")
		product := get(row, "product_name")
		if sn == "" || product == "" {
			continue
		}
		ts := parseInt64(get(row, "ts_unix_ms"))
		profile := panelselect.NormalizeProfile(product)

		lowHint, lowOK := hints.Resolve(sn, product, "low")
		if lowOK && strings.TrimSpace(lowHint.PanelSetup) != "" {
			out = append(out, trainingSample{
				TSUnixMS:    ts,
				DeviceSN:    sn,
				ProductName: product,
				Profile:     profile,
				Port:        "low",
				Watts:       parseFloat(get(row, "solar_low_in_w")),
				Volts:       parseFloat(get(row, "solar_low_v")),
				Amps:        parseFloat(get(row, "solar_low_a")),
				State:       normalizeState(get(row, "mppt_low_state")),
				Hint:        lowHint,
			})
		}
		highHint, highOK := hints.Resolve(sn, product, "high")
		if highOK && strings.TrimSpace(highHint.PanelSetup) != "" {
			out = append(out, trainingSample{
				TSUnixMS:    ts,
				DeviceSN:    sn,
				ProductName: product,
				Profile:     profile,
				Port:        "high",
				Watts:       parseFloat(get(row, "solar_high_in_w")),
				Volts:       parseFloat(get(row, "solar_high_v")),
				Amps:        parseFloat(get(row, "solar_high_a")),
				State:       normalizeState(get(row, "mppt_high_state")),
				Hint:        highHint,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].TSUnixMS != out[j].TSUnixMS {
			return out[i].TSUnixMS < out[j].TSUnixMS
		}
		if out[i].DeviceSN != out[j].DeviceSN {
			return out[i].DeviceSN < out[j].DeviceSN
		}
		return out[i].Port < out[j].Port
	})
	return out, nil
}

func trainModel(samples []trainingSample, sourceCSV string) *panelselect.Model {
	model := panelselect.NewModel()
	model.SourceCSV = sourceCSV
	model.GeneratedAtUTC = time.Now().UTC().Format(time.RFC3339)

	classes := map[classKey]*classAggregate{}
	for _, sample := range samples {
		setup := strings.TrimSpace(sample.Hint.PanelSetup)
		if setup == "" {
			continue
		}
		key := classKey{
			Profile: strings.ToLower(strings.TrimSpace(sample.Profile)),
			Port:    panelselect.NormalizePort(sample.Port),
			Setup:   setup,
		}
		agg, ok := classes[key]
		if !ok {
			agg = &classAggregate{
				Key:        key,
				PanelCount: sample.Hint.PanelCount,
				NominalW:   sample.Hint.NominalTotalW,
				DeviceSNs:  map[string]struct{}{},
				Tracker:    panelselect.NewTracker(1000000),
			}
			classes[key] = agg
		}
		agg.Tracker.Observe(sample.Watts, sample.Volts, sample.Amps, sample.State)
		agg.SampleCount++
		if sn := strings.TrimSpace(sample.DeviceSN); sn != "" {
			agg.DeviceSNs[sn] = struct{}{}
		}
	}

	keys := make([]classKey, 0, len(classes))
	for key := range classes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Profile != keys[j].Profile {
			return keys[i].Profile < keys[j].Profile
		}
		if keys[i].Port != keys[j].Port {
			return keys[i].Port < keys[j].Port
		}
		return keys[i].Setup < keys[j].Setup
	})

	for _, key := range keys {
		agg := classes[key]
		centroid, ok := agg.Tracker.FeatureVector()
		if !ok {
			continue
		}
		model.Classes = append(model.Classes, panelselect.Class{
			ID:            classID(key.Profile, key.Port, key.Setup),
			Profile:       key.Profile,
			Port:          key.Port,
			PanelSetup:    key.Setup,
			PanelCount:    agg.PanelCount,
			NominalTotalW: agg.NominalW,
			SampleCount:   agg.SampleCount,
			DeviceSNs:     sortedSNs(agg.DeviceSNs),
			Centroid:      centroid,
		})
	}
	return model
}

func replayModel(model *panelselect.Model, samples []trainingSample, everyN, minSamples int) replayStats {
	if everyN <= 0 {
		everyN = 1
	}
	if minSamples <= 0 {
		minSamples = 1
	}
	trackers := map[string]*panelselect.Tracker{}
	stats := replayStats{
		ProfilePortCounts: map[string]int{},
	}

	for idx, sample := range samples {
		stats.Rows++
		key := sample.DeviceSN + "|" + panelselect.NormalizePort(sample.Port)
		tracker, ok := trackers[key]
		if !ok {
			tracker = panelselect.NewTracker(panelselect.DefaultTrackerLimit)
			trackers[key] = tracker
		}
		tracker.Observe(sample.Watts, sample.Volts, sample.Amps, sample.State)
		if tracker.SampleCount() < minSamples {
			continue
		}
		if idx%everyN != 0 {
			continue
		}
		features, ok := tracker.FeatureVector()
		if !ok {
			continue
		}
		prediction, ok := panelselect.Predict(model, sample.Profile, sample.Port, features, tracker.SampleCount())
		if !ok {
			continue
		}
		stats.Predictions++
		stats.AvgConfidence += prediction.Confidence
		stats.ProfilePortCounts[sample.Profile+"|"+sample.Port]++

		if strings.EqualFold(strings.TrimSpace(prediction.PanelSetup), strings.TrimSpace(sample.Hint.PanelSetup)) {
			stats.Correct++
			stats.AvgCorrectConf += prediction.Confidence
		} else {
			stats.AvgWrongConf += prediction.Confidence
		}
	}
	return stats
}

func sortedSNs(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for sn := range set {
		out = append(out, sn)
	}
	sort.Strings(out)
	return out
}

func classID(profile, port, setup string) string {
	raw := strings.ToLower(strings.TrimSpace(profile + "_" + port + "_" + setup))
	raw = strings.NewReplacer(" ", "_", "/", "_", "\\", "_", "-", "_", "(", "_", ")", "_", ",", "_").Replace(raw)
	for strings.Contains(raw, "__") {
		raw = strings.ReplaceAll(raw, "__", "_")
	}
	return strings.Trim(raw, "_")
}

func normalizeState(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "charging", "active":
		return "charging"
	case "idle", "locked":
		return "idle"
	default:
		return raw
	}
}

func mapHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, col := range header {
		index[strings.TrimSpace(col)] = i
	}
	return index
}

func parseFloat(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
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

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

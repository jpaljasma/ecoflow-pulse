package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
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
	defaultPanelIndexJSON = "data/solar_panels/solar_panel_specs_v13.index.json"
	defaultReplayEveryN   = 10
	defaultReplayMinCount = 20
)

type panelHint struct {
	DeviceSN      string  `json:"device_sn"`
	ProductName   string  `json:"product_name"`
	Profile       string  `json:"profile"`
	Port          string  `json:"port"`
	PanelSetup    string  `json:"panel_setup"`
	PanelCount    int     `json:"panel_count"`
	NominalTotalW float64 `json:"nominal_total_w"`
}

type panelHintIndex struct {
	bySNPort      map[string]panelHint
	byProductPort map[string]panelHint
	byProfilePort map[string]panelHint
}

type sampleKey struct {
	DeviceSN string
	Profile  string
	Port     string
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

type rawSample struct {
	ts      int64
	sn      string
	product string
	profile string
	port    string
	watts   float64
	volts   float64
	amps    float64
	state   string
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

type sampleAgg struct {
	sn      string
	product string
	profile string
	port    string
	watts   []float64
	volts   []float64
	amps    []float64
	activeN int
}

type pvPortCaps struct {
	minVolts float64
	maxVolts float64
	maxAmps  float64
	maxWatts float64
}

type panelIndex struct {
	ByPanelKey  map[string]panelIndexEntry `json:"by_panel_key"`
	ByDeviceTag map[string][]string        `json:"by_device_tag"`
}

type panelIndexEntry struct {
	ID       string                             `json:"id"`
	Brand    string                             `json:"brand"`
	Model    string                             `json:"model"`
	Type     string                             `json:"type,omitempty"`
	PmaxSTCW float64                            `json:"pmax_stc_w"`
	VocV     float64                            `json:"voc_v"`
	VmpV     float64                            `json:"vmp_v"`
	ImpA     float64                            `json:"imp_a"`
	IscA     float64                            `json:"isc_a"`
	Compat   map[string]panelCompatibilityEntry `json:"compatibility"`
}

type panelCompatibilityEntry struct {
	Status string `json:"status"`
}

func main() {
	var (
		inputCSV     string
		outputModel  string
		panelMapPath string
		panelDBPath  string
		replay       bool
		replayEveryN int
		replayMin    int
	)

	flag.StringVar(&inputCSV, "csv", defaultInputCSV, "Path to telemetry training CSV")
	flag.StringVar(&outputModel, "out", defaultOutputModel, "Output model JSON path")
	flag.StringVar(&panelMapPath, "panel-map", "", "Optional JSON panel map for labels")
	flag.StringVar(&panelDBPath, "panel-db", defaultPanelIndexJSON, "Panel DB compact index JSON for synthetic class augmentation")
	flag.BoolVar(&replay, "replay", true, "Replay telemetry rows against trained model")
	flag.IntVar(&replayEveryN, "replay-every", defaultReplayEveryN, "Replay prediction cadence (rows)")
	flag.IntVar(&replayMin, "replay-min-samples", defaultReplayMinCount, "Minimum samples before replay predictions")
	flag.Parse()

	hints, err := loadPanelHints(panelMapPath)
	if err != nil {
		fatalf("load panel hints: %v", err)
	}
	panelDB, err := loadPanelIndex(panelDBPath)
	if err != nil {
		fatalf("load panel db index: %v", err)
	}

	samples, err := loadLabeledSamples(inputCSV, hints, panelDB)
	if err != nil {
		fatalf("load labeled telemetry samples: %v", err)
	}
	if len(samples) == 0 {
		fatalf("no labeled telemetry samples found")
	}

	model := trainModel(samples, inputCSV)
	augmentModelWithPanelDBClasses(model, panelDB)
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
		byProfilePort: map[string]panelHint{},
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
	return nil
}

func (i *panelHintIndex) Upsert(h panelHint) {
	h.Port = panelselect.NormalizePort(h.Port)
	h.Profile = strings.ToLower(strings.TrimSpace(h.Profile))
	if sn := strings.TrimSpace(h.DeviceSN); sn != "" {
		i.bySNPort[strings.ToLower(sn)+"|"+h.Port] = h
	}
	if name := strings.TrimSpace(h.ProductName); name != "" {
		i.byProductPort[strings.ToLower(name)+"|"+h.Port] = h
	}
	if h.Profile != "" {
		i.byProfilePort[h.Profile+"|"+h.Port] = h
	}
}

func (i panelHintIndex) Resolve(sn, productName, profile, port string) (panelHint, bool) {
	port = panelselect.NormalizePort(port)
	if hint, ok := i.bySNPort[strings.ToLower(strings.TrimSpace(sn))+"|"+port]; ok {
		return hint, true
	}
	if hint, ok := i.byProductPort[strings.ToLower(strings.TrimSpace(productName))+"|"+port]; ok {
		return hint, true
	}
	if hint, ok := i.byProfilePort[strings.ToLower(strings.TrimSpace(profile))+"|"+port]; ok {
		return hint, true
	}
	return panelHint{}, false
}

func loadLabeledSamples(path string, hints panelHintIndex, panelDB *panelIndex) ([]trainingSample, error) {
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

	raw := make([]rawSample, 0, len(rows)*2)
	for _, row := range rows[1:] {
		sn := get(row, "device_sn")
		product := get(row, "product_name")
		if sn == "" || product == "" {
			continue
		}
		ts := parseInt64(get(row, "ts_unix_ms"))
		profile := panelselect.NormalizeProfile(product)

		raw = append(raw, rawSample{
			ts:      ts,
			sn:      sn,
			product: product,
			profile: profile,
			port:    "low",
			watts:   parseFloat(get(row, "solar_low_in_w")),
			volts:   parseFloat(get(row, "solar_low_v")),
			amps:    parseFloat(get(row, "solar_low_a")),
			state:   normalizeState(get(row, "mppt_low_state")),
		})
		raw = append(raw, rawSample{
			ts:      ts,
			sn:      sn,
			product: product,
			profile: profile,
			port:    "high",
			watts:   parseFloat(get(row, "solar_high_in_w")),
			volts:   parseFloat(get(row, "solar_high_v")),
			amps:    parseFloat(get(row, "solar_high_a")),
			state:   normalizeState(get(row, "mppt_high_state")),
		})
	}

	autoHints := inferHintsFromData(raw, panelDB)
	out := make([]trainingSample, 0, len(raw))
	for _, sample := range raw {
		if strings.TrimSpace(sample.sn) == "" || strings.TrimSpace(sample.product) == "" {
			continue
		}
		hint, ok := hints.Resolve(sample.sn, sample.product, sample.profile, sample.port)
		if !ok || strings.TrimSpace(hint.PanelSetup) == "" {
			autoHint, hasAuto := autoHints[sampleKey{
				DeviceSN: strings.ToLower(strings.TrimSpace(sample.sn)),
				Profile:  strings.ToLower(strings.TrimSpace(sample.profile)),
				Port:     panelselect.NormalizePort(sample.port),
			}]
			if hasAuto {
				hint = autoHint
				ok = true
			}
		}
		if !ok || strings.TrimSpace(hint.PanelSetup) == "" {
			continue
		}
		out = append(out, trainingSample{
			TSUnixMS:    sample.ts,
			DeviceSN:    sample.sn,
			ProductName: sample.product,
			Profile:     sample.profile,
			Port:        sample.port,
			Watts:       sample.watts,
			Volts:       sample.volts,
			Amps:        sample.amps,
			State:       sample.state,
			Hint:        hint,
		})
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

func inferHintsFromData(samples []rawSample, db *panelIndex) map[sampleKey]panelHint {
	out := map[sampleKey]panelHint{}
	if db == nil || len(db.ByPanelKey) == 0 {
		return out
	}
	aggs := map[sampleKey]*sampleAgg{}
	for _, s := range samples {
		key := sampleKey{
			DeviceSN: strings.ToLower(strings.TrimSpace(s.sn)),
			Profile:  strings.ToLower(strings.TrimSpace(s.profile)),
			Port:     panelselect.NormalizePort(s.port),
		}
		item, ok := aggs[key]
		if !ok {
			item = &sampleAgg{
				sn:      s.sn,
				product: s.product,
				profile: s.profile,
				port:    panelselect.NormalizePort(s.port),
				watts:   make([]float64, 0, 1024),
				volts:   make([]float64, 0, 1024),
				amps:    make([]float64, 0, 1024),
			}
			aggs[key] = item
		}
		w := math.Abs(s.watts)
		if w > 0 {
			item.watts = append(item.watts, w)
		}
		if s.volts > 0 {
			item.volts = append(item.volts, s.volts)
		}
		if s.amps > 0 {
			item.amps = append(item.amps, s.amps)
		}
		if strings.EqualFold(s.state, "charging") || strings.EqualFold(s.state, "active") {
			item.activeN++
		}
	}

	for key, item := range aggs {
		if len(item.watts) == 0 {
			continue
		}
		candidates := panelCandidatesForProfilePort(db, item.profile, item.port)
		if len(candidates) == 0 {
			continue
		}
		caps, _ := inferPortCaps(item.profile, item.port)
		bestRec, bestCount, ok := inferBestPanelAndCount(candidates, caps, item)
		if !ok || bestRec.PmaxSTCW <= 0 || bestCount <= 0 {
			continue
		}
		setup := inferredPanelSetupLabel(bestRec, bestCount)
		out[key] = panelHint{
			DeviceSN:      item.sn,
			ProductName:   item.product,
			Port:          item.port,
			PanelSetup:    setup,
			PanelCount:    bestCount,
			NominalTotalW: float64(bestCount) * bestRec.PmaxSTCW,
		}
	}
	return out
}

func panelCandidatesForProfilePort(db *panelIndex, profile, port string) []panelIndexEntry {
	profile = strings.ToLower(strings.TrimSpace(profile))
	port = panelselect.NormalizePort(port)
	var tags []string
	switch profile {
	case "d2m":
		tags = []string{"d2_d2_max"}
	case "dpu":
		if port == "high" {
			tags = []string{"dpu_high", "dpu_x_high"}
		} else {
			tags = []string{"dpu_low"}
		}
	}
	if len(tags) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]panelIndexEntry, 0, 32)
	for _, tag := range tags {
		for _, panelKey := range db.ByDeviceTag[strings.ToLower(strings.TrimSpace(tag))] {
			if _, ok := seen[panelKey]; ok {
				continue
			}
			rec, ok := db.ByPanelKey[panelKey]
			if !ok {
				continue
			}
			if !isCompatible(rec, tag) || rec.PmaxSTCW <= 0 {
				continue
			}
			seen[panelKey] = struct{}{}
			out = append(out, rec)
		}
	}
	return out
}

func inferPortCaps(profile, port string) (pvPortCaps, bool) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	port = panelselect.NormalizePort(port)
	switch profile {
	case "d2m":
		return pvPortCaps{minVolts: 11, maxVolts: 60, maxAmps: 15, maxWatts: 500}, true
	case "dpu":
		if port == "high" {
			return pvPortCaps{minVolts: 80, maxVolts: 450, maxAmps: 15, maxWatts: 4000}, true
		}
		return pvPortCaps{minVolts: 30, maxVolts: 150, maxAmps: 15, maxWatts: 1600}, true
	default:
		return pvPortCaps{}, false
	}
}

func inferBestPanelAndCount(candidates []panelIndexEntry, caps pvPortCaps, obs *sampleAgg) (panelIndexEntry, int, bool) {
	if obs == nil {
		return panelIndexEntry{}, 0, false
	}
	maxWatts := maxValue(obs.watts)
	p95Watts := percentile(obs.watts, 0.95)
	maxVolts := maxValue(obs.volts)
	p95Volts := percentile(obs.volts, 0.95)
	maxAmps := maxValue(obs.amps)
	p95Amps := percentile(obs.amps, 0.95)
	if maxWatts <= 0 || p95Volts <= 0 {
		return panelIndexEntry{}, 0, false
	}
	if p95Amps <= 0 && maxAmps > 0 {
		p95Amps = maxAmps
	}
	// Irradiance signal factor from port utilization.
	sunSignal := 0.0
	if caps.maxWatts > 0 {
		sunSignal = math.Min(1, math.Max(0, p95Watts/math.Max(caps.maxWatts, 1)))
	}
	// On weak-sun windows, watts are less discriminative than volts/amps.
	wattsWeight := 0.18 + 0.22*math.Sqrt(sunSignal)
	voltsWeight := 0.34
	ampsWeight := 0.24
	irrWeight := 0.10
	peakWeight := 0.14
	peakAWWeight := 0.10

	bestScore := math.MaxFloat64
	bestCount := 0
	best := panelIndexEntry{}
	for _, rec := range candidates {
		if rec.PmaxSTCW <= 0 {
			continue
		}
		refV := rec.VmpV
		if refV <= 0 {
			refV = rec.VocV * 0.82
		}
		refA := rec.ImpA
		if refA <= 0 && refV > 0 {
			refA = rec.PmaxSTCW / refV
		}
		if refV <= 0 || refA <= 0 {
			continue
		}

		peakAllowance := 1.10
		if panelIsBifacial(rec) {
			peakAllowance = 1.25
		}
		// Lower bound from observed peak envelope; avoids tiny-panel collapse at medium sun.
		minCount := int(math.Ceil(maxWatts / math.Max(rec.PmaxSTCW*peakAllowance, 1)))
		if minCount < 1 {
			minCount = 1
		}
		maxCount := 24
		if caps.maxWatts > 0 {
			// Keep search bounded to realistic MPPT utilization.
			maxCount = int(math.Ceil((caps.maxWatts * 1.25) / rec.PmaxSTCW))
			if maxCount < 1 {
				maxCount = 1
			}
			if maxCount > 24 {
				maxCount = 24
			}
		}
		if minCount > maxCount {
			continue
		}
		for count := minCount; count <= maxCount; count++ {
			for series := 1; series <= count; series++ {
				if count%series != 0 {
					continue
				}
				parallel := count / series
				projectedW := float64(count) * rec.PmaxSTCW
				projectedV := float64(series) * refV
				projectedA := float64(parallel) * refA

				// MPPT safety/fit constraints for candidate layout.
				if caps.maxVolts > 0 {
					voc := rec.VocV
					if voc <= 0 {
						voc = refV * 1.20
					}
					coldVoc := float64(series) * voc * 1.10
					if coldVoc > caps.maxVolts*1.02 {
						continue
					}
				}
				if caps.maxAmps > 0 {
					isc := rec.IscA
					if isc <= 0 {
						isc = refA * 1.08
					}
					if float64(parallel)*isc > caps.maxAmps*1.05 {
						continue
					}
				}
				if caps.minVolts > 0 && projectedV < caps.minVolts*0.85 {
					continue
				}

				// Irradiance-consistent fitting on high-signal envelope.
				irrW := p95Watts / math.Max(projectedW, 1)
				irrA := p95Amps / math.Max(projectedA, 1)
				irr := (0.55 * irrW) + (0.45 * irrA)
				if irr <= 0 {
					continue
				}
				if irr > 1.35 {
					continue
				}
				irrPredPeak := maxWatts / math.Max(projectedW, 1)
				peakIrrCap := 1.15
				if panelIsBifacial(rec) {
					peakIrrCap = 1.30
				}
				if irrPredPeak > peakIrrCap {
					continue
				}
				// Shoulder-hours curve: boost modeled low-irradiance productivity modestly.
				irrCurve := irradianceCurveFactor(irr)
				vScale := 0.94 + (0.06 * math.Min(1.1, math.Max(0.2, irrCurve)))
				wPred := projectedW * irrCurve
				aPred := projectedA * irr
				vPred := projectedV * vScale

				errV := math.Abs(vPred-p95Volts) / math.Max(p95Volts, 1)
				errA := math.Abs(aPred-p95Amps) / math.Max(p95Amps, 1)
				errW := math.Abs(wPred-p95Watts) / math.Max(p95Watts, 1)
				errPeakW := 0.0
				if maxWatts > 0 {
					peakCap := projectedW * peakAllowance
					if maxWatts > peakCap {
						errPeakW = (maxWatts - peakCap) / math.Max(maxWatts, 1)
					}
				}
				errPeakA := 0.0
				if maxAmps > 0 {
					peakCapA := projectedA * peakAllowance
					if maxAmps > peakCapA {
						errPeakA = (maxAmps - peakCapA) / math.Max(maxAmps, 1)
					}
				}
				errPeakV := 0.0
				if maxVolts > 0 {
					// PV voltage varies less than current; keep this a soft penalty.
					peakCapV := projectedV * 1.12
					if maxVolts > peakCapV {
						errPeakV = (maxVolts - peakCapV) / math.Max(maxVolts, 1)
					}
				}
				irrConsistency := math.Abs(irrW - irrA)
				score := (wattsWeight * errW) +
					(voltsWeight * errV) +
					(ampsWeight * errA) +
					(irrWeight * irrConsistency) +
					(peakWeight * (errPeakW + errPeakV)) +
					(peakAWWeight * errPeakA)
				if caps.maxWatts > 0 && projectedW > caps.maxWatts {
					// Favor layouts that naturally fit MPPT watts cap over always-clipped alternatives.
					clipOver := (projectedW - caps.maxWatts) / math.Max(caps.maxWatts, 1)
					score += 0.80 * clipOver
				}
				score += 0.004 * float64(count)
				if series > 1 && parallel > 1 {
					score += 0.01
				}
				if score < bestScore {
					bestScore = score
					best = rec
					bestCount = count
				}
			}
		}
	}
	if bestCount <= 0 || best.PmaxSTCW <= 0 {
		return panelIndexEntry{}, 0, false
	}
	return best, bestCount, true
}

func irradianceCurveFactor(irr float64) float64 {
	if irr <= 0 {
		return 0
	}
	if irr >= 1 {
		return math.Min(1.10, irr)
	}
	// Slight shoulder-hours uplift to avoid low-sun underfitting.
	linear := irr
	shoulder := math.Sqrt(irr)
	return (0.72 * linear) + (0.28 * shoulder)
}

func panelIsBifacial(rec panelIndexEntry) bool {
	corpus := strings.ToLower(strings.TrimSpace(rec.Type + " " + rec.Model + " " + rec.Brand))
	return strings.Contains(corpus, "bifacial") || strings.Contains(corpus, "bi-facial")
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return minValue(values)
	}
	if q >= 1 {
		return maxValue(values)
	}
	clone := append([]float64(nil), values...)
	sort.Float64s(clone)
	pos := q * float64(len(clone)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return clone[lo]
	}
	frac := pos - float64(lo)
	return clone[lo]*(1-frac) + clone[hi]*frac
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > max {
			max = values[i]
		}
	}
	return max
}

func minValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] < min {
			min = values[i]
		}
	}
	return min
}

func inferredPanelSetupLabel(rec panelIndexEntry, count int) string {
	label := strings.TrimSpace(strings.TrimSpace(rec.Brand + " " + rec.Model))
	if label == "" {
		label = strings.TrimSpace(rec.ID)
	}
	if count > 1 {
		return fmt.Sprintf("%dx%.0fW %s", count, rec.PmaxSTCW, label)
	}
	return label
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

func loadPanelIndex(path string) (*panelIndex, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &panelIndex{}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read panel db index: %w", err)
	}
	var db panelIndex
	if err := json.Unmarshal(content, &db); err != nil {
		return nil, fmt.Errorf("parse panel db index json: %w", err)
	}
	if db.ByPanelKey == nil {
		db.ByPanelKey = map[string]panelIndexEntry{}
	}
	if db.ByDeviceTag == nil {
		db.ByDeviceTag = map[string][]string{}
	}
	return &db, nil
}

func augmentModelWithPanelDBClasses(model *panelselect.Model, db *panelIndex) {
	if model == nil || db == nil {
		return
	}
	known := make(map[string]struct{}, len(model.Classes))
	for _, class := range model.Classes {
		key := strings.ToLower(strings.TrimSpace(class.Profile + "|" + panelselect.NormalizePort(class.Port) + "|" + class.PanelSetup))
		if key != "" {
			known[key] = struct{}{}
		}
	}

	addFromTag := func(profile, port, tag string) {
		panelKeys := db.ByDeviceTag[strings.ToLower(strings.TrimSpace(tag))]
		for _, panelKey := range panelKeys {
			record, ok := db.ByPanelKey[panelKey]
			if !ok {
				continue
			}
			if !isCompatible(record, tag) {
				continue
			}
			panelW := record.PmaxSTCW
			if panelW <= 0 {
				continue
			}
			panelV := record.VmpV
			if panelV <= 0 {
				panelV = record.VocV * 0.82
			}
			if panelV <= 0 {
				continue
			}
			panelA := record.ImpA
			if panelA <= 0 {
				panelA = panelW / panelV
			}
			if panelA <= 0 {
				continue
			}
			setup := strings.TrimSpace(strings.TrimSpace(record.Brand + " " + record.Model))
			if setup == "" {
				continue
			}
			classKeyNorm := strings.ToLower(strings.TrimSpace(profile + "|" + port + "|" + setup))
			if _, exists := known[classKeyNorm]; exists {
				continue
			}

			centroid := syntheticCentroid(panelW, panelV, panelA)
			model.Classes = append(model.Classes, panelselect.Class{
				ID:            classID(profile, port, setup),
				Profile:       profile,
				Port:          port,
				PanelSetup:    setup,
				PanelCount:    1,
				NominalTotalW: panelW,
				SampleCount:   1,
				Synthetic:     true,
				DeviceSNs:     nil,
				Centroid:      centroid,
			})
			known[classKeyNorm] = struct{}{}
		}
	}

	// D2M supports the same panel set on low/high MPPT ports.
	addFromTag("d2m", "low", "d2_d2_max")
	addFromTag("d2m", "high", "d2_d2_max")
	// DPU has distinct low/high capabilities.
	addFromTag("dpu", "low", "dpu_low")
	addFromTag("dpu", "high", "dpu_high")
	addFromTag("dpu", "high", "dpu_x_high")

	sort.Slice(model.Classes, func(i, j int) bool {
		if model.Classes[i].Profile != model.Classes[j].Profile {
			return model.Classes[i].Profile < model.Classes[j].Profile
		}
		if panelselect.NormalizePort(model.Classes[i].Port) != panelselect.NormalizePort(model.Classes[j].Port) {
			return panelselect.NormalizePort(model.Classes[i].Port) < panelselect.NormalizePort(model.Classes[j].Port)
		}
		return model.Classes[i].PanelSetup < model.Classes[j].PanelSetup
	})
}

func isCompatible(record panelIndexEntry, tag string) bool {
	if record.Compat == nil {
		return true
	}
	compat, ok := record.Compat[strings.ToLower(strings.TrimSpace(tag))]
	if !ok {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(compat.Status))
	if status == "" {
		return true
	}
	return status != "no"
}

func syntheticCentroid(panelW, panelV, panelA float64) []float64 {
	// Synthetic class baseline tuned for daytime active PV behavior.
	medianW := panelW * 0.78
	p95W := panelW * 0.98
	medianV := panelV * 0.98
	p95V := panelV * 1.03
	medianA := panelA * 0.74
	activeRatio := 0.72
	chargingRatio := 0.68

	centroid := []float64{
		round2(medianW),
		round2(p95W),
		round2(medianV),
		round2(p95V),
		round2(medianA),
		round2(activeRatio),
		round2(chargingRatio),
	}
	return centroid
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
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

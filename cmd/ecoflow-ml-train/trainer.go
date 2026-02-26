package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultMaxChargeSOC     = 95.0
	defaultMinDischargeSOC  = 5.0
	minTrainingPowerWatts   = 1.0
	systemNetThresholdWatts = 5.0
)

type mlProfileName string

const (
	profileD2M     mlProfileName = "d2m"
	profileDPU     mlProfileName = "dpu"
	profileGeneric mlProfileName = "generic"
)

type profileParams struct {
	fastWindow        int
	stableWindow      int
	recentWindow      int
	mediumWindow      int
	trendWindow       int
	latestWeight      float64
	recentWeight      float64
	mediumWeight      float64
	trendWeight       float64
	netThresholdScale float64
}

type optimizerOptions struct {
	candidateCount int
	seed           int64
	stageFractions []float64
}

type optimizerResult struct {
	Profile        mlProfileName
	Rows           int
	SeriesCount    int
	SampleCount    int
	EstimatedCapWh float64
	Best           profileCandidate
	StageStats     []stageStat
}

type stageStat struct {
	Fraction       float64
	SampleCount    int
	CandidateCount int
	RetainedCount  int
	BestStageScore float64
	BestStageCover float64
	UsedStratified bool
}

type profileCandidate struct {
	Params   profileParams
	Score    float64
	Coverage float64
}

type sampleMode int

const (
	modeUnknown sampleMode = iota
	modeCharge
	modeDischarge
)

func (m sampleMode) String() string {
	switch m {
	case modeCharge:
		return "charge"
	case modeDischarge:
		return "discharge"
	default:
		return "unknown"
	}
}

type trainingRecord struct {
	TSUnixMS int64
	DeviceSN string
	Product  string
	Profile  mlProfileName
	Mode     sampleMode
	HasETA   bool
	ETAMin   float64
	HasSOC   bool
	SOCPct   float64
	HasNet   bool
	NetW     float64
}

type trainingSample struct {
	Series  *deviceSeries
	Index   int
	Mode    sampleMode
	ETAMin  float64
	SOCPct  float64
	Profile mlProfileName
	Stratum string
}

type deviceSeries struct {
	DeviceSN string
	Profile  mlProfileName
	Values   []float64

	prefixSum    []float64
	prefixAbsSum []float64

	meanCache    map[int][]float64
	absMeanCache map[int][]float64
}

func loadTrainingRecords(path string) ([]trainingRecord, error) {
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
	if len(rows) <= 1 {
		return nil, errors.New("training csv has no data rows")
	}

	header := make(map[string]int, len(rows[0]))
	for i, col := range rows[0] {
		header[strings.TrimSpace(col)] = i
	}
	required := []string{
		"ts_unix_ms",
		"device_sn",
		"product_name",
		"system_state",
		"estimate_mode",
		"estimate_eta_min",
		"soc_pct",
		"battery_net_w",
		"battery_in_w",
		"battery_out_w",
		"ac_in_w",
		"solar_in_w",
		"ac_out_w",
		"dc_out_w",
	}
	for _, key := range required {
		if _, ok := header[key]; !ok {
			return nil, fmt.Errorf("training csv missing required column %q", key)
		}
	}

	records := make([]trainingRecord, 0, len(rows)-1)
	for _, row := range rows[1:] {
		rec, ok := parseTrainingRecordRow(header, row)
		if !ok {
			continue
		}
		records = append(records, rec)
	}
	if len(records) == 0 {
		return nil, errors.New("no usable training rows")
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].DeviceSN == records[j].DeviceSN {
			return records[i].TSUnixMS < records[j].TSUnixMS
		}
		return records[i].DeviceSN < records[j].DeviceSN
	})
	return records, nil
}

func parseTrainingRecordRow(header map[string]int, row []string) (trainingRecord, bool) {
	get := func(key string) string {
		idx := header[key]
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	ts, ok := parseInt64(get("ts_unix_ms"))
	if !ok {
		return trainingRecord{}, false
	}
	deviceSN := get("device_sn")
	if deviceSN == "" {
		return trainingRecord{}, false
	}
	product := get("product_name")
	profile := detectProfileFromProduct(product)

	mode := normalizeMode(get("estimate_mode"), get("system_state"))
	eta, hasETA := parseFloat(get("estimate_eta_min"))
	soc, hasSOC := parseFloat(get("soc_pct"))

	netW, hasNet := parseFloat(get("battery_net_w"))
	if !hasNet {
		bIn, hasBIn := parseFloat(get("battery_in_w"))
		bOut, hasBOut := parseFloat(get("battery_out_w"))
		if hasBIn || hasBOut {
			netW = bIn - bOut
			hasNet = true
		}
	}
	if !hasNet {
		acIn, hasACIn := parseFloat(get("ac_in_w"))
		solarIn, hasSolarIn := parseFloat(get("solar_in_w"))
		acOut, hasACOut := parseFloat(get("ac_out_w"))
		dcOut, hasDCOut := parseFloat(get("dc_out_w"))
		if hasACIn || hasSolarIn || hasACOut || hasDCOut {
			netW = (acIn + solarIn) - (acOut + dcOut)
			hasNet = true
		}
	}
	if !hasNet || math.IsNaN(netW) || math.IsInf(netW, 0) {
		return trainingRecord{}, false
	}

	rec := trainingRecord{
		TSUnixMS: ts,
		DeviceSN: deviceSN,
		Product:  product,
		Profile:  profile,
		Mode:     mode,
		HasETA:   hasETA && eta > 0,
		ETAMin:   eta,
		HasSOC:   hasSOC,
		SOCPct:   soc,
		HasNet:   true,
		NetW:     netW,
	}
	return rec, true
}

func detectProfileFromProduct(productName string) mlProfileName {
	productName = strings.ToLower(strings.TrimSpace(productName))
	switch {
	case strings.Contains(productName, "delta 2 max"):
		return profileD2M
	case strings.Contains(productName, "delta pro ultra"):
		return profileDPU
	default:
		return profileGeneric
	}
}

func normalizeMode(estimateMode string, systemState string) sampleMode {
	raw := strings.ToLower(strings.TrimSpace(estimateMode))
	switch raw {
	case "charge", "charging":
		return modeCharge
	case "discharge", "discharging":
		return modeDischarge
	case "idle":
		return modeUnknown
	}
	raw = strings.ToLower(strings.TrimSpace(systemState))
	switch raw {
	case "charge", "charging":
		return modeCharge
	case "discharge", "discharging":
		return modeDischarge
	default:
		return modeUnknown
	}
}

func parseFloat(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "n/a") {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func parseInt64(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func buildTrainingSet(records []trainingRecord, target mlProfileName) ([]*deviceSeries, []trainingSample) {
	byDevice := make(map[string][]trainingRecord)
	for _, rec := range records {
		if target != profileGeneric && rec.Profile != target {
			continue
		}
		byDevice[rec.DeviceSN] = append(byDevice[rec.DeviceSN], rec)
	}

	seriesList := make([]*deviceSeries, 0, len(byDevice))
	samples := make([]trainingSample, 0, len(records)/2)

	for deviceSN, deviceRows := range byDevice {
		sort.Slice(deviceRows, func(i, j int) bool {
			return deviceRows[i].TSUnixMS < deviceRows[j].TSUnixMS
		})
		if len(deviceRows) < 8 {
			continue
		}

		series := &deviceSeries{
			DeviceSN:     deviceSN,
			Profile:      resolveSeriesProfile(deviceRows),
			Values:       make([]float64, 0, len(deviceRows)),
			meanCache:    make(map[int][]float64),
			absMeanCache: make(map[int][]float64),
		}

		prevTrainingMode := modeUnknown
		prevTrainingIdx := -1

		for _, rec := range deviceRows {
			series.Values = append(series.Values, rec.NetW)
			idx := len(series.Values) - 1

			if !rec.HasETA || !rec.HasSOC || rec.Mode == modeUnknown {
				continue
			}

			isTransition := false
			if prevTrainingIdx >= 0 {
				if prevTrainingMode != rec.Mode {
					isTransition = true
				}
				prevNet := series.Values[prevTrainingIdx]
				delta := math.Abs(rec.NetW - prevNet)
				if delta > math.Max(20.0, math.Abs(prevNet)*0.35) {
					isTransition = true
				}
			}
			prevTrainingMode = rec.Mode
			prevTrainingIdx = idx

			stratum := fmt.Sprintf("%s|%s|transition=%t", rec.Profile, rec.Mode.String(), isTransition)
			if target == profileGeneric {
				stratum = fmt.Sprintf("%s|%s|transition=%t", rec.Profile, rec.Mode.String(), isTransition)
			}
			samples = append(samples, trainingSample{
				Series:  series,
				Index:   idx,
				Mode:    rec.Mode,
				ETAMin:  rec.ETAMin,
				SOCPct:  rec.SOCPct,
				Profile: rec.Profile,
				Stratum: stratum,
			})
		}
		series.buildPrefix()
		if len(series.Values) >= 8 {
			seriesList = append(seriesList, series)
		}
	}

	filtered := samples[:0]
	for _, sample := range samples {
		if sample.Series == nil || sample.Index < 1 {
			continue
		}
		filtered = append(filtered, sample)
	}
	return seriesList, filtered
}

func resolveSeriesProfile(rows []trainingRecord) mlProfileName {
	for _, rec := range rows {
		if rec.Profile == profileD2M {
			return profileD2M
		}
		if rec.Profile == profileDPU {
			return profileDPU
		}
	}
	return profileGeneric
}

func (s *deviceSeries) buildPrefix() {
	n := len(s.Values)
	s.prefixSum = make([]float64, n+1)
	s.prefixAbsSum = make([]float64, n+1)
	for i, v := range s.Values {
		s.prefixSum[i+1] = s.prefixSum[i] + v
		s.prefixAbsSum[i+1] = s.prefixAbsSum[i] + math.Abs(v)
	}
}

func (s *deviceSeries) meanAt(window int, index int) float64 {
	if s == nil || len(s.Values) == 0 {
		return 0
	}
	window = clampInt(window, 1, len(s.Values))
	if index < 0 {
		index = 0
	}
	if index >= len(s.Values) {
		index = len(s.Values) - 1
	}
	series, ok := s.meanCache[window]
	if !ok {
		series = make([]float64, len(s.Values))
		for i := range s.Values {
			start := i - window + 1
			if start < 0 {
				start = 0
			}
			count := i - start + 1
			sum := s.prefixSum[i+1] - s.prefixSum[start]
			series[i] = sum / float64(count)
		}
		s.meanCache[window] = series
	}
	return series[index]
}

func (s *deviceSeries) absMeanAt(window int, index int) float64 {
	if s == nil || len(s.Values) == 0 {
		return 0
	}
	window = clampInt(window, 1, len(s.Values))
	if index < 0 {
		index = 0
	}
	if index >= len(s.Values) {
		index = len(s.Values) - 1
	}
	series, ok := s.absMeanCache[window]
	if !ok {
		series = make([]float64, len(s.Values))
		for i := range s.Values {
			start := i - window + 1
			if start < 0 {
				start = 0
			}
			count := i - start + 1
			sum := s.prefixAbsSum[i+1] - s.prefixAbsSum[start]
			series[i] = sum / float64(count)
		}
		s.absMeanCache[window] = series
	}
	return series[index]
}

func clampInt(v int, min int, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func estimateCapacityWhByProfile(samples []trainingSample) map[mlProfileName]float64 {
	byProfile := map[mlProfileName][]float64{
		profileD2M:     {},
		profileDPU:     {},
		profileGeneric: {},
	}
	for _, sample := range samples {
		denom := defaultMaxChargeSOC - sample.SOCPct
		if sample.Mode == modeDischarge {
			denom = sample.SOCPct - defaultMinDischargeSOC
		}
		if denom <= 1 {
			continue
		}
		observedPower := math.Abs(sample.Series.Values[sample.Index])
		if observedPower < 10 {
			continue
		}
		energyWh := (sample.ETAMin * observedPower) / 60.0
		capWh := (energyWh * 100.0) / denom
		if capWh <= 0 || math.IsNaN(capWh) || math.IsInf(capWh, 0) {
			continue
		}
		byProfile[sample.Profile] = append(byProfile[sample.Profile], capWh)
		byProfile[profileGeneric] = append(byProfile[profileGeneric], capWh)
	}

	out := make(map[mlProfileName]float64, 3)
	out[profileD2M] = robustMedianWh(byProfile[profileD2M], 4096)
	out[profileDPU] = robustMedianWh(byProfile[profileDPU], 12288)
	out[profileGeneric] = robustMedianWh(byProfile[profileGeneric], 8192)
	return out
}

func robustMedianWh(values []float64, fallback float64) float64 {
	if len(values) == 0 {
		return fallback
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	start := int(math.Floor(float64(len(cp)) * 0.1))
	end := int(math.Ceil(float64(len(cp)) * 0.9))
	if start < 0 {
		start = 0
	}
	if end > len(cp) {
		end = len(cp)
	}
	if end-start >= 5 {
		cp = cp[start:end]
	}
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2.0
	}
	return cp[mid]
}

func optimizeProfile(records []trainingRecord, profile mlProfileName, options optimizerOptions) (optimizerResult, error) {
	if options.candidateCount < 8 {
		options.candidateCount = 8
	}
	if len(options.stageFractions) == 0 {
		options.stageFractions = []float64{0.2, 0.5, 1.0}
	}
	series, samples := buildTrainingSet(records, profile)
	if len(samples) < 16 {
		return optimizerResult{}, fmt.Errorf("insufficient samples for profile %s: %d", profile, len(samples))
	}
	capacity := estimateCapacityWhByProfile(samples)
	targetCapWh := capacity[profile]
	if profile == profileGeneric {
		targetCapWh = capacity[profileGeneric]
	}

	rng := rand.New(rand.NewSource(options.seed))
	candidates := makeInitialCandidates(options.candidateCount, rng)
	if len(candidates) == 0 {
		return optimizerResult{}, errors.New("failed to build candidate set")
	}

	stageStats := make([]stageStat, 0, len(options.stageFractions))
	for stageIdx, fraction := range options.stageFractions {
		stageSamples := samples
		usedStratified := false
		if fraction < 0.999 {
			stageSeed := options.seed + int64((stageIdx+1)*1009)
			stageSamples = stratifiedSample(samples, fraction, stageSeed)
			usedStratified = true
		}
		evaluated := evaluateCandidates(candidates, stageSamples, targetCapWh)
		sort.Slice(evaluated, func(i, j int) bool {
			return evaluated[i].Score < evaluated[j].Score
		})
		retain := len(evaluated)
		if stageIdx < len(options.stageFractions)-1 {
			retain = int(math.Ceil(float64(len(evaluated)) / 3.0))
			if retain < 2 {
				retain = 2
			}
			if retain > len(evaluated) {
				retain = len(evaluated)
			}
		}
		stageStats = append(stageStats, stageStat{
			Fraction:       fraction,
			SampleCount:    len(stageSamples),
			CandidateCount: len(candidates),
			RetainedCount:  retain,
			BestStageScore: evaluated[0].Score,
			BestStageCover: evaluated[0].Coverage,
			UsedStratified: usedStratified,
		})
		candidates = make([]profileCandidate, retain)
		copy(candidates, evaluated[:retain])
	}

	final := evaluateCandidates(candidates, samples, targetCapWh)
	sort.Slice(final, func(i, j int) bool {
		return final[i].Score < final[j].Score
	})
	best := final[0]
	return optimizerResult{
		Profile:        profile,
		Rows:           len(records),
		SeriesCount:    len(series),
		SampleCount:    len(samples),
		EstimatedCapWh: targetCapWh,
		Best:           best,
		StageStats:     stageStats,
	}, nil
}

func evaluateCandidates(candidates []profileCandidate, samples []trainingSample, capWh float64) []profileCandidate {
	out := make([]profileCandidate, len(candidates))
	for i := range candidates {
		score, coverage := evaluateCandidate(candidates[i].Params, samples, capWh)
		candidate := candidates[i]
		candidate.Score = score
		candidate.Coverage = coverage
		out[i] = candidate
	}
	return out
}

func evaluateCandidate(params profileParams, samples []trainingSample, capWh float64) (float64, float64) {
	if len(samples) == 0 {
		return math.Inf(1), 0
	}
	total := 0.0
	valid := 0
	covered := 0

	for _, sample := range samples {
		predW, ok := predictPowerForMode(sample.Series, sample.Index, sample.Mode, params)
		if !ok {
			total += 2.5
			valid++
			continue
		}

		targetEnergyWh := capWh * (defaultMaxChargeSOC - sample.SOCPct) / 100.0
		if sample.Mode == modeDischarge {
			targetEnergyWh = capWh * (sample.SOCPct - defaultMinDischargeSOC) / 100.0
		}
		if targetEnergyWh < 0 {
			targetEnergyWh = 0
		}
		if targetEnergyWh <= 0 || predW <= minTrainingPowerWatts {
			total += 1.5
			valid++
			continue
		}

		predEta := (targetEnergyWh * 60.0) / predW
		etaErr := math.Abs(math.Log1p(predEta) - math.Log1p(sample.ETAMin))
		observedW := math.Abs(sample.Series.Values[sample.Index])
		powerErr := math.Abs(predW-observedW) / math.Max(25.0, observedW)

		total += etaErr + (0.35 * powerErr)
		valid++
		covered++
	}

	if valid == 0 {
		return math.Inf(1), 0
	}
	score := total / float64(valid)
	coverage := float64(covered) / float64(valid)
	if coverage < 0.75 {
		score += (0.75 - coverage) * 2.0
	}
	return score, coverage
}

func predictPowerForMode(series *deviceSeries, idx int, mode sampleMode, params profileParams) (float64, bool) {
	pred, threshold, ok := predictNetPower(series, idx, params)
	if !ok {
		return 0, false
	}
	switch mode {
	case modeCharge:
		if pred <= threshold {
			return 0, false
		}
		return pred, true
	case modeDischarge:
		if pred >= -threshold {
			return 0, false
		}
		return -pred, true
	default:
		return 0, false
	}
}

func predictNetPower(series *deviceSeries, idx int, params profileParams) (float64, float64, bool) {
	if series == nil || idx < 1 || idx >= len(series.Values) {
		return 0, 0, false
	}
	window := clampInt(params.stableWindow, 2, idx+1)
	if idx >= 11 {
		recentMean := meanRange(series.Values, idx-5, idx)
		prevMean := meanRange(series.Values, idx-11, idx-6)
		jump := math.Abs(recentMean - prevMean)
		span := math.Abs(series.Values[idx] - series.Values[idx-5])
		base := math.Max(systemNetThresholdWatts*2.0, series.absMeanAt(window, idx)*0.4)
		if jump > base*0.45 || span > base*0.65 {
			window = clampInt(params.fastWindow, 2, idx+1)
		}
	}

	recentWindow := clampInt(params.recentWindow, 1, window)
	mediumWindow := clampInt(params.mediumWindow, recentWindow, window)
	trendWindow := clampInt(params.trendWindow, 1, idx)

	latest := series.Values[idx]
	recent := series.meanAt(recentWindow, idx)
	medium := series.meanAt(mediumWindow, idx)
	trend := 0.0
	if trendWindow > 0 && idx-trendWindow >= 0 {
		prev := series.Values[idx-trendWindow]
		trend = (latest - prev) / float64(trendWindow)
	}
	pred := (params.latestWeight * latest) +
		(params.recentWeight * recent) +
		(params.mediumWeight * medium) +
		(params.trendWeight * trend)

	threshold := math.Max(3.0, systemNetThresholdWatts*params.netThresholdScale)
	if math.Abs(pred) < threshold*0.85 && math.Abs(recent) >= threshold {
		pred = recent
	}
	return pred, threshold, true
}

func meanRange(values []float64, start int, end int) float64 {
	if len(values) == 0 {
		return 0
	}
	if start < 0 {
		start = 0
	}
	if end >= len(values) {
		end = len(values) - 1
	}
	if start > end {
		return 0
	}
	sum := 0.0
	for i := start; i <= end; i++ {
		sum += values[i]
	}
	return sum / float64(end-start+1)
}

func stratifiedSample(samples []trainingSample, fraction float64, seed int64) []trainingSample {
	if len(samples) == 0 {
		return nil
	}
	if fraction <= 0 {
		fraction = 0.1
	}
	if fraction > 1 {
		fraction = 1
	}
	groups := make(map[string][]trainingSample)
	for _, sample := range samples {
		groups[sample.Stratum] = append(groups[sample.Stratum], sample)
	}

	out := make([]trainingSample, 0, int(math.Ceil(float64(len(samples))*fraction)))
	for key, group := range groups {
		count := int(math.Ceil(float64(len(group)) * fraction))
		if count < 1 {
			count = 1
		}
		if strings.Contains(key, "transition=true") {
			boosted := int(math.Ceil(float64(count) * 1.5))
			if boosted > count {
				count = boosted
			}
		}
		if count > len(group) {
			count = len(group)
		}
		rng := rand.New(rand.NewSource(seed + int64(stableHash(key))))
		perm := rng.Perm(len(group))
		for i := 0; i < count; i++ {
			out = append(out, group[perm[i]])
		}
	}
	return out
}

func stableHash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func makeInitialCandidates(count int, rng *rand.Rand) []profileCandidate {
	if count < 1 {
		return nil
	}
	out := make([]profileCandidate, 0, count)
	base := profileParams{
		fastWindow:        20,
		stableWindow:      36,
		recentWindow:      4,
		mediumWindow:      14,
		trendWindow:       7,
		latestWeight:      0.68,
		recentWeight:      0.20,
		mediumWeight:      0.12,
		trendWeight:       0.25,
		netThresholdScale: 0.9,
	}
	out = append(out, profileCandidate{Params: base})
	for len(out) < count {
		weights := randomWeights3(rng)
		candidate := profileParams{
			fastWindow:        randomIntRange(rng, 12, 30),
			stableWindow:      randomIntRange(rng, 24, 60),
			recentWindow:      randomIntRange(rng, 3, 8),
			mediumWindow:      randomIntRange(rng, 10, 24),
			trendWindow:       randomIntRange(rng, 3, 12),
			latestWeight:      weights[0],
			recentWeight:      weights[1],
			mediumWeight:      weights[2],
			trendWeight:       randomFloatRange(rng, 0.10, 0.35),
			netThresholdScale: randomFloatRange(rng, 0.70, 1.20),
		}
		if candidate.mediumWindow < candidate.recentWindow {
			candidate.mediumWindow = candidate.recentWindow
		}
		if candidate.stableWindow < candidate.mediumWindow {
			candidate.stableWindow = candidate.mediumWindow
		}
		out = append(out, profileCandidate{Params: candidate})
	}
	return out
}

func randomWeights3(rng *rand.Rand) [3]float64 {
	a := randomFloatRange(rng, 0.45, 0.90)
	b := randomFloatRange(rng, 0.05, 0.35)
	c := randomFloatRange(rng, 0.03, 0.25)
	sum := a + b + c
	if sum <= 0 {
		return [3]float64{0.7, 0.2, 0.1}
	}
	return [3]float64{a / sum, b / sum, c / sum}
}

func randomIntRange(rng *rand.Rand, min int, max int) int {
	if max <= min {
		return min
	}
	return min + rng.Intn(max-min+1)
}

func randomFloatRange(rng *rand.Rand, min float64, max float64) float64 {
	if max <= min {
		return min
	}
	return min + rng.Float64()*(max-min)
}

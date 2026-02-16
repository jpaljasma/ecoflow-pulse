package main

import (
	"math"
	"testing"
)

func TestStratifiedSampleKeepsAllStrata(t *testing.T) {
	series := &deviceSeries{
		DeviceSN:     "sn",
		Values:       []float64{10, 20, 30, 40, 50, 60},
		meanCache:    make(map[int][]float64),
		absMeanCache: make(map[int][]float64),
	}
	series.buildPrefix()

	samples := []trainingSample{
		{Series: series, Index: 1, Mode: modeCharge, ETAMin: 100, SOCPct: 40, Profile: profileD2M, Stratum: "d2m|charge|transition=false"},
		{Series: series, Index: 2, Mode: modeCharge, ETAMin: 90, SOCPct: 41, Profile: profileD2M, Stratum: "d2m|charge|transition=false"},
		{Series: series, Index: 3, Mode: modeDischarge, ETAMin: 80, SOCPct: 39, Profile: profileD2M, Stratum: "d2m|discharge|transition=true"},
		{Series: series, Index: 4, Mode: modeDischarge, ETAMin: 75, SOCPct: 38, Profile: profileDPU, Stratum: "dpu|discharge|transition=false"},
		{Series: series, Index: 5, Mode: modeCharge, ETAMin: 70, SOCPct: 37, Profile: profileDPU, Stratum: "dpu|charge|transition=true"},
	}

	subset := stratifiedSample(samples, 0.2, 42)
	if len(subset) < 4 {
		t.Fatalf("expected at least one sample per stratum, got=%d", len(subset))
	}

	seen := map[string]bool{}
	for _, sample := range subset {
		seen[sample.Stratum] = true
	}
	for _, sample := range samples {
		if !seen[sample.Stratum] {
			t.Fatalf("missing stratum %q in sampled set", sample.Stratum)
		}
	}
}

func TestDeviceSeriesFeatureCacheMeanMatchesManual(t *testing.T) {
	series := &deviceSeries{
		DeviceSN:     "sn",
		Values:       []float64{10, -20, 30, -40, 50, -60, 70},
		meanCache:    make(map[int][]float64),
		absMeanCache: make(map[int][]float64),
	}
	series.buildPrefix()

	gotMean := series.meanAt(3, 5)
	wantMean := (-40 + 50 - 60) / 3.0
	if math.Abs(gotMean-wantMean) > 1e-9 {
		t.Fatalf("mean mismatch: got=%f want=%f", gotMean, wantMean)
	}

	gotAbsMean := series.absMeanAt(4, 6)
	wantAbsMean := (40 + 50 + 60 + 70) / 4.0
	if math.Abs(gotAbsMean-wantAbsMean) > 1e-9 {
		t.Fatalf("abs mean mismatch: got=%f want=%f", gotAbsMean, wantAbsMean)
	}

	_ = series.meanAt(3, 4)
	_ = series.meanAt(3, 6)
	if len(series.meanCache) != 1 {
		t.Fatalf("expected mean cache to contain one window series, got=%d", len(series.meanCache))
	}
}

func TestOptimizeProfileReturnsFiniteCandidate(t *testing.T) {
	records := make([]trainingRecord, 0, 80)
	deviceSN := "R351TEST123"
	for i := 0; i < 80; i++ {
		mode := modeCharge
		netW := 120.0
		soc := 30.0 + float64(i)*0.2
		eta := 200.0 - float64(i)*1.2
		if i > 40 {
			mode = modeDischarge
			netW = -90
			soc = 50.0 - float64(i-40)*0.25
			eta = 260.0 - float64(i-40)*1.5
		}
		records = append(records, trainingRecord{
			TSUnixMS: int64(1_700_000_000_000 + i*10_000),
			DeviceSN: deviceSN,
			Product:  "DELTA 2 Max",
			Profile:  profileD2M,
			Mode:     mode,
			HasETA:   true,
			ETAMin:   eta,
			HasSOC:   true,
			SOCPct:   soc,
			HasNet:   true,
			NetW:     netW,
		})
	}

	result, err := optimizeProfile(records, profileD2M, optimizerOptions{
		candidateCount: 64,
		seed:           7,
		stageFractions: []float64{0.25, 0.5, 1.0},
	})
	if err != nil {
		t.Fatalf("optimize profile failed: %v", err)
	}
	if math.IsInf(result.Best.Score, 0) || math.IsNaN(result.Best.Score) {
		t.Fatalf("expected finite best score, got=%f", result.Best.Score)
	}
	if len(result.StageStats) != 3 {
		t.Fatalf("expected 3 stage stats, got=%d", len(result.StageStats))
	}
	if result.Best.Coverage <= 0 {
		t.Fatalf("expected positive coverage, got=%f", result.Best.Coverage)
	}
}

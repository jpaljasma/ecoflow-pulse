package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

func main() {
	var csvPath string
	var profileArg string
	var candidates int
	var seed int64
	var stagesArg string

	flag.StringVar(&csvPath, "csv", "logs/telemetry_training.csv", "Path to telemetry training CSV")
	flag.StringVar(&profileArg, "profile", "all", "Profile to train: d2m|dpu|generic|all")
	flag.IntVar(&candidates, "candidates", 720, "Number of random candidates before halving")
	flag.Int64Var(&seed, "seed", time.Now().UnixNano(), "Random seed")
	flag.StringVar(&stagesArg, "stages", "0.2,0.5,1.0", "Successive-halving data fractions")
	flag.Parse()

	stageFractions, err := parseStageFractions(stagesArg)
	if err != nil {
		log.Fatalf("invalid stage fractions: %v", err)
	}

	records, err := loadTrainingRecords(csvPath)
	if err != nil {
		log.Fatalf("load training records: %v", err)
	}

	profiles := resolveProfileSelection(profileArg)
	if len(profiles) == 0 {
		log.Fatalf("unsupported profile selection %q", profileArg)
	}

	options := optimizerOptions{
		candidateCount: candidates,
		seed:           seed,
		stageFractions: stageFractions,
	}

	fmt.Printf("training_csv: %s\n", csvPath)
	fmt.Printf("rows: %d\n", len(records))
	fmt.Printf("seed: %d\n", seed)
	fmt.Printf("stages: %s\n\n", stagesArg)

	for _, profile := range profiles {
		result, err := optimizeProfile(records, profile, options)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profile %s: %v\n", profile, err)
			continue
		}
		printResult(result)
		fmt.Println()
	}
}

func resolveProfileSelection(raw string) []mlProfileName {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "d2m":
		return []mlProfileName{profileD2M}
	case "dpu":
		return []mlProfileName{profileDPU}
	case "generic":
		return []mlProfileName{profileGeneric}
	case "all", "":
		return []mlProfileName{profileD2M, profileDPU, profileGeneric}
	default:
		return nil
	}
}

func parseStageFractions(raw string) ([]float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []float64{0.2, 0.5, 1.0}, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]float64, 0, len(parts))
	for _, part := range parts {
		value, ok := parseFloat(strings.TrimSpace(part))
		if !ok || value <= 0 || value > 1 {
			return nil, fmt.Errorf("bad stage fraction %q", part)
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no stage fractions")
	}
	if math.Abs(out[len(out)-1]-1.0) > 1e-9 {
		out = append(out, 1.0)
	}
	return out, nil
}

func printResult(result optimizerResult) {
	fmt.Printf("profile: %s\n", result.Profile)
	fmt.Printf("rows: %d, series: %d, samples: %d\n", result.Rows, result.SeriesCount, result.SampleCount)
	fmt.Printf("estimated_capacity_wh: %.1f\n", result.EstimatedCapWh)
	for i, stage := range result.StageStats {
		fmt.Printf(
			"stage[%d]: fraction=%.2f samples=%d candidates=%d retained=%d best_score=%.6f best_coverage=%.3f stratified=%t\n",
			i+1,
			stage.Fraction,
			stage.SampleCount,
			stage.CandidateCount,
			stage.RetainedCount,
			stage.BestStageScore,
			stage.BestStageCover,
			stage.UsedStratified,
		)
	}
	fmt.Printf("best_score: %.6f\n", result.Best.Score)
	fmt.Printf("best_coverage: %.3f\n", result.Best.Coverage)
	fmt.Printf("params:\n")
	fmt.Printf("  fastWindow: %d\n", result.Best.Params.fastWindow)
	fmt.Printf("  stableWindow: %d\n", result.Best.Params.stableWindow)
	fmt.Printf("  recentWindow: %d\n", result.Best.Params.recentWindow)
	fmt.Printf("  mediumWindow: %d\n", result.Best.Params.mediumWindow)
	fmt.Printf("  trendWindow: %d\n", result.Best.Params.trendWindow)
	fmt.Printf("  latestWeight: %.15f\n", result.Best.Params.latestWeight)
	fmt.Printf("  recentWeight: %.15f\n", result.Best.Params.recentWeight)
	fmt.Printf("  mediumWeight: %.15f\n", result.Best.Params.mediumWeight)
	fmt.Printf("  trendWeight: %.15f\n", result.Best.Params.trendWeight)
	fmt.Printf("  netThresholdScale: %.15f\n", result.Best.Params.netThresholdScale)
}

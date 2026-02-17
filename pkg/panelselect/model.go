package panelselect

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ModelVersion = 1
)

var FeatureNames = []string{
	"median_active_w",
	"p95_active_w",
	"median_active_v",
	"p95_active_v",
	"median_active_a",
	"active_ratio",
	"charging_ratio",
}

var featureScales = []float64{
	30.0,
	45.0,
	8.0,
	12.0,
	2.0,
	0.20,
	0.20,
}

var featureWeights = []float64{
	1.00,
	0.80,
	1.80,
	1.20,
	1.00,
	0.60,
	0.60,
}

type Model struct {
	Version        int      `json:"version"`
	GeneratedAtUTC string   `json:"generated_at_utc"`
	SourceCSV      string   `json:"source_csv"`
	Classes        []Class  `json:"classes"`
	FeatureNames   []string `json:"feature_names"`
}

type Class struct {
	ID            string    `json:"id"`
	Profile       string    `json:"profile"`
	Port          string    `json:"port"`
	PanelSetup    string    `json:"panel_setup"`
	PanelCount    int       `json:"panel_count"`
	NominalTotalW float64   `json:"nominal_total_w"`
	SampleCount   int       `json:"sample_count"`
	DeviceSNs     []string  `json:"device_sns"`
	Centroid      []float64 `json:"centroid"`
}

type Prediction struct {
	ClassID      string
	PanelSetup   string
	Confidence   float64
	Distance     float64
	SampleCount  int
	Profile      string
	Port         string
	NominalTotal float64
}

func NewModel() *Model {
	return &Model{
		Version:        ModelVersion,
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		FeatureNames:   append([]string(nil), FeatureNames...),
		Classes:        []Class{},
	}
}

func Load(path string) (*Model, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model: %w", err)
	}
	var model Model
	if err := json.Unmarshal(content, &model); err != nil {
		return nil, fmt.Errorf("decode model json: %w", err)
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

func (m *Model) Save(path string) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal model json: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return fmt.Errorf("write model file: %w", err)
	}
	return nil
}

func (m *Model) Validate() error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	if len(m.FeatureNames) == 0 {
		m.FeatureNames = append([]string(nil), FeatureNames...)
	}
	if len(m.FeatureNames) != len(FeatureNames) {
		return fmt.Errorf("feature name mismatch: got=%d want=%d", len(m.FeatureNames), len(FeatureNames))
	}
	if m.Version == 0 {
		m.Version = ModelVersion
	}
	for i, class := range m.Classes {
		if len(class.Centroid) != len(FeatureNames) {
			return fmt.Errorf("class %q centroid width mismatch: got=%d want=%d", class.ID, len(class.Centroid), len(FeatureNames))
		}
		if class.ID == "" {
			return fmt.Errorf("class at index %d has empty id", i)
		}
	}
	return nil
}

func NormalizeProfile(productName string) string {
	productName = strings.ToLower(strings.TrimSpace(productName))
	switch {
	case strings.Contains(productName, "delta 2 max"):
		return "d2m"
	case strings.Contains(productName, "delta pro ultra"):
		return "dpu"
	default:
		return "generic"
	}
}

func NormalizePort(port string) string {
	switch strings.ToLower(strings.TrimSpace(port)) {
	case "high":
		return "high"
	default:
		return "low"
	}
}

func (m *Model) ClassesFor(profile, port string) []Class {
	if m == nil {
		return nil
	}
	profile = strings.ToLower(strings.TrimSpace(profile))
	port = NormalizePort(port)
	out := make([]Class, 0, len(m.Classes))
	for _, class := range m.Classes {
		if strings.ToLower(strings.TrimSpace(class.Profile)) != profile {
			continue
		}
		if NormalizePort(class.Port) != port {
			continue
		}
		out = append(out, class)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func Predict(model *Model, profile, port string, features []float64, sampleCount int) (Prediction, bool) {
	if model == nil || len(features) != len(FeatureNames) {
		return Prediction{}, false
	}
	classes := model.ClassesFor(profile, port)
	if len(classes) == 0 {
		return Prediction{}, false
	}

	bestIdx := -1
	bestDistance := math.MaxFloat64
	secondDistance := math.MaxFloat64
	for idx, class := range classes {
		d := featureDistance(features, class.Centroid)
		if d < bestDistance {
			secondDistance = bestDistance
			bestDistance = d
			bestIdx = idx
			continue
		}
		if d < secondDistance {
			secondDistance = d
		}
	}
	if bestIdx < 0 {
		return Prediction{}, false
	}

	bestClass := classes[bestIdx]
	confidence := predictionConfidence(bestDistance, secondDistance, len(classes), sampleCount)
	return Prediction{
		ClassID:      bestClass.ID,
		PanelSetup:   bestClass.PanelSetup,
		Confidence:   confidence,
		Distance:     bestDistance,
		SampleCount:  sampleCount,
		Profile:      bestClass.Profile,
		Port:         bestClass.Port,
		NominalTotal: bestClass.NominalTotalW,
	}, true
}

func featureDistance(left, right []float64) float64 {
	total := 0.0
	for i := range left {
		scale := featureScales[i]
		if scale <= 0 {
			scale = 1
		}
		weight := featureWeights[i]
		if weight <= 0 {
			weight = 1
		}
		delta := (left[i] - right[i]) / scale
		total += weight * delta * delta
	}
	return math.Sqrt(total)
}

func predictionConfidence(bestDistance, secondDistance float64, classCount int, sampleCount int) float64 {
	if classCount <= 1 {
		// With one class, confidence is driven by fit and sample maturity.
		fit := 1.0 / (1.0 + 0.25*bestDistance)
		maturity := clamp(float64(sampleCount)/120.0, 0, 1)
		value := 0.5 + 0.5*(fit*maturity)
		return clamp(value, 0, 1)
	}
	if secondDistance <= 0 {
		return 0
	}
	sep := (secondDistance - bestDistance) / secondDistance
	base := 1.0 / (1.0 + bestDistance)
	value := (0.55 * base) + (0.45 * clamp(sep, 0, 1))
	return clamp(value, 0, 1)
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

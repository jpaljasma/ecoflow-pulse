package inference

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	valkey "github.com/valkey-io/valkey-go"

	"github.com/jpaljasma/ecoflow-pulse/internal/energydashboard"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

const (
	energyComparisonModelKey     = "energy-comparison-score"
	energyComparisonModelVersion = "v1"
	energyComparisonTTL          = 2 * time.Hour
)

type EnergyComparisonVerdict string

const (
	EnergyComparisonVerdictSolarFreedomUp   EnergyComparisonVerdict = "solar_freedom_up"
	EnergyComparisonVerdictSolarFreedomDown EnergyComparisonVerdict = "solar_freedom_down"
	EnergyComparisonVerdictMixedShift       EnergyComparisonVerdict = "mixed_shift"
	EnergyComparisonVerdictSteadyState      EnergyComparisonVerdict = "steady_state"
)

type EnergyComparisonCardCategory string

const (
	EnergyComparisonCardSelfSufficiency EnergyComparisonCardCategory = "self_sufficiency"
	EnergyComparisonCardSolar           EnergyComparisonCardCategory = "solar"
	EnergyComparisonCardLoad            EnergyComparisonCardCategory = "load"
	EnergyComparisonCardBattery         EnergyComparisonCardCategory = "battery"
	EnergyComparisonCardGrid            EnergyComparisonCardCategory = "grid"
	EnergyComparisonCardValue           EnergyComparisonCardCategory = "value"
)

type EnergyComparisonScope struct {
	Mode              string   `json:"mode"`
	DeviceID          string   `json:"device_id,omitempty"`
	ResolvedDeviceIDs []string `json:"resolved_device_ids,omitempty"`
}

type EnergyComparisonCard struct {
	Category       EnergyComparisonCardCategory `json:"category"`
	Title          string                       `json:"title"`
	Summary        string                       `json:"summary"`
	Recommendation string                       `json:"recommendation,omitempty"`
	Score          float64                      `json:"score"`
	Confidence     float64                      `json:"confidence"`
	Evidence       []Evidence                   `json:"evidence,omitempty"`
	Attributes     map[string]any               `json:"attributes,omitempty"`
}

type EnergyComparisonInsight struct {
	ID           string                  `json:"id"`
	Scope        EnergyComparisonScope   `json:"scope"`
	Preset       string                  `json:"preset"`
	Timezone     string                  `json:"timezone"`
	VerdictClass EnergyComparisonVerdict `json:"verdict_class"`
	Headline     string                  `json:"headline"`
	Summary      string                  `json:"summary"`
	Score        float64                 `json:"score"`
	Confidence   float64                 `json:"confidence"`
	ModelKey     string                  `json:"model_key"`
	ModelVersion string                  `json:"model_version"`
	GeneratedAt  time.Time               `json:"generated_at"`
	ExpiresAt    time.Time               `json:"expires_at"`
	Tags         []string                `json:"tags,omitempty"`
	Cards        []EnergyComparisonCard  `json:"cards,omitempty"`
	Evidence     []Evidence              `json:"evidence,omitempty"`
	Attributes   map[string]any          `json:"attributes,omitempty"`
}

type EnergyComparisonRecord struct {
	Status       Status                   `json:"status"`
	StatusDetail string                   `json:"status_detail,omitempty"`
	Insight      *EnergyComparisonInsight `json:"insight,omitempty"`
}

type EnergyComparisonCacheKey struct {
	ScopeMode         string
	DeviceID          string
	ResolvedDeviceIDs []string
	Preset            string
	Timezone          string
	GridPricePerKwh   float64
	Currency          string
	RefreshSlotUnixMs int64
}

type EnergyComparisonCache interface {
	GetEnergyComparison(ctx context.Context, key EnergyComparisonCacheKey) (*EnergyComparisonRecord, error)
	PutEnergyComparison(ctx context.Context, key EnergyComparisonCacheKey, value EnergyComparisonRecord) error
}

type EnergyComparisonInput struct {
	Now             time.Time
	Scope           EnergyComparisonScope
	Preset          string
	Timezone        string
	GridPricePerKwh float64
	Currency        string
	CurrentEnergy   telemetryquery.Series
	PreviousEnergy  telemetryquery.Series
	CurrentPower    telemetryquery.Series
	PreviousPower   telemetryquery.Series
}

func (s *ValkeyStore) GetEnergyComparison(ctx context.Context, key EnergyComparisonCacheKey) (*EnergyComparisonRecord, error) {
	raw, err := s.client.Do(ctx, s.client.B().Get().Key(s.energyComparisonKey(key)).Build()).ToString()
	if err != nil {
		if errorsIsValkeyNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read energy comparison cache: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out EnergyComparisonRecord
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode energy comparison cache: %w", err)
	}
	if out.Insight != nil {
		cloned := cloneEnergyComparisonInsight(*out.Insight)
		out.Insight = &cloned
	}
	return &out, nil
}

func (s *ValkeyStore) PutEnergyComparison(ctx context.Context, key EnergyComparisonCacheKey, value EnergyComparisonRecord) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal energy comparison cache: %w", err)
	}
	return s.client.Do(
		ctx,
		s.client.B().Set().Key(s.energyComparisonKey(key)).Value(valkey.BinaryString(encoded)).ExSeconds(int64(energyComparisonTTL/time.Second)).Build(),
	).Error()
}

func (s *ValkeyStore) energyComparisonKey(key EnergyComparisonCacheKey) string {
	scopeTag := strings.TrimSpace(key.ScopeMode)
	if scopeTag == "" {
		scopeTag = "device"
	}
	deviceIDs := append([]string(nil), key.ResolvedDeviceIDs...)
	sort.Strings(deviceIDs)
	h := sha1.New()
	_, _ = h.Write([]byte(scopeTag))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strings.TrimSpace(key.DeviceID)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strings.Join(deviceIDs, ",")))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strings.TrimSpace(key.Preset)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strings.TrimSpace(key.Timezone)))
	_, _ = h.Write([]byte("|"))
	_, _ = fmt.Fprintf(h, "%.4f|%s|%d", key.GridPricePerKwh, strings.TrimSpace(key.Currency), key.RefreshSlotUnixMs)
	return fmt.Sprintf("%s:{energy-comparison:%s}", s.keyPrefix, hex.EncodeToString(h.Sum(nil)))
}

func BuildEnergyComparisonInsight(input EnergyComparisonInput) EnergyComparisonRecord {
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summary := energydashboard.BuildSummary(input.CurrentEnergy, input.PreviousEnergy, input.GridPricePerKwh)
	deviceLabel := "fleet"
	if strings.TrimSpace(input.Scope.DeviceID) != "" {
		deviceLabel = strings.TrimSpace(input.Scope.DeviceID)
	}
	confidence := comparisonConfidence(summary)

	selfSufficiencySignal := signedSignal(summary.SelfSufficiencyPct.Delta, 18)
	gridSignal := signedSignal(-summary.EstimatedACInputCost.Delta, 0.20)
	solarSignal := signedSignal(summary.SolarGeneratedKWh.Delta, 1.0)
	valueSignal := signedSignal(summary.EstimatedValue.Delta, 0.20)
	loadSignal := signedSignal(-summary.LoadConsumedKWh.Delta, 1.0)
	batterySignal := batteryStrategySignal(summary.BatteryNetKWh.Delta)

	totalScore := 0.36*selfSufficiencySignal + 0.22*gridSignal + 0.18*solarSignal + 0.12*valueSignal + 0.07*loadSignal + 0.05*batterySignal
	totalScore = clamp(totalScore, -1, 1)

	verdict := EnergyComparisonVerdictMixedShift
	headline := "Mixed solar freedom"
	summaryText := "Solar freedom signals moved in different directions."
	switch {
	case totalScore >= 0.18:
		verdict = EnergyComparisonVerdictSolarFreedomUp
		headline = "More solar freedom"
		summaryText = "Self-sufficiency and solar-backed usage improved versus the previous window."
	case totalScore <= -0.18:
		verdict = EnergyComparisonVerdictSolarFreedomDown
		headline = "Less solar freedom"
		summaryText = "Grid dependence or weaker solar usage reduced solar freedom versus the previous window."
	case math.Abs(totalScore) < 0.08:
		verdict = EnergyComparisonVerdictSteadyState
		headline = "Mostly unchanged"
		summaryText = "The overall energy pattern stayed close to the previous window."
	}

	cards := rankComparisonCards(summary, confidence)
	topEvidence := []Evidence{
		{
			Source:  EvidenceSourceModelOutput,
			Summary: "Weighted comparison model prioritized self-sufficiency, grid dependence, and solar generation.",
			Metrics: map[string]any{
				"total_score":             totalScore,
				"self_sufficiency_signal": selfSufficiencySignal,
				"grid_dependence_signal":  gridSignal,
				"solar_generation_signal": solarSignal,
				"value_signal":            valueSignal,
				"load_signal":             loadSignal,
				"battery_strategy_signal": batterySignal,
				"resolved_device_count":   len(input.Scope.ResolvedDeviceIDs),
				"current_energy_points":   len(input.CurrentEnergy.Points),
				"previous_energy_points":  len(input.PreviousEnergy.Points),
				"current_power_points":    len(input.CurrentPower.Points),
				"previous_power_points":   len(input.PreviousPower.Points),
			},
		},
		{
			Source:  EvidenceSourceRollupHistory,
			Summary: "Comparison used EnergyService rollup summaries for the current and previous windows.",
			Metrics: map[string]any{
				"preset":                 input.Preset,
				"timezone":               input.Timezone,
				"solar_generated_delta":  summary.SolarGeneratedKWh.Delta,
				"load_consumed_delta":    summary.LoadConsumedKWh.Delta,
				"self_sufficiency_delta": summary.SelfSufficiencyPct.Delta,
				"battery_net_delta":      summary.BatteryNetKWh.Delta,
				"estimated_value_delta":  summary.EstimatedValue.Delta,
				"ac_input_cost_delta":    summary.EstimatedACInputCost.Delta,
			},
		},
	}

	insight := EnergyComparisonInsight{
		ID:           uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("%s|%s|%s", deviceLabel, input.Preset, input.Timezone))).String(),
		Scope:        cloneEnergyComparisonScope(input.Scope),
		Preset:       strings.TrimSpace(input.Preset),
		Timezone:     strings.TrimSpace(input.Timezone),
		VerdictClass: verdict,
		Headline:     headline,
		Summary:      summaryText,
		Score:        totalScore,
		Confidence:   confidence,
		ModelKey:     energyComparisonModelKey,
		ModelVersion: energyComparisonModelVersion,
		GeneratedAt:  now,
		ExpiresAt:    now.Add(time.Hour),
		Tags:         []string{"energy", "comparison", input.Scope.Mode},
		Cards:        cards,
		Evidence:     topEvidence,
		Attributes: map[string]any{
			"currency":            strings.TrimSpace(input.Currency),
			"grid_price_per_kwh":  input.GridPricePerKwh,
			"verdict_class":       string(verdict),
			"resolved_device_ids": append([]string(nil), input.Scope.ResolvedDeviceIDs...),
		},
	}

	return EnergyComparisonRecord{
		Status:       StatusReady,
		StatusDetail: "derived from hourly-cached energy comparison inference",
		Insight:      &insight,
	}
}

func rankComparisonCards(summary energydashboard.Summary, confidence float64) []EnergyComparisonCard {
	cards := []EnergyComparisonCard{
		buildComparisonCard(
			EnergyComparisonCardSelfSufficiency,
			"Self-sufficiency",
			summary.SelfSufficiencyPct.Delta,
			18,
			confidence,
			fmt.Sprintf("Self-sufficiency changed by %.1f percentage points.", summary.SelfSufficiencyPct.Delta),
			selfSufficiencyRecommendation(summary.SelfSufficiencyPct.Delta),
			map[string]any{
				"current_pct":  summary.SelfSufficiencyPct.Current,
				"previous_pct": summary.SelfSufficiencyPct.Previous,
				"delta_pct_pt": summary.SelfSufficiencyPct.Delta,
			},
		),
		buildComparisonCard(
			EnergyComparisonCardSolar,
			"Solar generation",
			summary.SolarGeneratedKWh.Delta,
			1.0,
			confidence,
			fmt.Sprintf("Solar generation changed by %.2fkWh.", summary.SolarGeneratedKWh.Delta),
			"Keep high-draw tasks inside the strongest solar window when generation climbs.",
			map[string]any{
				"current_kwh":  summary.SolarGeneratedKWh.Current,
				"previous_kwh": summary.SolarGeneratedKWh.Previous,
				"delta_kwh":    summary.SolarGeneratedKWh.Delta,
			},
		),
		buildComparisonCard(
			EnergyComparisonCardLoad,
			"Load pressure",
			-summary.LoadConsumedKWh.Delta,
			1.0,
			confidence,
			fmt.Sprintf("Load changed by %.2fkWh.", summary.LoadConsumedKWh.Delta),
			"Shift discretionary AC-heavy loads into the solar window when load grows faster than solar.",
			map[string]any{
				"current_kwh":  summary.LoadConsumedKWh.Current,
				"previous_kwh": summary.LoadConsumedKWh.Previous,
				"delta_kwh":    summary.LoadConsumedKWh.Delta,
			},
		),
		buildComparisonCard(
			EnergyComparisonCardGrid,
			"Grid dependence",
			-summary.EstimatedACInputCost.Delta,
			0.20,
			confidence,
			fmt.Sprintf("Estimated AC input cost changed by %.2f.", summary.EstimatedACInputCost.Delta),
			"Reduce off-solar charging and late-grid peaks when grid cost rises.",
			map[string]any{
				"current_cost":  summary.EstimatedACInputCost.Current,
				"previous_cost": summary.EstimatedACInputCost.Previous,
				"delta_cost":    summary.EstimatedACInputCost.Delta,
			},
		),
		buildComparisonCard(
			EnergyComparisonCardValue,
			"Solar value",
			summary.EstimatedValue.Delta,
			0.20,
			confidence,
			fmt.Sprintf("Estimated solar value changed by %.2f.", summary.EstimatedValue.Delta),
			"Preserve the solar window for flexible consumption when recovered value is climbing.",
			map[string]any{
				"current_value":  summary.EstimatedValue.Current,
				"previous_value": summary.EstimatedValue.Previous,
				"delta_value":    summary.EstimatedValue.Delta,
			},
		),
		buildComparisonCard(
			EnergyComparisonCardBattery,
			"Battery strategy",
			batteryStrategySignal(summary.BatteryNetKWh.Delta),
			1,
			confidence,
			fmt.Sprintf("Battery net changed by %.2fkWh.", summary.BatteryNetKWh.Delta),
			"Favor charging from solar and discharging into peak load windows when battery movement gets less helpful.",
			map[string]any{
				"current_kwh":  summary.BatteryNetKWh.Current,
				"previous_kwh": summary.BatteryNetKWh.Previous,
				"delta_kwh":    summary.BatteryNetKWh.Delta,
			},
		),
	}
	sort.Slice(cards, func(i, j int) bool {
		return math.Abs(cards[i].Score) > math.Abs(cards[j].Score)
	})
	if len(cards) > 4 {
		cards = cards[:4]
	}
	return cards
}

func buildComparisonCard(
	category EnergyComparisonCardCategory,
	title string,
	score float64,
	scale float64,
	confidence float64,
	summary string,
	recommendation string,
	metrics map[string]any,
) EnergyComparisonCard {
	return EnergyComparisonCard{
		Category:       category,
		Title:          title,
		Summary:        summary,
		Recommendation: recommendation,
		Score:          clamp(score/scale, -1, 1),
		Confidence:     confidence,
		Evidence: []Evidence{
			{
				Source:  EvidenceSourceRollupHistory,
				Summary: summary,
				Metrics: metrics,
			},
		},
		Attributes: metrics,
	}
}

func comparisonConfidence(summary energydashboard.Summary) float64 {
	signals := 0
	if math.Abs(summary.SelfSufficiencyPct.Delta) >= 5 {
		signals++
	}
	if math.Abs(summary.SolarGeneratedKWh.Delta) >= 0.3 {
		signals++
	}
	if math.Abs(summary.LoadConsumedKWh.Delta) >= 0.3 {
		signals++
	}
	if math.Abs(summary.EstimatedACInputCost.Delta) >= 0.05 {
		signals++
	}
	if math.Abs(summary.EstimatedValue.Delta) >= 0.05 {
		signals++
	}
	return clamp(0.52+0.09*float64(signals), 0.52, 0.92)
}

func selfSufficiencyRecommendation(delta float64) string {
	if delta >= 0 {
		return "Keep flexible loads aligned to the solar window to preserve the self-sufficiency gain."
	}
	return "Shift more demand into solar hours or reduce off-solar charging to recover self-sufficiency."
}

func batteryStrategySignal(delta float64) float64 {
	switch {
	case delta > 0.4:
		return 0.35
	case delta < -0.4:
		return -0.35
	default:
		return delta / 1.2
	}
}

func signedSignal(delta float64, scale float64) float64 {
	if scale <= 0 {
		scale = 1
	}
	return clamp(delta/scale, -1, 1)
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func cloneEnergyComparisonScope(in EnergyComparisonScope) EnergyComparisonScope {
	return EnergyComparisonScope{
		Mode:              in.Mode,
		DeviceID:          in.DeviceID,
		ResolvedDeviceIDs: append([]string(nil), in.ResolvedDeviceIDs...),
	}
}

func cloneEnergyComparisonInsight(in EnergyComparisonInsight) EnergyComparisonInsight {
	out := in
	out.Scope = cloneEnergyComparisonScope(in.Scope)
	out.Tags = append([]string(nil), in.Tags...)
	out.Cards = make([]EnergyComparisonCard, 0, len(in.Cards))
	for _, card := range in.Cards {
		out.Cards = append(out.Cards, EnergyComparisonCard{
			Category:       card.Category,
			Title:          card.Title,
			Summary:        card.Summary,
			Recommendation: card.Recommendation,
			Score:          card.Score,
			Confidence:     card.Confidence,
			Evidence:       append([]Evidence(nil), card.Evidence...),
			Attributes:     cloneAnyMap(card.Attributes),
		})
	}
	out.Evidence = append([]Evidence(nil), in.Evidence...)
	out.Attributes = cloneAnyMap(in.Attributes)
	return out
}

func errorsIsValkeyNil(err error) bool {
	return err != nil && err == valkey.Nil
}

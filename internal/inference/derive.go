package inference

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	batteryExpansionModelKey     = "battery-expansion-rule"
	batteryExpansionModelVersion = "v1"
	defaultInsightTTL            = 6 * time.Hour
	ecoFlowInviteCode            = "ATH7F3EF1P"
)

type DeriveInput struct {
	Now        time.Time
	Identity   Identity
	Device     DeviceContext
	RawMetrics map[string]float64
}

func DeriveDeviceInsights(input DeriveInput) DeviceInsights {
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	deviceID := strings.TrimSpace(input.Identity.DeviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(input.Device.DeviceID)
	}

	out := DeviceInsights{
		DeviceID:     deviceID,
		Status:       StatusReady,
		StatusDetail: "derived from live inference projection",
		RefreshedAt:  now,
		Insights:     []Insight{},
	}

	model := firstNonEmpty(input.Device.Model, input.Device.ProductName)
	if strings.TrimSpace(model) == "" {
		out.Status = StatusUnavailable
		out.StatusDetail = "device model unavailable for inference derivation"
		return out
	}

	if insight, ok := deriveBatteryExpansionInsight(now, input.Device); ok {
		out.Insights = append(out.Insights, insight)
	}
	return out
}

func deriveBatteryExpansionInsight(now time.Time, device DeviceContext) (Insight, bool) {
	model := strings.TrimSpace(device.Model)
	productName := strings.TrimSpace(device.ProductName)
	if model == "" {
		model = productName
	}
	if model == "" {
		return Insight{}, false
	}

	maxPacks := maxBatteryPacksForModel(model)
	if maxPacks <= 1 {
		return Insight{}, false
	}

	currentPacks := batteryPackCount(device)
	if currentPacks <= 0 {
		currentPacks = 1
	}
	if currentPacks >= maxPacks {
		return Insight{}, false
	}
	if !supportsExtraBattery(device) {
		return Insight{}, false
	}

	headroom := maxPacks - currentPacks
	score := 0.5 + (float64(headroom)/math.Max(float64(maxPacks), 1))*0.5
	if score > 1 {
		score = 1
	}

	titleModel := firstNonEmpty(productName, model)
	summary := fmt.Sprintf("%s is using %d of %d supported battery packs.", titleModel, currentPacks, maxPacks)

	attrs := map[string]any{
		"product_name":                 titleModel,
		"model":                        model,
		"current_battery_packs":        currentPacks,
		"max_battery_packs":            maxPacks,
		"recommended_additional_packs": headroom,
	}

	evidenceMetrics := map[string]any{
		"battery_pack_count":     currentPacks,
		"max_battery_pack_count": maxPacks,
		"supports_extra_battery": true,
	}
	if capacityKWh, ok := batteryCapacityKWh(model, currentPacks); ok {
		attrs["battery_capacity_kwh"] = capacityKWh
		evidenceMetrics["battery_capacity_kwh"] = capacityKWh
	}

	id := insightUUID(device.DeviceID, titleModel, KindBatteryExpansion)
	actions := []Action{}
	if target := batteryUpsellURL(model); target != "" {
		actions = append(actions, Action{
			Kind:   ActionKindExternalURL,
			Label:  fmt.Sprintf("Get More Batteries (%d)", headroom),
			Target: target,
		})
	}
	return Insight{
		ID:           id,
		DeviceID:     strings.TrimSpace(device.DeviceID),
		Kind:         KindBatteryExpansion,
		Title:        "Add extra battery capacity",
		Summary:      summary,
		Score:        score,
		Rank:         1,
		ModelKey:     batteryExpansionModelKey,
		ModelVersion: batteryExpansionModelVersion,
		GeneratedAt:  now,
		ExpiresAt:    now.Add(defaultInsightTTL),
		Tags:         []string{"battery", "upsell"},
		Evidence: []Evidence{
			{
				Source:  EvidenceSourceDeviceCapabilities,
				Summary: "Provider-device capabilities indicate extra-battery support and current pack count.",
				Metrics: evidenceMetrics,
			},
		},
		Actions:    actions,
		Attributes: attrs,
	}, true
}

func insightUUID(deviceID string, seed string, kind Kind) string {
	value := strings.TrimSpace(fmt.Sprintf("%s|%s|%s", deviceID, kind, seed))
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(value)).String()
}

func maxBatteryPacksForModel(model string) int {
	m := normalizeModel(model)
	switch {
	case strings.Contains(m, "delta pro ultra"):
		return 5
	case strings.Contains(m, "delta 2 max"):
		return 3
	default:
		return 0
	}
}

func batteryCapacityKWh(model string, batteryPacks int) (float64, bool) {
	m := normalizeModel(model)
	switch {
	case strings.Contains(m, "delta pro ultra"):
		if batteryPacks <= 0 {
			batteryPacks = 2
		}
		return 6.0 * float64(maxInt(batteryPacks, 1)), true
	case strings.Contains(m, "delta 2 max"):
		if batteryPacks <= 0 {
			batteryPacks = 2
		}
		return 2.048 * float64(maxInt(batteryPacks, 1)), true
	default:
		return 0, false
	}
}

func batteryUpsellURL(model string) string {
	switch {
	case strings.Contains(normalizeModel(model), "delta pro ultra"):
		return "https://us.ecoflow.com/products/delta-pro-ultra-battery?variant=41446274465865&inviteCode=" + ecoFlowInviteCode
	case strings.Contains(normalizeModel(model), "delta 2 max"):
		return "https://us.ecoflow.com/products/delta-2-max-smart-extra-battery-flash-sales?_pos=1&_sid=ed8ecff75&_ss=r&variant=40573812310089&inviteCode=" + ecoFlowInviteCode
	default:
		return ""
	}
}

func batteryPackCount(device DeviceContext) int {
	if value, ok := intFromAny(device.Capabilities["battery_pack_count"]); ok && value > 0 {
		return value
	}
	if value, ok := intFromAny(device.Capabilities["batteryPacks"]); ok && value > 0 {
		return value
	}
	groups, _ := asMap(device.Metadata["groups"])
	if group, ok := asMap(groups["hs_yj751_pd_appshow_addr"]); ok {
		if value, ok := intFromAny(group["bpNum"]); ok && value > 0 {
			return value
		}
	}
	if group, ok := asMap(groups["bms_kitInfo"]); ok {
		if value, ok := intFromAny(group["kitNum"]); ok && value > 0 {
			return value
		}
	}
	return 0
}

func supportsExtraBattery(device DeviceContext) bool {
	if value, ok := boolFromAny(device.Capabilities["supports_extra_battery"]); ok {
		return value
	}
	if value, ok := boolFromAny(device.Capabilities["extraBattery"]); ok {
		return value
	}
	return maxBatteryPacksForModel(device.Model) > 1
}

func normalizeModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.ReplaceAll(model, "-", " ")
	model = strings.ReplaceAll(model, "_", " ")
	return strings.Join(strings.Fields(model), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case int32:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func boolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	default:
		return false, false
	}
}

func asMap(value any) (map[string]any, bool) {
	v, ok := value.(map[string]any)
	return v, ok
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

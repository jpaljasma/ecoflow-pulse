package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func formatAverageWhNoUnit(sum float64, samples int) string {
	wh, ok := averageWh(sum, samples)
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", wh)
}

func formatPercentNoUnit(value float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", value)
}

func averageValue(sum float64, samples int) (float64, bool) {
	if samples <= 0 {
		return 0, false
	}
	return sum / float64(samples), true
}

func averageWh(sum float64, samples int) (float64, bool) {
	if samples <= 0 {
		return 0, false
	}
	avgWatts := sum / float64(samples)
	return avgWatts / 60.0, true
}

func formatWhNoUnit(value float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", value)
}

func formatTemperatureSummary(temps map[string]float64, limit int) string {
	if len(temps) == 0 {
		return "n/a"
	}
	keys := make([]string, 0, len(temps))
	for key := range temps {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%.1fC", key, temps[key]))
	}
	return strings.Join(parts, ",")
}

func formatETAMinutes(minutes float64) string {
	if minutes <= 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return "n/a"
	}
	rounded := int64(math.Round(minutes))
	if rounded < 1 {
		rounded = 1
	}
	return fmt.Sprintf("%dmin (~%s)", rounded, formatMinutesHumanETA(rounded))
}

func formatOptionalWatts(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return formatWatts(value)
}

func formatOptionalVolts(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return fmt.Sprintf("%.1fV", value)
}

func formatOptionalAmps(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return fmt.Sprintf("%.2fA", value)
}

func formatWatts(value float64) string {
	if math.Abs(value) > 1000 {
		return fmt.Sprintf("%.2fkW", value/1000.0)
	}
	return fmt.Sprintf("%.1fW", value)
}

func formatEnergyWh(value float64) string {
	if math.Abs(value) > 1000 {
		return fmt.Sprintf("%.2fkWh", value/1000.0)
	}
	return fmt.Sprintf("%.1fWh", value)
}

func formatSmoothedWattsValue(rawValue string, hasRaw bool, rawWatts float64, hasSmooth bool, smoothWatts float64) string {
	if !hasSmooth || !hasRaw {
		return rawValue
	}
	if math.Abs(rawWatts-smoothWatts) < 0.05 {
		return rawValue
	}
	return fmt.Sprintf("%s (~%s avg)", formatWatts(rawWatts), formatWatts(smoothWatts))
}

func checkboxStatus(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func formatXT150DirectionalValues(has bool, totalWatts float64) (inValue string, outValue string) {
	if !has {
		return "n/a", "n/a"
	}
	// XT150 is directional: negative means battery->inverter (input), positive means inverter->battery (output).
	if totalWatts < 0 {
		return formatWatts(-totalWatts), formatWatts(0)
	}
	if totalWatts > 0 {
		return formatWatts(0), formatWatts(totalWatts)
	}
	return formatWatts(0), formatWatts(0)
}

func normalizeInputChannelWatts(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func normalizeOutputChannelWatts(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func minMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	return minValue, maxValue
}

func formatMinutesHuman(totalMinutes int64) string {
	if totalMinutes <= 0 {
		return "0m"
	}
	const minutesPerHour = int64(60)
	const minutesPerDay = int64(24) * minutesPerHour

	days := totalMinutes / minutesPerDay
	remaining := totalMinutes % minutesPerDay
	hours := remaining / minutesPerHour
	minutes := remaining % minutesPerHour

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatMinutesHumanETA(totalMinutes int64) string {
	if totalMinutes <= 0 {
		return "0m"
	}
	const minutesPerHour = int64(60)
	const minutesPerDay = int64(24) * minutesPerHour

	days := totalMinutes / minutesPerDay
	remaining := totalMinutes % minutesPerDay
	hours := remaining / minutesPerHour
	minutes := remaining % minutesPerHour

	// For long ETAs, keep a compact day/hour representation.
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

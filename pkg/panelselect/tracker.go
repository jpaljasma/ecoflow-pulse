package panelselect

import (
	"math"
	"sort"
	"strings"
)

const (
	DefaultTrackerLimit = 240
)

type Tracker struct {
	limit int

	totalSamples    int
	activeSamples   int
	chargingSamples int

	wattsActive []float64
	voltsActive []float64
	ampsActive  []float64
}

func NewTracker(limit int) *Tracker {
	if limit <= 0 {
		limit = DefaultTrackerLimit
	}
	return &Tracker{
		limit: limit,
	}
}

func (t *Tracker) Observe(watts, volts, amps float64, state string) {
	if t == nil {
		return
	}
	t.totalSamples++
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "charging" {
		t.chargingSamples++
	}

	if watts <= 0 {
		return
	}
	t.activeSamples++
	t.push(&t.wattsActive, sanitizeNumber(watts))
	if volts > 0 {
		t.push(&t.voltsActive, sanitizeNumber(volts))
	}
	if amps > 0 {
		t.push(&t.ampsActive, sanitizeNumber(amps))
	}
}

func (t *Tracker) SampleCount() int {
	if t == nil {
		return 0
	}
	return t.totalSamples
}

func (t *Tracker) FeatureVector() ([]float64, bool) {
	if t == nil || t.totalSamples < 2 || len(t.wattsActive) == 0 {
		return nil, false
	}
	activeRatio := ratio(float64(t.activeSamples), float64(t.totalSamples))
	chargingRatio := ratio(float64(t.chargingSamples), float64(t.totalSamples))

	out := []float64{
		median(t.wattsActive),
		percentile(t.wattsActive, 0.95),
		median(t.voltsActive),
		percentile(t.voltsActive, 0.95),
		median(t.ampsActive),
		activeRatio,
		chargingRatio,
	}
	return out, true
}

func (t *Tracker) push(target *[]float64, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	*target = append(*target, value)
	if len(*target) > t.limit {
		overflow := len(*target) - t.limit
		*target = append((*target)[:0], (*target)[overflow:]...)
	}
}

func sanitizeNumber(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func ratio(num, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return num / den
}

func median(values []float64) float64 {
	return percentile(values, 0.5)
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
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	pos := q * float64(len(cp)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return cp[lo]
	}
	f := pos - float64(lo)
	return cp[lo]*(1-f) + cp[hi]*f
}

func minValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

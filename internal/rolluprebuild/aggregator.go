package rolluprebuild

import (
	"fmt"
	"sort"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
)

const defaultSolarCarryForwardMaxGap = 90 * time.Second

type Resolution string

const (
	ResolutionMinute Resolution = "minute"
	ResolutionHour   Resolution = "hour"
	ResolutionDay    Resolution = "day"
)

type metricAccumulator struct {
	sum   float64
	count int
	min   float64
	max   float64
	valid bool
}

func (m *metricAccumulator) add(value float64) {
	if !m.valid {
		m.sum = value
		m.count = 1
		m.min = value
		m.max = value
		m.valid = true
		return
	}
	m.sum += value
	m.count++
	if value < m.min {
		m.min = value
	}
	if value > m.max {
		m.max = value
	}
}

type BucketRow struct {
	Provider         string
	ProviderDeviceID string
	DeviceID         string
	BucketStart      time.Time
	SampleCount      int
	FirstTsUnixMS    int64
	LastTsUnixMS     int64

	SOC     metricAccumulator
	ACIn    metricAccumulator
	PV      metricAccumulator
	DC      metricAccumulator
	Load    metricAccumulator
	Net     metricAccumulator
	Battery metricAccumulator
	Temp    metricAccumulator

	SolarGeneratedWh    float64
	HasSolarGeneratedWh bool
}

type deviceState struct {
	lastEnvelopeAt    time.Time
	hasLastEnvelopeAt bool
	lastPVAt          time.Time
	hasLastPVAt       bool
	currentPV         float64
	hasPV             bool
}

type Aggregator struct {
	minute map[string]*BucketRow
	hour   map[string]*BucketRow
	day    map[string]*BucketRow

	deviceStateByProviderDeviceID map[string]deviceState
	maxSolarCarryForwardGap       time.Duration
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		minute:                        make(map[string]*BucketRow),
		hour:                          make(map[string]*BucketRow),
		day:                           make(map[string]*BucketRow),
		deviceStateByProviderDeviceID: make(map[string]deviceState),
		maxSolarCarryForwardGap:       defaultSolarCarryForwardMaxGap,
	}
}

func (a *Aggregator) ApplySample(sample *rollupworker.RollupSample) {
	if a == nil || sample == nil {
		return
	}

	stateKey := sample.Provider + "|" + sample.ProviderDeviceID
	state := a.deviceStateByProviderDeviceID[stateKey]
	if state.hasLastEnvelopeAt && sample.EventTime.After(state.lastEnvelopeAt) && state.hasPV {
		a.integrateSolar(sample, state.lastEnvelopeAt, sample.EventTime, state.currentPV, state.lastPVAt)
	}

	if !state.hasLastEnvelopeAt || sample.EventTime.After(state.lastEnvelopeAt) {
		state.lastEnvelopeAt = sample.EventTime
		state.hasLastEnvelopeAt = true
	}

	if sample.Metrics.PV.Valid {
		state.lastPVAt = sample.EventTime
		state.hasLastPVAt = true
		if sample.Metrics.PV.Value > 0 {
			state.currentPV = sample.Metrics.PV.Value
			state.hasPV = true
		} else {
			state.currentPV = 0
			state.hasPV = false
		}
	}
	a.deviceStateByProviderDeviceID[stateKey] = state

	a.addPoint(a.minute, sample, sample.EventTime.Truncate(time.Minute))
	a.addPoint(a.hour, sample, sample.EventTime.Truncate(time.Hour))
	a.addPoint(a.day, sample, time.Date(sample.EventTime.Year(), sample.EventTime.Month(), sample.EventTime.Day(), 0, 0, 0, 0, time.UTC))
}

func (a *Aggregator) Rows(resolution Resolution) []BucketRow {
	if a == nil {
		return nil
	}
	var source map[string]*BucketRow
	switch resolution {
	case ResolutionMinute:
		source = a.minute
	case ResolutionHour:
		source = a.hour
	case ResolutionDay:
		source = a.day
	default:
		return nil
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]BucketRow, 0, len(keys))
	for _, key := range keys {
		if row := source[key]; row != nil {
			rows = append(rows, *row)
		}
	}
	return rows
}

func (a *Aggregator) addPoint(target map[string]*BucketRow, sample *rollupworker.RollupSample, bucketStart time.Time) {
	row := ensureBucketRow(target, sample, bucketStart)
	row.SampleCount++
	if row.FirstTsUnixMS == 0 || sample.EventUnixMs < row.FirstTsUnixMS {
		row.FirstTsUnixMS = sample.EventUnixMs
	}
	if sample.EventUnixMs > row.LastTsUnixMS {
		row.LastTsUnixMS = sample.EventUnixMs
	}
	if sample.Metrics.SOC.Valid {
		row.SOC.add(sample.Metrics.SOC.Value)
	}
	if sample.Metrics.ACIn.Valid {
		row.ACIn.add(sample.Metrics.ACIn.Value)
	}
	if sample.Metrics.PV.Valid {
		row.PV.add(sample.Metrics.PV.Value)
	}
	if sample.Metrics.DC.Valid {
		row.DC.add(sample.Metrics.DC.Value)
	}
	if sample.Metrics.Load.Valid {
		row.Load.add(sample.Metrics.Load.Value)
	}
	if sample.Metrics.Net.Valid {
		row.Net.add(sample.Metrics.Net.Value)
	}
	if sample.Metrics.Battery.Valid {
		row.Battery.add(sample.Metrics.Battery.Value)
	}
	if sample.Metrics.Temp.Valid {
		row.Temp.add(sample.Metrics.Temp.Value)
	}
}

func (a *Aggregator) integrateSolar(sample *rollupworker.RollupSample, start, end time.Time, pvWatts float64, lastPVAt time.Time) {
	if a == nil || sample == nil || !end.After(start) || pvWatts <= 0 {
		return
	}
	if a.maxSolarCarryForwardGap > 0 {
		maxEnd := lastPVAt.Add(a.maxSolarCarryForwardGap)
		if end.After(maxEnd) {
			end = maxEnd
		}
	}
	if !end.After(start) {
		return
	}
	a.integrateSolarAcrossBuckets(a.minute, sample, start, end, pvWatts, time.Minute)
	a.integrateSolarAcrossBuckets(a.hour, sample, start, end, pvWatts, time.Hour)
	a.integrateSolarAcrossBuckets(a.day, sample, start, end, pvWatts, 24*time.Hour)
}

func (a *Aggregator) integrateSolarAcrossBuckets(target map[string]*BucketRow, sample *rollupworker.RollupSample, start, end time.Time, pvWatts float64, bucketWidth time.Duration) {
	cursor := start
	for cursor.Before(end) {
		bucketStart := cursor.Truncate(bucketWidth)
		if bucketWidth == 24*time.Hour {
			bucketStart = time.Date(cursor.Year(), cursor.Month(), cursor.Day(), 0, 0, 0, 0, time.UTC)
		}
		segmentEnd := bucketStart.Add(bucketWidth)
		if segmentEnd.After(end) {
			segmentEnd = end
		}
		durationHours := segmentEnd.Sub(cursor).Hours()
		if durationHours > 0 {
			row := ensureBucketRow(target, sample, bucketStart)
			row.SolarGeneratedWh += pvWatts * durationHours
			row.HasSolarGeneratedWh = true
		}
		cursor = segmentEnd
	}
}

func ensureBucketRow(target map[string]*BucketRow, sample *rollupworker.RollupSample, bucketStart time.Time) *BucketRow {
	key := bucketMapKey(sample.Provider, sample.ProviderDeviceID, bucketStart)
	if row := target[key]; row != nil {
		return row
	}
	row := &BucketRow{
		Provider:         sample.Provider,
		ProviderDeviceID: sample.ProviderDeviceID,
		DeviceID:         sample.DeviceID,
		BucketStart:      bucketStart.UTC(),
	}
	target[key] = row
	return row
}

func bucketMapKey(provider, providerDeviceID string, bucketStart time.Time) string {
	return fmt.Sprintf("%s|%s|%d", provider, providerDeviceID, bucketStart.UTC().Unix())
}

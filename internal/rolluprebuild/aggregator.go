package rolluprebuild

import (
	"fmt"
	"sort"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
)

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

	SOC      metricAccumulator
	ACIn     metricAccumulator
	ACOutput metricAccumulator
	PV       metricAccumulator
	DC       metricAccumulator
	Load     metricAccumulator
	Net      metricAccumulator
	Battery  metricAccumulator
	Temp     metricAccumulator

	SolarGeneratedWh      float64
	HasSolarGeneratedWh   bool
	ACInputEnergyWh       float64
	HasACInputEnergyWh    bool
	ACOutputEnergyWh      float64
	HasACOutputEnergyWh   bool
	DCOutputEnergyWh      float64
	HasDCOutputEnergyWh   bool
	LoadEnergyWh          float64
	HasLoadEnergyWh       bool
	BatteryChargeWh       float64
	HasBatteryChargeWh    bool
	BatteryDischargeWh    float64
	HasBatteryDischargeWh bool
}

type PVPortBucketRow struct {
	Provider             string
	ProviderDeviceID     string
	DeviceID             string
	PortID               string
	PortLabel            string
	BucketStart          time.Time
	SampleCount          int
	FirstTsUnixMS        int64
	LastTsUnixMS         int64
	MaxObservedVolts     float64
	MaxObservedAmps      float64
	MaxObservedWatts     float64
	LastObservedVolts    float64
	LastObservedAmps     float64
	LastObservedWatts    float64
	LastObservedAtUnixMS int64
}

type powerState struct {
	lastAt    time.Time
	hasLastAt bool
	watts     float64
	hasWatts  bool
}

type deviceState struct {
	provider          string
	providerDeviceID  string
	deviceID          string
	lastEnvelopeAt    time.Time
	hasLastEnvelopeAt bool
	lastPVAt          time.Time
	hasLastPVAt       bool
	currentPV         float64
	hasPV             bool
	acIn              powerState
	acOutput          powerState
	dcOutput          powerState
	load              powerState
	batteryCharge     powerState
	batteryDischarge  powerState
}

type Aggregator struct {
	minute       map[string]*BucketRow
	hour         map[string]*BucketRow
	day          map[string]*BucketRow
	pvPortMinute map[string]*PVPortBucketRow
	pvPortHour   map[string]*PVPortBucketRow
	pvPortDay    map[string]*PVPortBucketRow

	deviceStateByProviderDeviceID map[string]deviceState
	maxSolarCarryForwardGap       time.Duration
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		minute:                        make(map[string]*BucketRow),
		hour:                          make(map[string]*BucketRow),
		day:                           make(map[string]*BucketRow),
		pvPortMinute:                  make(map[string]*PVPortBucketRow),
		pvPortHour:                    make(map[string]*PVPortBucketRow),
		pvPortDay:                     make(map[string]*PVPortBucketRow),
		deviceStateByProviderDeviceID: make(map[string]deviceState),
		maxSolarCarryForwardGap:       rollupworker.DefaultSolarCarryForwardMaxGap,
	}
}

func (a *Aggregator) ApplySample(sample *rollupworker.RollupSample) {
	if a == nil || sample == nil {
		return
	}

	stateKey := sample.Provider + "|" + sample.ProviderDeviceID
	state := a.deviceStateByProviderDeviceID[stateKey]
	state.provider = sample.Provider
	state.providerDeviceID = sample.ProviderDeviceID
	state.deviceID = sample.DeviceID
	if state.hasLastEnvelopeAt && sample.EventTime.After(state.lastEnvelopeAt) && state.hasPV {
		a.integrateSolar(sample, state.lastEnvelopeAt, sample.EventTime, state.currentPV, state.lastPVAt)
	}
	if state.hasLastEnvelopeAt && sample.EventTime.After(state.lastEnvelopeAt) {
		a.integrateEnergy(sample, state, state.lastEnvelopeAt, sample.EventTime)
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
	state.acIn = advancePowerState(state.acIn, sample.EventTime, sample.Metrics.ACIn.Value, sample.Metrics.ACIn.Valid)
	state.acOutput = advancePowerState(state.acOutput, sample.EventTime, sample.Metrics.ACOutput.Value, sample.Metrics.ACOutput.Valid)
	state.dcOutput = advancePowerState(state.dcOutput, sample.EventTime, sample.Metrics.DC.Value, sample.Metrics.DC.Valid)
	state.load = advancePowerState(state.load, sample.EventTime, sample.Metrics.Load.Value, sample.Metrics.Load.Valid)
	chargeValue, chargeValid := positiveMetricValue(sample.Metrics.Battery.Value, sample.Metrics.Battery.Valid)
	dischargeValue, dischargeValid := negativeMetricValue(sample.Metrics.Battery.Value, sample.Metrics.Battery.Valid)
	state.batteryCharge = advancePowerState(state.batteryCharge, sample.EventTime, chargeValue, chargeValid)
	state.batteryDischarge = advancePowerState(state.batteryDischarge, sample.EventTime, dischargeValue, dischargeValid)
	a.deviceStateByProviderDeviceID[stateKey] = state

	a.addPoint(a.minute, sample, sample.EventTime.Truncate(time.Minute))
	a.addPoint(a.hour, sample, sample.EventTime.Truncate(time.Hour))
	a.addPoint(a.day, sample, time.Date(sample.EventTime.Year(), sample.EventTime.Month(), sample.EventTime.Day(), 0, 0, 0, 0, time.UTC))
	a.addPVPortPoints(a.pvPortMinute, sample, sample.EventTime.Truncate(time.Minute))
	a.addPVPortPoints(a.pvPortHour, sample, sample.EventTime.Truncate(time.Hour))
	a.addPVPortPoints(a.pvPortDay, sample, time.Date(sample.EventTime.Year(), sample.EventTime.Month(), sample.EventTime.Day(), 0, 0, 0, 0, time.UTC))
}

func (a *Aggregator) Finalize(windowEnd time.Time) {
	if a == nil || windowEnd.IsZero() {
		return
	}
	windowEnd = windowEnd.UTC()
	for key, state := range a.deviceStateByProviderDeviceID {
		if !state.hasLastEnvelopeAt || !state.hasPV || state.provider == "" || state.providerDeviceID == "" || state.deviceID == "" {
			continue
		}
		end := windowEnd
		if a.maxSolarCarryForwardGap > 0 {
			maxEnd := state.lastPVAt.Add(a.maxSolarCarryForwardGap)
			if end.After(maxEnd) {
				end = maxEnd
			}
		}
		if !end.After(state.lastEnvelopeAt) {
			continue
		}
		sample := &rollupworker.RollupSample{
			Provider:         state.provider,
			ProviderDeviceID: state.providerDeviceID,
			DeviceID:         state.deviceID,
			EventTime:        end,
			EventUnixMs:      end.UnixMilli(),
		}
		a.integrateSolar(sample, state.lastEnvelopeAt, end, state.currentPV, state.lastPVAt)
		a.integrateEnergy(sample, state, state.lastEnvelopeAt, end)
		state.lastEnvelopeAt = end
		a.deviceStateByProviderDeviceID[key] = state
	}
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
	if sample.Metrics.ACOutput.Valid {
		row.ACOutput.add(sample.Metrics.ACOutput.Value)
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

func (a *Aggregator) PVPortRows(resolution Resolution) []PVPortBucketRow {
	if a == nil {
		return nil
	}
	var source map[string]*PVPortBucketRow
	switch resolution {
	case ResolutionMinute:
		source = a.pvPortMinute
	case ResolutionHour:
		source = a.pvPortHour
	case ResolutionDay:
		source = a.pvPortDay
	default:
		return nil
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]PVPortBucketRow, 0, len(keys))
	for _, key := range keys {
		if row := source[key]; row != nil {
			rows = append(rows, *row)
		}
	}
	return rows
}

func (a *Aggregator) addPVPortPoints(target map[string]*PVPortBucketRow, sample *rollupworker.RollupSample, bucketStart time.Time) {
	if sample == nil || len(sample.PVPorts) == 0 {
		return
	}
	for _, observation := range sample.PVPorts {
		key := fmt.Sprintf("%s|%s|%s|%s", sample.Provider, sample.ProviderDeviceID, observation.PortID, bucketStart.UTC().Format(time.RFC3339Nano))
		row := target[key]
		if row == nil {
			row = &PVPortBucketRow{
				Provider:         sample.Provider,
				ProviderDeviceID: sample.ProviderDeviceID,
				DeviceID:         sample.DeviceID,
				PortID:           observation.PortID,
				PortLabel:        observation.PortLabel,
				BucketStart:      bucketStart.UTC(),
			}
			target[key] = row
		}
		row.SampleCount++
		if row.FirstTsUnixMS == 0 || sample.EventUnixMs < row.FirstTsUnixMS {
			row.FirstTsUnixMS = sample.EventUnixMs
		}
		if sample.EventUnixMs > row.LastTsUnixMS {
			row.LastTsUnixMS = sample.EventUnixMs
		}
		if observation.Volts > row.MaxObservedVolts {
			row.MaxObservedVolts = observation.Volts
		}
		if observation.Amps > row.MaxObservedAmps {
			row.MaxObservedAmps = observation.Amps
		}
		if observation.Watts > row.MaxObservedWatts {
			row.MaxObservedWatts = observation.Watts
		}
		if sample.EventUnixMs >= row.LastObservedAtUnixMS {
			row.PortLabel = observation.PortLabel
			row.LastObservedVolts = observation.Volts
			row.LastObservedAmps = observation.Amps
			row.LastObservedWatts = observation.Watts
			row.LastObservedAtUnixMS = sample.EventUnixMs
		}
	}
}

func (a *Aggregator) integrateSolar(sample *rollupworker.RollupSample, start, end time.Time, pvWatts float64, lastPVAt time.Time) {
	if a == nil || sample == nil || !end.After(start) || pvWatts <= 0 {
		return
	}
	a.integrateSolarAcrossBuckets(a.minute, sample, start, end, pvWatts, lastPVAt, time.Minute, false)
	a.integrateSolarAcrossBuckets(a.hour, sample, start, end, pvWatts, lastPVAt, time.Hour, false)
	a.integrateSolarAcrossBuckets(a.day, sample, start, end, pvWatts, lastPVAt, 24*time.Hour, true)
}

func (a *Aggregator) integrateSolarAcrossBuckets(target map[string]*BucketRow, sample *rollupworker.RollupSample, start, end time.Time, pvWatts float64, lastPVAt time.Time, bucketWidth time.Duration, dayBucket bool) {
	rollupworker.IntegrateSolarWindow(start, end, lastPVAt, pvWatts, a.maxSolarCarryForwardGap, bucketWidth, dayBucket, func(bucketStart time.Time, _ time.Time, _ time.Time, wattHours float64) {
		row := ensureBucketRow(target, sample, bucketStart)
		row.SolarGeneratedWh += wattHours
		row.HasSolarGeneratedWh = true
	})
}

func (a *Aggregator) integrateEnergy(sample *rollupworker.RollupSample, state deviceState, start, end time.Time) {
	if a == nil || sample == nil || !end.After(start) {
		return
	}
	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.acIn, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.ACInputEnergyWh += wattHours
		row.HasACInputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.acIn, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.ACInputEnergyWh += wattHours
		row.HasACInputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.acIn, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.ACInputEnergyWh += wattHours
		row.HasACInputEnergyWh = true
	})

	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.acOutput, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.ACOutputEnergyWh += wattHours
		row.HasACOutputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.acOutput, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.ACOutputEnergyWh += wattHours
		row.HasACOutputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.acOutput, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.ACOutputEnergyWh += wattHours
		row.HasACOutputEnergyWh = true
	})

	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.dcOutput, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.DCOutputEnergyWh += wattHours
		row.HasDCOutputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.dcOutput, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.DCOutputEnergyWh += wattHours
		row.HasDCOutputEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.dcOutput, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.DCOutputEnergyWh += wattHours
		row.HasDCOutputEnergyWh = true
	})

	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.load, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.LoadEnergyWh += wattHours
		row.HasLoadEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.load, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.LoadEnergyWh += wattHours
		row.HasLoadEnergyWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.load, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.LoadEnergyWh += wattHours
		row.HasLoadEnergyWh = true
	})

	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.batteryCharge, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.BatteryChargeWh += wattHours
		row.HasBatteryChargeWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.batteryCharge, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.BatteryChargeWh += wattHours
		row.HasBatteryChargeWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.batteryCharge, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.BatteryChargeWh += wattHours
		row.HasBatteryChargeWh = true
	})

	a.integrateEnergyAcrossBuckets(a.minute, sample, start, end, state.batteryDischarge, time.Minute, false, func(row *BucketRow, wattHours float64) {
		row.BatteryDischargeWh += wattHours
		row.HasBatteryDischargeWh = true
	})
	a.integrateEnergyAcrossBuckets(a.hour, sample, start, end, state.batteryDischarge, time.Hour, false, func(row *BucketRow, wattHours float64) {
		row.BatteryDischargeWh += wattHours
		row.HasBatteryDischargeWh = true
	})
	a.integrateEnergyAcrossBuckets(a.day, sample, start, end, state.batteryDischarge, 24*time.Hour, true, func(row *BucketRow, wattHours float64) {
		row.BatteryDischargeWh += wattHours
		row.HasBatteryDischargeWh = true
	})
}

func (a *Aggregator) integrateEnergyAcrossBuckets(target map[string]*BucketRow, sample *rollupworker.RollupSample, start, end time.Time, state powerState, bucketWidth time.Duration, dayBucket bool, apply func(row *BucketRow, wattHours float64)) {
	if apply == nil || !state.hasWatts || !state.hasLastAt {
		return
	}
	rollupworker.IntegratePowerWindow(start, end, state.lastAt, state.watts, a.maxSolarCarryForwardGap, bucketWidth, dayBucket, func(bucketStart time.Time, _ time.Time, _ time.Time, wattHours float64) {
		row := ensureBucketRow(target, sample, bucketStart)
		apply(row, wattHours)
	})
}

func advancePowerState(state powerState, at time.Time, value float64, valid bool) powerState {
	if !valid {
		return state
	}
	state.lastAt = at
	state.hasLastAt = true
	if value > 0 {
		state.watts = value
		state.hasWatts = true
	} else {
		state.watts = 0
		state.hasWatts = false
	}
	return state
}

func positiveMetricValue(value float64, valid bool) (float64, bool) {
	if !valid || value <= 0 {
		return 0, true
	}
	return value, true
}

func negativeMetricValue(value float64, valid bool) (float64, bool) {
	if !valid || value >= 0 {
		return 0, true
	}
	return -value, true
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

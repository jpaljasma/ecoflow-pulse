package rollupworker

import (
	"errors"
	"time"
)

var (
	ErrInvalidRollupEnvelope = errors.New("invalid rollup envelope")
	ErrNoRollupMetrics       = errors.New("no rollup metrics")
)

type optionalFloat struct {
	Value float64
	Valid bool
}

func (o optionalFloat) sqlValue() any {
	if !o.Valid {
		return nil
	}
	return o.Value
}

type RollupMetrics struct {
	SOC              optionalFloat
	ACIn             optionalFloat
	ACOutput         optionalFloat
	PV               optionalFloat
	DC               optionalFloat
	Load             optionalFloat
	Net              optionalFloat
	Battery          optionalFloat
	Temp             optionalFloat
	SolarGeneratedWh optionalFloat
}

func (m RollupMetrics) HasAny() bool {
	return m.SOC.Valid ||
		m.ACIn.Valid ||
		m.ACOutput.Valid ||
		m.PV.Valid ||
		m.DC.Valid ||
		m.Load.Valid ||
		m.Net.Valid ||
		m.Battery.Valid ||
		m.Temp.Valid ||
		m.SolarGeneratedWh.Valid
}

type PVPortObservation struct {
	PortID    string
	PortLabel string
	Volts     float64
	Amps      float64
	Watts     float64
}

type RollupSample struct {
	Provider         string
	ProviderDeviceID string
	DeviceID         string
	EventTime        time.Time
	EventUnixMs      int64
	IngestedUnixMs   int64
	Metrics          RollupMetrics
	PVPorts          []PVPortObservation
}

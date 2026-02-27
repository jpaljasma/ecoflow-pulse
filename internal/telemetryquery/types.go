package telemetryquery

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidResolution = errors.New("invalid rollup resolution")
	ErrInvalidRange      = errors.New("invalid rollup range")
)

type Resolution int

const (
	ResolutionUnknown Resolution = iota
	ResolutionMinute
	ResolutionHour
	ResolutionDay
)

func (r Resolution) BucketDuration() time.Duration {
	switch r {
	case ResolutionMinute:
		return time.Minute
	case ResolutionHour:
		return time.Hour
	case ResolutionDay:
		return 24 * time.Hour
	default:
		return 0
	}
}

func (r Resolution) TableName() (string, error) {
	switch r {
	case ResolutionMinute:
		return "telemetry_rollup_minute", nil
	case ResolutionHour:
		return "telemetry_rollup_hour", nil
	case ResolutionDay:
		return "telemetry_rollup_day", nil
	default:
		return "", ErrInvalidResolution
	}
}

type Metrics struct {
	SOCAvgPct        *float64
	SOCMinPct        *float64
	SOCMaxPct        *float64
	ACInAvgW         *float64
	ACInMaxW         *float64
	PVAvgW           *float64
	PVMaxW           *float64
	DCAvgW           *float64
	DCMaxW           *float64
	LoadAvgW         *float64
	LoadMaxW         *float64
	NetAvgW          *float64
	NetMinW          *float64
	NetMaxW          *float64
	BatteryAvgW      *float64
	BatteryMinW      *float64
	BatteryMaxW      *float64
	TempAvgC         *float64
	TempMinC         *float64
	TempMaxC         *float64
	SolarGeneratedWh *float64
}

type Point struct {
	BucketStart   time.Time
	BucketEnd     time.Time
	SampleCount   uint64
	FirstTsUnixMs int64
	LastTsUnixMs  int64
	Metrics       Metrics
}

type Series struct {
	DeviceID   string
	Resolution Resolution
	From       time.Time
	To         time.Time
	Points     []Point
}

type RangeQuery struct {
	DeviceID   string
	Resolution Resolution
	From       time.Time
	To         time.Time
	Limit      int
}

type Reader interface {
	QueryRange(ctx context.Context, query RangeQuery) (Series, error)
	Close() error
}

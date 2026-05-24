package main

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"

	weatherv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/weather/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type WeatherService struct {
	weatherv1.UnimplementedWeatherServiceServer

	log     *slog.Logger
	service *weatherd.Service
}

type WeatherServiceDeps struct {
	Log     *slog.Logger
	Service *weatherd.Service
}

func NewWeatherServiceWithDeps(deps WeatherServiceDeps) *WeatherService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &WeatherService{
		log:     log,
		service: deps.Service,
	}
}

func (s *WeatherService) Get7DayForecast(ctx context.Context, req *weatherv1.Get7DayForecastRequest) (*weatherv1.Get7DayForecastResponse, error) {
	domainReq, err := requestFromProto(req.GetLocation())
	if err != nil {
		return nil, err
	}
	if s.service == nil {
		return nil, status.Error(codes.Unavailable, "weather service is not configured")
	}
	bundle, err := s.service.Get7DayForecast(ctx, domainReq)
	if err != nil {
		return nil, mapWeatherError(err)
	}
	return bundleToProto(bundle, domainReq.UnitSystem), nil
}

func requestFromProto(location *weatherv1.WeatherLocationRequest) (weatherd.Request, error) {
	if location == nil {
		return weatherd.Request{}, status.Error(codes.InvalidArgument, "location is required")
	}
	req := weatherd.Request{
		Latitude:            location.GetLatitude(),
		Longitude:           location.GetLongitude(),
		UnitSystem:          unitSystemFromProto(location.GetUnitSystem()),
		PanelTiltDegrees:    getWrappedFloat(location.GetPanelTiltDegrees()),
		PanelAzimuthDegrees: getWrappedFloat(location.GetPanelAzimuthDegrees()),
		Timezone:            strings.TrimSpace(location.GetTimezone()),
	}
	if !isFiniteInRange(req.Latitude, -90, 90) {
		return weatherd.Request{}, status.Error(codes.InvalidArgument, "latitude must be between -90 and 90")
	}
	if !isFiniteInRange(req.Longitude, -180, 180) {
		return weatherd.Request{}, status.Error(codes.InvalidArgument, "longitude must be between -180 and 180")
	}
	if req.UnitSystem == "" {
		req.UnitSystem = weatherd.UnitSystemMetric
	}
	return req.Normalized(), nil
}

func bundleToProto(bundle *weatherd.Bundle, unitSystem weatherd.UnitSystem) *weatherv1.Get7DayForecastResponse {
	if bundle == nil {
		return &weatherv1.Get7DayForecastResponse{}
	}
	out := &weatherv1.Get7DayForecastResponse{
		Provenance: &weatherv1.ForecastProvenance{
			Source:               bundle.Provenance.Source,
			ModelSelection:       bundle.Provenance.ModelSelection,
			ActualSource:         bundle.Provenance.ActualSource,
			Timezone:             bundle.Provenance.Timezone,
			CanonicalLocationKey: bundle.Provenance.CanonicalLocationKey,
			IssuedAtUnixMs:       bundle.Provenance.IssuedAt.UTC().UnixMilli(),
			Latitude:             bundle.Provenance.Latitude,
			Longitude:            bundle.Provenance.Longitude,
			Elevation:            bundle.Provenance.Elevation,
		},
		TimezoneAbbreviation: bundle.Provenance.TimezoneAbbreviation,
		UnitSystem:           unitSystemToProto(unitSystem),
		Hourly:               make([]*weatherv1.HourlyForecastPoint, 0, len(bundle.Hourly)),
		Daily:                make([]*weatherv1.DailyForecastPoint, 0, len(bundle.Daily)),
	}
	for _, point := range bundle.Hourly {
		out.Hourly = append(out.Hourly, &weatherv1.HourlyForecastPoint{
			TimeUnixMs: point.Time.UTC().UnixMilli(),
			Condition:  conditionToProto(point.Condition),
			Raw:        forecastValuesToProto(point.Raw),
			Corrected:  forecastValuesToProto(point.Corrected),
		})
	}
	for _, point := range bundle.Daily {
		out.Daily = append(out.Daily, &weatherv1.DailyForecastPoint{
			DateUnixMs:              point.Date.UTC().UnixMilli(),
			Condition:               conditionToProto(point.Condition),
			SunriseUnixMs:           point.Sunrise.UTC().UnixMilli(),
			SunsetUnixMs:            point.Sunset.UTC().UnixMilli(),
			DaylightDurationSeconds: wrapFloat(point.DaylightDurationSeconds),
			Raw:                     dailyValuesToProto(point.Raw),
			Corrected:               dailyValuesToProto(point.Corrected),
		})
	}
	return out
}

func forecastValuesToProto(values weatherd.ForecastValueSet) *weatherv1.ForecastValueSet {
	return &weatherv1.ForecastValueSet{
		Temperature:             wrapFloat(values.Temperature),
		WindSpeed:               wrapFloat(values.WindSpeed),
		WindDirectionDegrees:    wrapFloat(values.WindDirectionDegrees),
		Precipitation:           wrapFloat(values.Precipitation),
		CloudCover:              wrapFloat(values.CloudCover),
		Visibility:              wrapFloat(values.Visibility),
		SunshineDurationSeconds: wrapFloat(values.SunshineDurationSeconds),
		ShortwaveRadiation:      wrapFloat(values.ShortwaveRadiation),
		UvIndex:                 wrapFloat(values.UVIndex),
		GlobalTiltedIrradiance:  wrapFloat(values.GlobalTiltedIrradiance),
	}
}

func dailyValuesToProto(values weatherd.DailyValueSet) *weatherv1.DailyValueSet {
	return &weatherv1.DailyValueSet{
		SunshineDurationSeconds: wrapFloat(values.SunshineDurationSeconds),
		ShortwaveRadiationSum:   wrapFloat(values.ShortwaveRadiationSum),
		UvIndexMax:              wrapFloat(values.UVIndexMax),
	}
}

func conditionToProto(condition weatherd.WeatherCondition) *weatherv1.WeatherCondition {
	return &weatherv1.WeatherCondition{
		WeatherCode: condition.WeatherCode,
		WeatherText: condition.WeatherText,
	}
}

func unitSystemFromProto(unitSystem weatherv1.UnitSystem) weatherd.UnitSystem {
	switch unitSystem {
	case weatherv1.UnitSystem_UNIT_SYSTEM_IMPERIAL:
		return weatherd.UnitSystemImperial
	default:
		return weatherd.UnitSystemMetric
	}
}

func unitSystemToProto(unitSystem weatherd.UnitSystem) weatherv1.UnitSystem {
	switch unitSystem {
	case weatherd.UnitSystemImperial:
		return weatherv1.UnitSystem_UNIT_SYSTEM_IMPERIAL
	default:
		return weatherv1.UnitSystem_UNIT_SYSTEM_METRIC
	}
}

func wrapFloat(value *float64) *wrapperspb.DoubleValue {
	if value == nil {
		return nil
	}
	return wrapperspb.Double(*value)
}

func getWrappedFloat(value *wrapperspb.DoubleValue) *float64 {
	if value == nil {
		return nil
	}
	v := value.GetValue()
	return &v
}

func mapWeatherError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, weatherd.ErrUpstreamBudgetExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Errorf(codes.Internal, "weather request failed: %v", err)
	}
}

func isFiniteInRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}

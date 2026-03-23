package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	solarforecastv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/solarforecast/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SolarForecastService struct {
	solarforecastv1.UnimplementedSolarForecastServiceServer

	log               *slog.Logger
	service           *solarforecastd.Service
	controlPlaneStore controlplane.Store
}

type SolarForecastServiceDeps struct {
	Log               *slog.Logger
	Service           *solarforecastd.Service
	ControlPlaneStore controlplane.Store
}

func NewSolarForecastServiceWithDeps(deps SolarForecastServiceDeps) *SolarForecastService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &SolarForecastService{
		log:               log,
		service:           deps.Service,
		controlPlaneStore: deps.ControlPlaneStore,
	}
}

func (s *SolarForecastService) GetSolarOutlook(ctx context.Context, req *solarforecastv1.GetSolarOutlookRequest) (*solarforecastv1.GetSolarOutlookResponse, error) {
	if s.service == nil {
		return nil, status.Error(codes.Unavailable, "solar forecast service is not configured")
	}
	weatherReq, err := requestFromProto(req.GetLocation())
	if err != nil {
		return nil, err
	}
	deviceIDs, err := s.resolveVisibleDeviceIDs(ctx, strings.TrimSpace(req.GetDeviceId()), req.GetUseAllDevices())
	if err != nil {
		return nil, err
	}
	outlook, err := s.service.GetSolarOutlook(ctx, solarforecastd.Input{
		WeatherRequest:    weatherReq,
		ResolvedDeviceIDs: deviceIDs,
		Scope: solarforecastd.Scope{
			Mode:     solarScopeMode(req.GetUseAllDevices()),
			DeviceID: strings.TrimSpace(req.GetDeviceId()),
		},
	})
	if err != nil {
		return nil, mapSolarForecastError(err)
	}
	return solarOutlookToProto(outlook), nil
}

func (s *SolarForecastService) resolveVisibleDeviceIDs(ctx context.Context, requestedDeviceID string, useAllDevices bool) ([]string, error) {
	requestedDeviceID = strings.TrimSpace(requestedDeviceID)
	if !useAllDevices {
		if requestedDeviceID == "" {
			return nil, status.Error(codes.InvalidArgument, "device_id required when use_all_devices is false")
		}
		if err := authorizeDeviceAccess(ctx, s.controlPlaneStore, requestedDeviceID); err != nil {
			return nil, err
		}
		return []string{requestedDeviceID}, nil
	}
	if s.controlPlaneStore == nil {
		return nil, status.Error(codes.InvalidArgument, "all-device solar scope unavailable without user device store")
	}
	claims, ok := grpcmw.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil, status.Error(codes.InvalidArgument, "all-device solar scope requires user context")
	}
	rows, err := s.controlPlaneStore.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: claims.Subject})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list visible solar devices: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DeviceID) != "" {
			out = append(out, row.DeviceID)
		}
	}
	if len(out) == 0 {
		return nil, status.Error(codes.NotFound, "no visible devices available for solar forecast")
	}
	return out, nil
}

func solarOutlookToProto(outlook *solarforecastd.Outlook) *solarforecastv1.GetSolarOutlookResponse {
	if outlook == nil {
		return &solarforecastv1.GetSolarOutlookResponse{}
	}
	out := &solarforecastv1.GetSolarOutlookResponse{
		Scope: &solarforecastv1.SolarForecastScope{
			Mode:              outlook.Scope.Mode,
			DeviceId:          outlook.Scope.DeviceID,
			ResolvedDeviceIds: append([]string(nil), outlook.Scope.ResolvedDeviceIDs...),
		},
		Provenance: &solarforecastv1.SolarForecastProvenance{
			ForecastSource:             outlook.Provenance.ForecastSource,
			ForecastModel:              outlook.Provenance.ForecastModel,
			ServedVariant:              outlook.Provenance.ServedVariant,
			BaselineModel:              outlook.Provenance.BaselineModel,
			CalibrationApplied:         outlook.Provenance.CalibrationApplied,
			CalibrationSampleCount:     int32(outlook.Provenance.CalibrationSampleCount),
			CalibrationUpdatedAtUnixMs: timeToUnixMs(outlook.Provenance.CalibrationUpdatedAt),
			SameDayCurtailmentApplied:  outlook.Provenance.SameDayCurtailmentApplied,
			SameDayCurtailmentReason:   outlook.Provenance.SameDayCurtailmentReason,
			ActualsSource:              outlook.Provenance.ActualsSource,
			WeatherSource:              outlook.Provenance.WeatherSource,
			WeatherModelSelection:      outlook.Provenance.WeatherModelSelection,
			Timezone:                   outlook.Provenance.Timezone,
			CanonicalLocationKey:       outlook.Provenance.CanonicalLocationKey,
			IssuedAtUnixMs:             outlook.Provenance.IssuedAt.UTC().UnixMilli(),
			RefreshedAtUnixMs:          outlook.Provenance.RefreshedAt.UTC().UnixMilli(),
		},
		Capacity:     capacityToProto(outlook.Capacity),
		Today:        generationDayToProto(outlook.Today),
		Next_7Days:   make([]*solarforecastv1.SolarGenerationDay, 0, len(outlook.Next7Days)),
		Next_24Hours: make([]*solarforecastv1.SolarGenerationPoint, 0, len(outlook.Next24Hours)),
	}
	for _, day := range outlook.Next7Days {
		out.Next_7Days = append(out.Next_7Days, generationDayToProto(day))
	}
	for _, point := range outlook.Next24Hours {
		out.Next_24Hours = append(out.Next_24Hours, generationPointToProto(point))
	}
	return out
}

func capacityToProto(capacity solarforecastd.CapacityEstimate) *solarforecastv1.SolarCapacityEstimate {
	return &solarforecastv1.SolarCapacityEstimate{
		EstimatedPeakWatts: wrapFloat(capacity.EstimatedPeakWatts),
		ObservedPvWatts:    wrapFloat(capacity.ObservedPvWatts),
		Method:             capacity.Method,
	}
}

func generationDayToProto(day solarforecastd.GenerationDay) *solarforecastv1.SolarGenerationDay {
	return &solarforecastv1.SolarGenerationDay{
		DateUnixMs:           day.Date.UTC().UnixMilli(),
		ActualGeneratedKwh:   wrapFloat(day.ActualGeneratedKWh),
		ForecastRemainingKwh: wrapFloat(day.ForecastRemainingKWh),
		ForecastTotalKwh:     wrapFloat(day.ForecastTotalKWh),
		EstimatedPeakWatts:   wrapFloat(day.EstimatedPeakWatts),
		PeakTimeUnixMs:       timeToUnixMs(day.PeakTime),
		Confidence:           confidenceToProto(day.Confidence),
	}
}

func generationPointToProto(point solarforecastd.GenerationPoint) *solarforecastv1.SolarGenerationPoint {
	return &solarforecastv1.SolarGenerationPoint{
		TimeUnixMs:             point.Time.UTC().UnixMilli(),
		ActualGeneratedWh:      wrapFloat(point.ActualGeneratedWh),
		ForecastGeneratedWh:    wrapFloat(point.ForecastGeneratedWh),
		EstimatedPeakWatts:     wrapFloat(point.EstimatedPeakWatts),
		ShortwaveRadiation:     wrapFloat(point.ShortwaveRadiation),
		GlobalTiltedIrradiance: wrapFloat(point.GlobalTiltedIrradiance),
		CloudCover:             wrapFloat(point.CloudCover),
		Confidence:             confidenceToProto(point.Confidence),
	}
}

func confidenceToProto(value solarforecastd.Confidence) solarforecastv1.SolarForecastConfidence {
	switch value {
	case solarforecastd.ConfidenceHigh:
		return solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_HIGH
	case solarforecastd.ConfidenceMedium:
		return solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_MEDIUM
	default:
		return solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_LOW
	}
}

func timeToUnixMs(value *time.Time) int64 {
	if value == nil || value.IsZero() {
		return 0
	}
	return value.UTC().UnixMilli()
}

func mapSolarForecastError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, solarforecastd.ErrWeatherUnavailable), errors.Is(err, solarforecastd.ErrTelemetryUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, solarforecastd.ErrNoVisibleDevices):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "solar forecast request failed: %v", err)
	}
}

func solarScopeMode(useAllDevices bool) string {
	if useAllDevices {
		return "all"
	}
	return "device"
}

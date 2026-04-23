package solarforecastd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

var (
	ErrWeatherUnavailable   = errors.New("solar forecast weather source is not configured")
	ErrTelemetryUnavailable = errors.New("solar forecast telemetry source is not configured")
	ErrNoVisibleDevices     = errors.New("solar forecast scope requires at least one visible device")
)

const (
	baseSystemEfficiencyFactor    = 0.83
	maxForecastPeakOutputScale    = 0.98
	minTodayProgressScaleHour     = 11
	minTodayProgressForecastWh    = 800.0
	minTodayProgressScale         = 0.50
	maxTodayProgressScale         = 1.00
	minTodayTrailingScaleHour     = 14
	minTodayTrailingForecastWh    = 250.0
	minTodayTrailingScale         = 0.35
	todayTrailingWindowHours      = 2
	defaultTrainingPersistTimeout = 10 * time.Second
	saturatedPotentialUpliftScale = 1.12
	minQualifiedSaturatedHours    = 3
	minQualifiedSaturatedDays     = 2
	minSaturatedPotentialWatts    = 150.0
	minSaturatedRelativeStrength  = 0.85
	minSaturatedChargeEnergyWh    = 40.0
	maxSaturatedChargeEnergyRatio = 0.15
	deviceShareLookbackDays       = 7
	minDeviceShareDayWh           = 250.0
	deviceShareRecentDayWeight    = 0.35
)

const sameDayCurtailmentReasonBatteryNearFull = "battery_near_full"

type potentialEvidence struct {
	baseEnvelopeW           float64
	saturatedEnvelopeW      float64
	finalEnvelopeW          float64
	qualifiedSaturatedDays  int
	qualifiedSaturatedHours int
}

type WeatherForecaster interface {
	Get7DayForecast(ctx context.Context, req weatherd.Request) (*weatherd.Bundle, error)
}

type aggregateTelemetryReader interface {
	QueryRangeMany(ctx context.Context, query telemetryquery.AggregateRangeQuery) (telemetryquery.Series, error)
}

type Config struct {
	Log                    *slog.Logger
	Store                  TrainingStore
	Metrics                *Metrics
	NowFn                  func() time.Time
	PersistTrainingInline  bool
	TrainingPersistTimeout time.Duration
}

type Service struct {
	weather                WeatherForecaster
	query                  telemetryquery.Reader
	log                    *slog.Logger
	store                  TrainingStore
	metrics                *Metrics
	nowFn                  func() time.Time
	persistTrainingInline  bool
	trainingPersistTimeout time.Duration
}

func NewService(weather WeatherForecaster, query telemetryquery.Reader, cfg Config) (*Service, error) {
	nowFn := cfg.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	persistTimeout := cfg.TrainingPersistTimeout
	if persistTimeout <= 0 {
		persistTimeout = defaultTrainingPersistTimeout
	}
	return &Service{
		weather:                weather,
		query:                  query,
		log:                    log,
		store:                  cfg.Store,
		metrics:                cfg.Metrics,
		nowFn:                  nowFn,
		persistTrainingInline:  cfg.PersistTrainingInline,
		trainingPersistTimeout: persistTimeout,
	}, nil
}

func (s *Service) GetSolarOutlook(ctx context.Context, in Input) (outlook *Outlook, err error) {
	startedAt := s.nowFn()
	requestStartedAt := time.Now()
	scopeMode := normalizedScopeMode(in.Scope.Mode, len(in.ResolvedDeviceIDs))
	stageTimings := solarForecastStageTimings{}
	defer func() {
		if s.metrics != nil {
			s.metrics.ObserveRequest(scopeMode, err, s.nowFn().Sub(startedAt))
		}
		s.logSolarStageTimings(ctx, scopeMode, err, stageTimings, time.Since(requestStartedAt))
	}()
	if s.weather == nil {
		return nil, ErrWeatherUnavailable
	}
	if s.query == nil {
		return nil, ErrTelemetryUnavailable
	}
	deviceIDs := normalizedDeviceIDs(in.ResolvedDeviceIDs)
	if len(deviceIDs) == 0 {
		return nil, ErrNoVisibleDevices
	}
	siteDeviceIDs := normalizedDeviceIDs(in.SiteResolvedDeviceIDs)
	if len(siteDeviceIDs) == 0 {
		siteDeviceIDs = append([]string(nil), deviceIDs...)
	}
	canonicalSiteDeviceIDs := append([]string(nil), siteDeviceIDs...)
	useDeviceSiteAllocation := scopeMode == "device" && len(deviceIDs) == 1 && len(siteDeviceIDs) > 1

	weatherReq := in.WeatherRequest.Normalized()
	weatherReq.UnitSystem = weatherd.UnitSystemMetric
	weatherFetchStartedAt := time.Now()
	bundle, err := s.weather.Get7DayForecast(ctx, weatherReq)
	stageTimings.weatherFetch = time.Since(weatherFetchStartedAt)
	s.observeSolarStageTiming(scopeMode, solarForecastStageWeatherFetch, err, stageTimings.weatherFetch)
	if err != nil {
		return nil, err
	}
	if bundle == nil {
		return nil, ErrWeatherUnavailable
	}

	loc := loadLocation(bundle.Provenance.Timezone)
	nowUTC := s.nowFn().UTC()
	nowLocal := nowUTC.In(loc)
	todayStartLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	todayISO := localDateISO(nowLocal, loc)
	telemetryLookbackStartedAt := time.Now()
	todayHistory, err := s.querySeries(ctx, deviceIDs, todayStartLocal.UTC(), nowUTC, 32)
	stageTimings.telemetryLookback = time.Since(telemetryLookbackStartedAt)
	s.observeSolarStageTiming(scopeMode, solarForecastStageTelemetryLookback, err, stageTimings.telemetryLookback)
	if err != nil {
		return nil, err
	}
	actualTodayWh := todayActualWh(todayHistory.Points, loc, todayISO)
	forecastHistory := todayHistory
	forecastActualTodayWh := actualTodayWh
	if useDeviceSiteAllocation {
		telemetryLookbackStartedAt = time.Now()
		forecastHistory, err = s.querySeries(ctx, siteDeviceIDs, todayStartLocal.UTC(), nowUTC, 32)
		stageTimings.telemetryLookback += time.Since(telemetryLookbackStartedAt)
		if err != nil {
			return nil, err
		}
		forecastActualTodayWh = todayActualWh(forecastHistory.Points, loc, todayISO)
	}
	siteKey := buildSiteKey(bundle.Provenance.CanonicalLocationKey, canonicalSiteDeviceIDs)
	forecastModel := "deterministic_baseline_v1"
	servingState, err := s.loadServingState(ctx, siteKey, forecastModel)
	if err != nil {
		return nil, err
	}
	capacity := inferCapacityEstimateFromServingState(servingState, forecastHistory.Points, currentWeatherPoint(bundle.Hourly, nowUTC), loc)
	calibrationLoadsStartedAt := time.Now()
	calibrationStates, err := s.loadCalibrationStates(ctx, siteKey, forecastModel)
	stageTimings.calibrationLoads = time.Since(calibrationLoadsStartedAt)
	s.observeSolarStageTiming(scopeMode, solarForecastStageCalibrationLoads, err, stageTimings.calibrationLoads)
	if err != nil {
		return nil, err
	}
	recentSiteCalibration := recentSiteCalibrationFromServingState(servingState)
	calibrationIndex := BuildCalibrationIndex(calibrationStates)
	calibrationApplied := hasUsableCalibration(calibrationStates) || recentSiteCalibration.MultiplicativeRatio != nil
	calibrationSampleCount, calibrationUpdatedAt := summarizeCalibration(calibrationStates, recentSiteCalibration)
	if s.metrics != nil {
		s.metrics.ObserveModel(calibrationModeLabel(calibrationApplied))
	}
	summarizationStartedAt := time.Now()
	todayRemainingScale, todayCurtailmentReason := deriveTodayRemainingScale(forecastHistory, bundle.Hourly, capacity.EstimatedPeakWatts, forecastActualTodayWh, todayISO, nowUTC, loc, calibrationIndex, recentSiteCalibration.MultiplicativeRatio)
	daily := summarizeDailyOutlook(bundle.Hourly, bundle.Daily, capacity.EstimatedPeakWatts, forecastActualTodayWh, todayISO, todayRemainingScale, nowUTC, loc, calibrationIndex, recentSiteCalibration.MultiplicativeRatio)
	next24 := summarizeNext24Hours(bundle.Hourly, capacity.EstimatedPeakWatts, todayISO, todayRemainingScale, nowUTC, loc, calibrationIndex, recentSiteCalibration.MultiplicativeRatio)
	today := firstDayForISO(daily, todayISO)
	persistCapacity := capacity
	persistDaily := cloneGenerationDays(daily)
	persistNext24 := cloneGenerationPoints(next24)
	persistToday := firstDayForISO(persistDaily, todayISO)
	if useDeviceSiteAllocation {
		shareLookbackStartLocal := todayStartLocal.AddDate(0, 0, -deviceShareLookbackDays)
		deviceShareHistory, shareErr := s.queryLookbackSeries(ctx, deviceIDs, shareLookbackStartLocal.UTC(), nowUTC)
		if shareErr != nil {
			return nil, shareErr
		}
		siteShareHistory, shareErr := s.queryLookbackSeries(ctx, siteDeviceIDs, shareLookbackStartLocal.UTC(), nowUTC)
		if shareErr != nil {
			return nil, shareErr
		}
		share := deriveDeviceAllocationShare(deviceShareHistory.Points, siteShareHistory.Points, loc, todayISO, defaultDeviceAllocationShare(len(siteDeviceIDs)))
		capacity = allocateCapacityEstimate(capacity, deviceShareHistory.Points, share)
		daily = allocateGenerationDays(daily, actualTodayWh, share, todayISO)
		next24 = allocateGenerationPoints(next24, share)
		today = firstDayForISO(daily, todayISO)
	}
	stageTimings.summarization = time.Since(summarizationStartedAt)
	s.observeSolarStageTiming(scopeMode, solarForecastStageSummarization, nil, stageTimings.summarization)

	outlook = &Outlook{
		Scope: Scope{
			Mode:              scopeMode,
			DeviceID:          strings.TrimSpace(in.Scope.DeviceID),
			ResolvedDeviceIDs: append([]string(nil), deviceIDs...),
		},
		Provenance: Provenance{
			ForecastSource:            "solarforecastd",
			ForecastModel:             forecastModel,
			ServedVariant:             calibrationModeLabel(calibrationApplied),
			BaselineModel:             forecastModel,
			CalibrationApplied:        calibrationApplied,
			CalibrationSampleCount:    calibrationSampleCount,
			CalibrationUpdatedAt:      calibrationUpdatedAt,
			SameDayCurtailmentApplied: todayCurtailmentReason != "",
			SameDayCurtailmentReason:  todayCurtailmentReason,
			ActualsSource:             "telemetry_rollups",
			WeatherSource:             bundle.Provenance.Source,
			WeatherModelSelection:     bundle.Provenance.ModelSelection,
			Timezone:                  bundle.Provenance.Timezone,
			CanonicalLocationKey:      bundle.Provenance.CanonicalLocationKey,
			IssuedAt:                  bundle.Provenance.IssuedAt.UTC(),
			RefreshedAt:               nowUTC,
		},
		Capacity:    capacity,
		Today:       today,
		Next7Days:   daily,
		Next24Hours: next24,
	}
	if s.metrics != nil {
		s.metrics.ObserveConfidence(outlook)
		s.metrics.ObserveServedOutlook(siteKey, outlook, nowUTC)
	}
	trainingOutlook := canonicalTrainingOutlook(outlook, canonicalSiteDeviceIDs)
	if useDeviceSiteAllocation {
		trainingOutlook = canonicalTrainingOutlook(&Outlook{
			Scope: Scope{
				Mode:              "all",
				ResolvedDeviceIDs: append([]string(nil), canonicalSiteDeviceIDs...),
			},
			Provenance:  outlook.Provenance,
			Capacity:    persistCapacity,
			Today:       persistToday,
			Next7Days:   persistDaily,
			Next24Hours: persistNext24,
		}, canonicalSiteDeviceIDs)
	}
	trainingKickoffStartedAt := time.Now()
	s.persistTrainingRunBestEffort(ctx, in, bundle, forecastHistory, trainingOutlook, forecastActualTodayWh, nowUTC, nowLocal, calibrationIndex, recentSiteCalibration.MultiplicativeRatio, todayRemainingScale)
	stageTimings.trainingKickoff = time.Since(trainingKickoffStartedAt)
	s.observeSolarStageTiming(scopeMode, solarForecastStageTrainingKickoff, nil, stageTimings.trainingKickoff)
	return outlook, nil
}

func (s *Service) persistTrainingRunBestEffort(
	ctx context.Context,
	in Input,
	bundle *weatherd.Bundle,
	history telemetryquery.Series,
	outlook *Outlook,
	actualTodayWh float64,
	nowUTC time.Time,
	nowLocal time.Time,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
	todayRemainingScale float64,
) {
	if s == nil || s.store == nil || bundle == nil || outlook == nil {
		return
	}
	persistFn := func(persistCtx context.Context) {
		if persistErr := s.persistTrainingRun(
			persistCtx,
			in,
			bundle,
			history,
			outlook,
			actualTodayWh,
			nowUTC,
			nowLocal,
			calibrationIndex,
			siteCalibrationRatio,
			todayRemainingScale,
		); persistErr != nil {
			s.log.Warn("solar forecast training persistence failed", "error", persistErr.Error())
		}
	}
	if s.persistTrainingInline {
		persistFn(ctx)
		return
	}
	go func() {
		baseCtx := context.Background()
		if ctx != nil {
			baseCtx = context.WithoutCancel(ctx)
		}
		persistCtx, cancel := context.WithTimeout(baseCtx, s.trainingPersistTimeout)
		defer cancel()
		persistFn(persistCtx)
	}()
}

func (s *Service) queryLookbackSeries(ctx context.Context, deviceIDs []string, fromUTC, toUTC time.Time) (telemetryquery.Series, error) {
	return s.querySeries(ctx, deviceIDs, fromUTC, toUTC, 24*8)
}

func (s *Service) querySeries(ctx context.Context, deviceIDs []string, fromUTC, toUTC time.Time, limit int) (telemetryquery.Series, error) {
	if limit <= 0 {
		limit = hourRangeLimit(fromUTC, toUTC)
	}
	if len(deviceIDs) == 1 {
		return s.query.QueryRange(ctx, telemetryquery.RangeQuery{
			DeviceID:   deviceIDs[0],
			Resolution: telemetryquery.ResolutionHour,
			From:       fromUTC,
			To:         toUTC,
			Limit:      limit,
		})
	}
	if aggregate, ok := s.query.(aggregateTelemetryReader); ok {
		return aggregate.QueryRangeMany(ctx, telemetryquery.AggregateRangeQuery{
			DeviceIDs:   deviceIDs,
			AggregateID: strings.Join(deviceIDs, ","),
			Resolution:  telemetryquery.ResolutionHour,
			From:        fromUTC,
			To:          toUTC,
			Limit:       limit,
		})
	}

	seriesByDevice := make([]telemetryquery.Series, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		series, err := s.query.QueryRange(ctx, telemetryquery.RangeQuery{
			DeviceID:   deviceID,
			Resolution: telemetryquery.ResolutionHour,
			From:       fromUTC,
			To:         toUTC,
			Limit:      limit,
		})
		if err != nil {
			return telemetryquery.Series{}, err
		}
		seriesByDevice = append(seriesByDevice, series)
	}
	return aggregateSeries(seriesByDevice, fromUTC, toUTC), nil
}

func (s *Service) VerifyIssuedForecasts(ctx context.Context, before time.Time, limit int) error {
	if s == nil || s.store == nil || s.query == nil {
		return nil
	}
	cutoff := before.UTC().Truncate(time.Hour)
	if cutoff.IsZero() {
		cutoff = s.nowFn().UTC().Truncate(time.Hour)
	}
	if limit <= 0 {
		limit = 24 * 64
	}
	if claimer, ok := s.store.(PendingRunClaimer); ok {
		return s.verifyClaimedRuns(ctx, cutoff, limit, claimer)
	}
	pending, err := s.store.ListPendingHourlyRecords(ctx, cutoff, limit)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveVerificationRun(err)
		}
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	var verifyErrs []error
	for _, batch := range groupPendingByRun(pending) {
		if err := s.verifyRunBatch(ctx, batch, cutoff); err != nil {
			verifyErrs = append(verifyErrs, err)
			if s.metrics != nil {
				s.metrics.ObserveVerificationRun(err)
			}
			continue
		}
		if s.metrics != nil {
			s.metrics.ObserveVerificationRun(nil)
		}
	}
	return errors.Join(verifyErrs...)
}

func (s *Service) verifyClaimedRuns(ctx context.Context, cutoff time.Time, limit int, claimer PendingRunClaimer) error {
	processedRows := 0
	var verifyErrs []error
	for processedRows < limit {
		claim, err := claimer.ClaimPendingRun(ctx, cutoff)
		if err != nil {
			if s.metrics != nil {
				s.metrics.ObserveVerificationRun(err)
			}
			return err
		}
		if claim == nil {
			break
		}
		if len(claim.Rows) == 0 {
			if rollbackErr := claim.Rollback(); rollbackErr != nil {
				verifyErrs = append(verifyErrs, rollbackErr)
			}
			break
		}
		processedRows += len(claim.Rows)
		if err := s.verifyClaimedRunBatch(ctx, claim, cutoff); err != nil {
			verifyErrs = append(verifyErrs, err)
			if rollbackErr := claim.Rollback(); rollbackErr != nil {
				verifyErrs = append(verifyErrs, rollbackErr)
			}
			if s.metrics != nil {
				s.metrics.ObserveVerificationRun(err)
			}
			continue
		}
		if err := claim.Commit(); err != nil {
			verifyErrs = append(verifyErrs, err)
			if s.metrics != nil {
				s.metrics.ObserveVerificationRun(err)
			}
			continue
		}
		if s.metrics != nil {
			s.metrics.ObserveVerificationRun(nil)
		}
	}
	return errors.Join(verifyErrs...)
}

func (s *Service) verifyRunBatch(ctx context.Context, batch []HourlyTrainingRecord, verifiedAt time.Time) error {
	if len(batch) == 0 {
		return nil
	}
	run, err := s.store.GetRun(ctx, batch[0].RunID)
	if err != nil {
		return err
	}
	return s.verifyRunBatchWithStore(ctx, s.store, run, batch, verifiedAt)
}

func (s *Service) verifyClaimedRunBatch(ctx context.Context, claim *PendingRunClaim, verifiedAt time.Time) error {
	if claim == nil {
		return nil
	}
	return s.verifyRunBatchWithStore(ctx, claim.Store, claim.Run, claim.Rows, verifiedAt)
}

func (s *Service) verifyRunBatchWithStore(ctx context.Context, store VerificationMutationStore, run *Run, batch []HourlyTrainingRecord, verifiedAt time.Time) error {
	if len(batch) == 0 || store == nil || run == nil {
		return nil
	}
	deviceIDs := deviceIDsFromRun(run)
	if len(deviceIDs) == 0 {
		return ErrNoVisibleDevices
	}
	fromUTC, toUTC := targetBounds(batch)
	series, err := s.querySeries(ctx, deviceIDs, fromUTC, toUTC, hourRangeLimit(fromUTC, toUTC))
	if err != nil {
		return err
	}
	actualByTime := telemetryByTime(series.Points)
	verifiedRows := make([]HourlyTrainingRecord, 0, len(batch))
	affectedDates := make(map[string]time.Time, len(batch))
	for _, row := range batch {
		updated := row
		updated.VerifiedAt = timePtr(verifiedAt)
		updated.UpdatedAt = verifiedAt
		if point, ok := actualByTime[row.TargetTime.UTC()]; ok && point.Metrics.SolarGeneratedWh != nil {
			actual := *point.Metrics.SolarGeneratedWh
			signed := row.ForecastGenerationWh - actual
			abs := math.Abs(signed)
			squared := signed * signed
			var baselineAbs, baselineSquared *float64
			if row.BaselineForecastGenerationWh != nil {
				baselineSigned := *row.BaselineForecastGenerationWh - actual
				baselineAbs = floatPtr(round1(math.Abs(baselineSigned)))
				baselineSquared = floatPtr(round1(baselineSigned * baselineSigned))
			}
			updated.ActualGenerationWh = floatPtr(actual)
			updated.SignedErrorWh = floatPtr(round1(signed))
			updated.AbsoluteErrorWh = floatPtr(round1(abs))
			updated.SquaredErrorWh2 = floatPtr(round1(squared))
			updated.BaselineAbsoluteErrorWh = baselineAbs
			updated.BaselineSquaredErrorWh2 = baselineSquared
			updated.VerificationStatus = VerificationStatusVerified
		} else {
			updated.ActualGenerationWh = nil
			updated.SignedErrorWh = nil
			updated.AbsoluteErrorWh = nil
			updated.SquaredErrorWh2 = nil
			updated.BaselineAbsoluteErrorWh = nil
			updated.BaselineSquaredErrorWh2 = nil
			updated.VerificationStatus = VerificationStatusMissingTruth
		}
		verifiedRows = append(verifiedRows, updated)
		affectedDates[row.TargetLocalDate.Format("2006-01-02")] = row.TargetLocalDate
	}
	if err := store.CompleteHourlyVerification(ctx, verifiedRows); err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.ObserveVerificationRows(verifiedRows, run.ServedVariant)
	}
	if err := s.applyCalibrationRows(ctx, store, run.SiteKey, run.ForecastVersion, verifiedAt, verifiedRows); err != nil {
		return err
	}
	if err := s.upsertRunDailyRollups(ctx, store, run, verifiedRows, verifiedAt); err != nil {
		return err
	}
	rollups, err := s.rebuildDailyRollupsWithStore(ctx, store, run.SiteKey, affectedDates, verifiedAt)
	if err != nil {
		return err
	}
	for _, rollup := range rollups {
		if err := store.UpsertDailyVerificationRollup(ctx, rollup); err != nil {
			return err
		}
		if s.metrics != nil {
			s.metrics.ObserveDailyRollup(rollup)
		}
	}
	if err := s.refreshServingState(ctx, run, verifiedAt); err != nil {
		s.log.Warn("solar forecast serving state refresh failed", "site_key", run.SiteKey, "error", err.Error())
	}
	return nil
}

func (s *Service) applyCalibrationRows(ctx context.Context, store VerificationMutationStore, siteKey, forecastVersion string, verifiedAt time.Time, rows []HourlyTrainingRecord) error {
	if len(rows) == 0 || store == nil {
		return nil
	}
	if applier, ok := store.(CalibrationBatchApplier); ok {
		return applier.ApplyCalibrationRows(ctx, siteKey, forecastVersion, verifiedAt, rows)
	}
	calibrationStates, err := store.LoadCalibrationStates(ctx, siteKey, forecastVersion)
	if err != nil {
		return err
	}
	updatedCalibration := UpdateCalibrationStates(verifiedAt, siteKey, forecastVersion, rows, calibrationStates)
	return store.UpsertCalibrationStates(ctx, updatedCalibration)
}

func (s *Service) upsertRunDailyRollups(ctx context.Context, store VerificationMutationStore, run *Run, rows []HourlyTrainingRecord, verifiedAt time.Time) error {
	if len(rows) == 0 || store == nil || run == nil {
		return nil
	}
	if upserter, ok := store.(DailyRunRollupUpserter); ok {
		return upserter.UpsertRunDailyVerificationRollups(ctx, run, rows, verifiedAt)
	}
	return nil
}

func (s *Service) rebuildDailyRollupsWithStore(ctx context.Context, store VerificationMutationStore, siteKey string, affectedDates map[string]time.Time, nowUTC time.Time) ([]DailyVerificationRollup, error) {
	if len(affectedDates) == 0 {
		return nil, nil
	}
	if rebuilder, ok := store.(DailyRollupRebuilder); ok {
		return rebuilder.RebuildDailyVerificationRollups(ctx, siteKey, affectedDates, nowUTC)
	}
	fromDate, toDate := dateRangeBounds(affectedDates)
	records, err := store.ListVerificationRecords(ctx, siteKey, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	return summarizeVerificationRollups(siteKey, records, nowUTC), nil
}

func (s *Service) loadCalibrationStates(ctx context.Context, siteKey, forecastVersion string) ([]CalibrationState, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.LoadCalibrationStates(ctx, siteKey, forecastVersion)
}

func (s *Service) loadServingState(ctx context.Context, siteKey, forecastVersion string) (*ServingState, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.LoadServingState(ctx, siteKey, forecastVersion)
}

func recentSiteCalibrationFromServingState(state *ServingState) RecentSiteCalibration {
	if state == nil || state.RecentSiteRatio == nil {
		return RecentSiteCalibration{}
	}
	return RecentSiteCalibration{
		MultiplicativeRatio: state.RecentSiteRatio,
		SampleCount:         state.RecentSiteSampleCount,
		UpdatedAt:           state.RecentSiteUpdatedAt,
	}
}

func (s *Service) refreshServingState(ctx context.Context, run *Run, verifiedAt time.Time) error {
	if s == nil || s.store == nil || s.query == nil || run == nil {
		return nil
	}
	loc := loadLocation(run.Timezone)
	deviceIDs := deviceIDsFromRun(run)
	if len(deviceIDs) == 0 {
		return nil
	}
	recentCalibration, err := s.loadRecentSiteCalibrationSummary(ctx, run.SiteKey, run.ForecastVersion, verifiedAt.In(loc), loc)
	if err != nil {
		return err
	}
	windowEndLocal := time.Date(verifiedAt.In(loc).Year(), verifiedAt.In(loc).Month(), verifiedAt.In(loc).Day(), 0, 0, 0, 0, loc)
	windowStartLocal := windowEndLocal.AddDate(0, 0, -7)
	history, err := s.queryLookbackSeries(ctx, deviceIDs, windowStartLocal.UTC(), windowEndLocal.UTC())
	if err != nil {
		return err
	}
	evidence := observedPotentialWatts(history.Points, loc)
	state := ServingState{
		SiteKey:                 run.SiteKey,
		ForecastVersion:         run.ForecastVersion,
		Timezone:                run.Timezone,
		RecentSiteRatio:         recentCalibration.MultiplicativeRatio,
		RecentSiteSampleCount:   recentCalibration.SampleCount,
		RecentSiteUpdatedAt:     recentCalibration.UpdatedAt,
		PotentialBaseEnvelopeW:  optionalPositiveFloat(evidence.baseEnvelopeW),
		PotentialSaturatedW:     optionalPositiveFloat(evidence.saturatedEnvelopeW),
		PotentialFinalEnvelopeW: optionalPositiveFloat(evidence.finalEnvelopeW),
		QualifiedSaturatedDays:  evidence.qualifiedSaturatedDays,
		QualifiedSaturatedHours: evidence.qualifiedSaturatedHours,
		HistoryFrom:             timePtr(windowStartLocal.UTC()),
		HistoryTo:               timePtr(windowEndLocal.UTC()),
		UpdatedAt:               verifiedAt.UTC(),
	}
	return s.store.UpsertServingState(ctx, state)
}

func (s *Service) loadRecentSiteCalibrationSummary(ctx context.Context, siteKey, forecastVersion string, nowLocal time.Time, loc *time.Location) (RecentSiteCalibration, error) {
	if s == nil || s.store == nil || loc == nil {
		return RecentSiteCalibration{}, nil
	}
	yesterday := nowLocal.In(loc).AddDate(0, 0, -1)
	toDate := parseDateISO(localDateISO(yesterday, loc))
	fromDate := parseDateISO(localDateISO(yesterday.AddDate(0, 0, -2), loc))
	records, err := s.store.ListRecentCalibrationRecords(ctx, siteKey, forecastVersion, fromDate, toDate)
	if err != nil {
		return RecentSiteCalibration{}, err
	}
	return BuildRecentSiteCalibration(records, forecastVersion), nil
}

func aggregateSeries(seriesList []telemetryquery.Series, fromUTC, toUTC time.Time) telemetryquery.Series {
	out := telemetryquery.Series{
		DeviceID:   "all",
		Resolution: telemetryquery.ResolutionHour,
		From:       fromUTC,
		To:         toUTC,
	}
	if len(seriesList) == 0 {
		return out
	}
	type bucket struct {
		start time.Time
		end   time.Time
		point telemetryquery.Point
	}
	buckets := make(map[int64]bucket, len(seriesList)*24)
	for _, series := range seriesList {
		for _, point := range series.Points {
			key := point.BucketStart.UTC().UnixMilli()
			entry, ok := buckets[key]
			if !ok {
				entry = bucket{start: point.BucketStart.UTC(), end: point.BucketEnd.UTC(), point: point}
			} else {
				entry.point.SampleCount += point.SampleCount
				entry.point.FirstTsUnixMs = minInt64(entry.point.FirstTsUnixMs, point.FirstTsUnixMs)
				entry.point.LastTsUnixMs = maxInt64(entry.point.LastTsUnixMs, point.LastTsUnixMs)
				entry.point.Metrics = sumMetrics(entry.point.Metrics, point.Metrics)
			}
			buckets[key] = entry
		}
	}
	keys := make([]int64, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out.Points = make([]telemetryquery.Point, 0, len(keys))
	for _, key := range keys {
		entry := buckets[key]
		entry.point.BucketStart = entry.start
		entry.point.BucketEnd = entry.end
		out.Points = append(out.Points, entry.point)
	}
	return out
}

func sumMetrics(left, right telemetryquery.Metrics) telemetryquery.Metrics {
	return telemetryquery.Metrics{
		PVAvgW:           sumFloatPtr(left.PVAvgW, right.PVAvgW),
		PVMaxW:           sumFloatPtr(left.PVMaxW, right.PVMaxW),
		SolarGeneratedWh: sumFloatPtr(left.SolarGeneratedWh, right.SolarGeneratedWh),
	}
}

func inferCapacityEstimate(points []telemetryquery.Point, current *weatherd.HourlyForecastPoint, loc *time.Location) CapacityEstimate {
	observedPeak := observedPeakWatts(points)
	evidence := observedPotentialWatts(points, loc)
	observedEnvelope := evidence.finalEnvelopeW
	if observedEnvelope <= 0 && observedPeak <= 0 {
		return CapacityEstimate{Method: "unavailable"}
	}
	if observedEnvelope > 0 {
		estimate := observedEnvelope
		method := "rolling_observed_p95"
		if factor := resolveIrradianceFactor(current); factor >= 0.35 {
			method = "rolling_observed_p95_and_irradiance"
			if observedPeak > observedEnvelope {
				recoveryCeiling := math.Min(observedPeak, observedEnvelope*1.3)
				recoveryWeight := clamp((factor-0.35)/0.55, 0, 1) * 0.5
				recovered := observedEnvelope + ((recoveryCeiling - observedEnvelope) * recoveryWeight)
				estimate = math.Max(observedEnvelope, recovered)
			}
		}
		capacity := CapacityEstimate{
			EstimatedPeakWatts: floatPtr(roundWatts(estimate)),
			Method:             method,
		}
		if observedPeak > 0 {
			capacity.ObservedPvWatts = floatPtr(roundWatts(observedPeak))
		}
		return capacity
	}
	return CapacityEstimate{
		EstimatedPeakWatts: floatPtr(roundWatts(observedPeak)),
		ObservedPvWatts:    floatPtr(roundWatts(observedPeak)),
		Method:             "live_pv_only",
	}
}

func inferCapacityEstimateFromServingState(state *ServingState, points []telemetryquery.Point, current *weatherd.HourlyForecastPoint, loc *time.Location) CapacityEstimate {
	if state != nil && state.PotentialFinalEnvelopeW != nil && *state.PotentialFinalEnvelopeW > 0 {
		estimate := *state.PotentialFinalEnvelopeW
		method := "rolling_observed_p95"
		if factor := resolveIrradianceFactor(current); factor >= 0.35 {
			method = "rolling_observed_p95_and_irradiance"
		}
		return CapacityEstimate{
			EstimatedPeakWatts: floatPtr(roundWatts(estimate)),
			Method:             method,
		}
	}
	return inferCapacityEstimate(points, current, loc)
}

func summarizeDailyOutlook(
	hourly []weatherd.HourlyForecastPoint,
	daily []weatherd.DailyForecastPoint,
	estimatedPeakWatts *float64,
	actualTodayWh float64,
	todayISO string,
	todayRemainingScale float64,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) []GenerationDay {
	filteredDaily := make([]weatherd.DailyForecastPoint, 0, len(daily))
	for _, day := range daily {
		if localDateISO(day.Date, loc) < todayISO {
			continue
		}
		filteredDaily = append(filteredDaily, day)
		if len(filteredDaily) == 7 {
			break
		}
	}
	out := make([]GenerationDay, 0, len(filteredDaily))
	for _, day := range filteredDaily {
		dayISO := localDateISO(day.Date, loc)
		dayHours := make([]weatherd.HourlyForecastPoint, 0, 24)
		for _, point := range hourly {
			if localDateISO(point.Time, loc) == dayISO {
				dayHours = append(dayHours, point)
			}
		}
		var forecastRemainingWh float64
		var peakWatts float64
		var peakTime *time.Time
		for _, point := range dayHours {
			if dayISO == todayISO && !point.Time.After(nowUTC) {
				continue
			}
			watts := estimateDisplayedForecastWatts(point, estimatedPeakWatts, todayISO, todayRemainingScale, nowUTC, loc, calibrationIndex, siteCalibrationRatio)
			if watts == nil || *watts <= 0 {
				continue
			}
			forecastRemainingWh += *watts
			if *watts > peakWatts {
				peakWatts = *watts
				t := point.Time.UTC()
				peakTime = &t
			}
		}
		totalWh := forecastRemainingWh
		var actualGeneratedKWh *float64
		if dayISO == todayISO && actualTodayWh > 0 {
			totalWh += actualTodayWh
			actualGeneratedKWh = floatPtr(round1(actualTodayWh / 1000))
		}
		var forecastRemainingKWh *float64
		if forecastRemainingWh > 0 {
			forecastRemainingKWh = floatPtr(round1(forecastRemainingWh / 1000))
		}
		var forecastTotalKWh *float64
		if totalWh > 0 {
			forecastTotalKWh = floatPtr(round1(totalWh / 1000))
		}
		var estimatedPeak *float64
		if peakWatts > 0 {
			estimatedPeak = floatPtr(roundWatts(peakWatts))
		}
		out = append(out, GenerationDay{
			Date:                 parseDateISO(dayISO),
			ActualGeneratedKWh:   actualGeneratedKWh,
			ForecastRemainingKWh: forecastRemainingKWh,
			ForecastTotalKWh:     forecastTotalKWh,
			EstimatedPeakWatts:   estimatedPeak,
			PeakTime:             peakTime,
			Confidence:           dayConfidence(dayISO == todayISO, actualTodayWh, estimatedPeakWatts),
		})
	}
	return out
}

func summarizeNext24Hours(
	hourly []weatherd.HourlyForecastPoint,
	estimatedPeakWatts *float64,
	todayISO string,
	todayRemainingScale float64,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) []GenerationPoint {
	out := make([]GenerationPoint, 0, 24)
	for _, point := range hourly {
		if !point.Time.After(nowUTC) {
			continue
		}
		out = append(out, GenerationPoint{
			Time:                   point.Time.UTC(),
			ForecastGeneratedWh:    estimateDisplayedForecastWatts(point, estimatedPeakWatts, todayISO, todayRemainingScale, nowUTC, loc, calibrationIndex, siteCalibrationRatio),
			EstimatedPeakWatts:     estimatedPeakWatts,
			ShortwaveRadiation:     valueOrNil(point.Corrected.ShortwaveRadiation, point.Raw.ShortwaveRadiation),
			GlobalTiltedIrradiance: valueOrNil(point.Corrected.GlobalTiltedIrradiance, point.Raw.GlobalTiltedIrradiance),
			CloudCover:             valueOrNil(point.Corrected.CloudCover, point.Raw.CloudCover),
			Confidence:             pointConfidence(estimatedPeakWatts),
		})
		if len(out) == 24 {
			break
		}
	}
	return out
}

func (s *Service) persistTrainingRun(
	ctx context.Context,
	in Input,
	bundle *weatherd.Bundle,
	history telemetryquery.Series,
	outlook *Outlook,
	actualTodayWh float64,
	nowUTC time.Time,
	nowLocal time.Time,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
	todayRemainingScale float64,
) error {
	if s == nil || s.store == nil || bundle == nil || outlook == nil {
		return nil
	}
	issueAt := alignToInterval(bundle.Provenance.IssuedAt.UTC(), 4*time.Hour)
	issueLoc := loadLocation(outlook.Provenance.Timezone)
	issueLocal := issueAt.In(issueLoc)
	runUUID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	siteMetadataJSON, err := json.Marshal(map[string]any{
		"scope_mode":               outlook.Scope.Mode,
		"resolved_device_ids":      outlook.Scope.ResolvedDeviceIDs,
		"site_resolved_device_ids": normalizedDeviceIDs(in.SiteResolvedDeviceIDs),
		"request_latitude":         in.WeatherRequest.Latitude,
		"request_longitude":        in.WeatherRequest.Longitude,
		"panel_tilt_degrees":       in.WeatherRequest.PanelTiltDegrees,
		"panel_azimuth_degrees":    in.WeatherRequest.PanelAzimuthDegrees,
	})
	if err != nil {
		return err
	}
	provenanceJSON, err := json.Marshal(outlook.Provenance)
	if err != nil {
		return err
	}
	_, offsetSeconds := issueLocal.Zone()
	run := Run{
		ID:                       runUUID.String(),
		SiteKey:                  buildSiteKey(outlook.Provenance.CanonicalLocationKey, outlook.Scope.ResolvedDeviceIDs),
		ScopeKind:                "all",
		DeviceID:                 nil,
		ServedVariant:            normalizedServedVariant(outlook.Provenance.ServedVariant),
		CanonicalLocationKey:     outlook.Provenance.CanonicalLocationKey,
		Timezone:                 outlook.Provenance.Timezone,
		IssuedAt:                 issueAt,
		IssueLocalDate:           parseDateISO(localDateISO(issueLocal, issueLoc)),
		IssueLocalHour:           issueLocal.Hour(),
		IssueUTCOffsetMinutes:    offsetSeconds / 60,
		ForecastVersion:          outlook.Provenance.ForecastModel,
		FeatureVersion:           "weather_v1",
		WeatherSnapshotID:        "",
		CapacityEstimateW:        outlook.Capacity.EstimatedPeakWatts,
		ActualSoFarWh:            actualTodayWh,
		ForecastRemainingTodayWh: kwhToWh(outlook.Today.ForecastRemainingKWh),
		ForecastTotalTodayWh:     kwhToWh(outlook.Today.ForecastTotalKWh),
		SiteMetadataJSON:         siteMetadataJSON,
		ProvenanceJSON:           provenanceJSON,
		CreatedAt:                nowUTC,
		UpdatedAt:                nowUTC,
	}
	canonicalRunID, err := s.store.InsertRun(ctx, run)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveTrainingRun(err)
		}
		return err
	}
	if strings.TrimSpace(canonicalRunID) != "" {
		run.ID = canonicalRunID
	}
	if s.metrics != nil {
		s.metrics.ObserveTrainingRun(nil)
	}
	rows := buildTrainingRows(run, bundle, history, outlook, nowUTC, calibrationIndex, siteCalibrationRatio, todayRemainingScale)
	if err := s.store.InsertHourlyRecords(ctx, rows); err != nil {
		if s.metrics != nil {
			s.metrics.ObserveTrainingHours(len(rows), err)
		}
		return err
	}
	if s.metrics != nil {
		s.metrics.ObserveTrainingHours(len(rows), nil)
	}
	return nil
}

func buildTrainingRows(
	run Run,
	bundle *weatherd.Bundle,
	history telemetryquery.Series,
	outlook *Outlook,
	nowUTC time.Time,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
	todayRemainingScale float64,
) []HourlyTrainingRecord {
	if bundle == nil || outlook == nil {
		return nil
	}
	loc := loadLocation(outlook.Provenance.Timezone)
	limitDate := ""
	if len(outlook.Next7Days) > 0 {
		limitDate = outlook.Next7Days[len(outlook.Next7Days)-1].Date.Format("2006-01-02")
	}
	featureSnapshotJSON, _ := json.Marshal(map[string]any{
		"capacity_method":     outlook.Capacity.Method,
		"observed_pv_watts":   outlook.Capacity.ObservedPvWatts,
		"history_point_count": len(history.Points),
	})
	rows := make([]HourlyTrainingRecord, 0, len(bundle.Hourly))
	for _, point := range bundle.Hourly {
		if !point.Time.After(nowUTC) {
			continue
		}
		targetDateISO := localDateISO(point.Time, loc)
		if limitDate != "" && targetDateISO > limitDate {
			continue
		}
		hoursAhead := int(math.Ceil(point.Time.Sub(nowUTC).Hours()))
		if hoursAhead < 0 {
			hoursAhead = 0
		}
		rows = append(rows, HourlyTrainingRecord{
			RunID:                        run.ID,
			SiteKey:                      run.SiteKey,
			DeviceID:                     run.DeviceID,
			IssuedAt:                     run.IssuedAt,
			TargetTime:                   point.Time.UTC(),
			TargetLocalDate:              parseDateISO(targetDateISO),
			TargetLocalHour:              point.Time.In(loc).Hour(),
			TargetUTCOffsetMinutes:       offsetMinutes(point.Time.In(loc)),
			HorizonHours:                 hoursAhead,
			HorizonBucket:                horizonBucketForHours(hoursAhead),
			ForecastGenerationWh:         floatValue(estimateForecastWatts(point, outlook.Capacity.EstimatedPeakWatts, nowUTC, loc, calibrationIndex, siteCalibrationRatio)),
			BaselineForecastGenerationWh: estimateForecastWatts(point, outlook.Capacity.EstimatedPeakWatts, nowUTC, loc, nil, nil),
			ForecastGTIWm2:               valueOrNil(point.Corrected.GlobalTiltedIrradiance, point.Raw.GlobalTiltedIrradiance),
			ForecastShortwaveWm2:         valueOrNil(point.Corrected.ShortwaveRadiation, point.Raw.ShortwaveRadiation),
			ForecastTemperatureC:         valueOrNil(point.Corrected.Temperature, point.Raw.Temperature),
			ForecastCloudCoverPct:        valueOrNil(point.Corrected.CloudCover, point.Raw.CloudCover),
			ForecastIrradianceSource:     irradianceSourceForPoint(point),
			VerificationStatus:           VerificationStatusPending,
			FeatureSnapshotJSON:          featureSnapshotJSON,
			WeatherRaw:                   point.Raw,
			WeatherCorrected:             point.Corrected,
			CreatedAt:                    run.CreatedAt,
			UpdatedAt:                    run.UpdatedAt,
		})
	}
	return rows
}

func estimateForecastWatts(point weatherd.HourlyForecastPoint, estimatedPeakWatts *float64, nowUTC time.Time, loc *time.Location, calibrationIndex CalibrationIndex, siteCalibrationRatio *float64) *float64 {
	base := estimateBaselineForecastWatts(point, estimatedPeakWatts)
	if base == nil {
		return nil
	}
	futureDaySuppression := futureDayPeakSuppressionFactor(point, nowUTC, loc)
	baseWatts := *base * atmosphericAttenuationFactor(point) * futureDaySuppression
	if baseWatts <= 0 {
		return nil
	}
	capLimit := *estimatedPeakWatts * maxForecastPeakOutputScale
	if futureDaySuppression < 1 {
		capLimit *= futureDaySuppression
	}
	base = floatPtr(round1(math.Min(baseWatts, capLimit)))
	if loc == nil {
		return base
	}
	hoursAhead := int(math.Ceil(point.Time.Sub(nowUTC).Hours()))
	if hoursAhead < 0 {
		hoursAhead = 0
	}
	state := lookupCalibration(calibrationIndex, horizonBucketForHours(hoursAhead), point.Time.In(loc).Hour())
	if hasUsableCalibrationState(state) {
		calibrated := ApplyGenerationCalibration(base, state)
		if calibrated == nil {
			return nil
		}
		return floatPtr(round1(math.Min(*calibrated, capLimit)))
	}
	if siteCalibrationRatio == nil {
		return base
	}
	return floatPtr(round1(math.Min(*base*clampCalibrationRatio(*siteCalibrationRatio), capLimit)))
}

func estimateBaselineForecastWatts(point weatherd.HourlyForecastPoint, estimatedPeakWatts *float64) *float64 {
	if estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 {
		return nil
	}
	factor := resolveIrradianceFactor(&point)
	if factor <= 0 {
		return nil
	}
	temperature := valueOrNil(point.Corrected.Temperature, point.Raw.Temperature)
	temperatureFactor := 1.0
	if temperature != nil && !math.IsNaN(*temperature) {
		temperatureFactor = math.Max(0.82, 1-math.Max(*temperature-25, 0)*0.004)
	}
	watts := math.Min(
		*estimatedPeakWatts*maxForecastPeakOutputScale,
		*estimatedPeakWatts*factor*temperatureFactor*baseSystemEfficiencyFactor,
	)
	if watts <= 0 {
		return nil
	}
	return floatPtr(round1(watts))
}

func estimateDisplayedForecastWatts(
	point weatherd.HourlyForecastPoint,
	estimatedPeakWatts *float64,
	todayISO string,
	todayRemainingScale float64,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) *float64 {
	base := estimateForecastWatts(point, estimatedPeakWatts, nowUTC, loc, calibrationIndex, siteCalibrationRatio)
	if base == nil || loc == nil || todayISO == "" || todayRemainingScale == 1 {
		return base
	}
	if !point.Time.After(nowUTC) || localDateISO(point.Time, loc) != todayISO {
		return base
	}
	return floatPtr(round1(*base * todayRemainingScale))
}

func futureDayPeakSuppressionFactor(point weatherd.HourlyForecastPoint, nowUTC time.Time, loc *time.Location) float64 {
	if loc == nil {
		return 1
	}
	todayISO := localDateISO(nowUTC, loc)
	if localDateISO(point.Time, loc) <= todayISO {
		return 1
	}
	factor := 1.0
	if visibility := valueOrNil(point.Corrected.Visibility, point.Raw.Visibility); visibility != nil && !math.IsNaN(*visibility) {
		visibilityKM := *visibility / 1000
		switch {
		case visibilityKM < 2:
			factor = math.Min(factor, 0.86)
		case visibilityKM < 5:
			factor = math.Min(factor, 0.90)
		case visibilityKM < 10:
			factor = math.Min(factor, 0.95)
		}
	}
	switch point.Condition.WeatherCode {
	case 45, 48:
		factor = math.Min(factor, 0.88)
	case 51, 53, 55, 56, 57:
		factor = math.Min(factor, 0.95)
	case 61, 63, 65, 66, 67, 80, 81, 82:
		factor = math.Min(factor, 0.92)
	case 71, 73, 75, 77, 85, 86:
		factor = math.Min(factor, 0.94)
	case 95, 96, 99:
		factor = math.Min(factor, 0.86)
	}
	if precipitation := valueOrNil(point.Corrected.Precipitation, point.Raw.Precipitation); precipitation != nil && !math.IsNaN(*precipitation) {
		switch {
		case *precipitation > 1.5:
			factor = math.Min(factor, 0.90)
		case *precipitation > 0.5:
			factor = math.Min(factor, 0.94)
		}
	}
	if visibility := valueOrNil(point.Corrected.Visibility, point.Raw.Visibility); visibility != nil && !math.IsNaN(*visibility) {
		visibilityKM := *visibility / 1000
		if visibilityKM < 5 {
			if precipitation := valueOrNil(point.Corrected.Precipitation, point.Raw.Precipitation); precipitation != nil && !math.IsNaN(*precipitation) && *precipitation > 0.5 {
				factor = math.Min(factor, 0.88)
			}
			switch point.Condition.WeatherCode {
			case 45, 48, 61, 63, 65, 66, 67, 80, 81, 82:
				factor = math.Min(factor, 0.88)
			}
		}
	}
	return clamp(factor, 0.86, 1)
}

func resolveIrradianceFactor(point *weatherd.HourlyForecastPoint) float64 {
	if point == nil {
		return 0
	}
	value := valueOrNil(point.Corrected.GlobalTiltedIrradiance, point.Raw.GlobalTiltedIrradiance)
	if value == nil {
		value = valueOrNil(point.Corrected.ShortwaveRadiation, point.Raw.ShortwaveRadiation)
	}
	if value == nil {
		return 0
	}
	return clamp(*value/1000, 0, 1.1)
}

func atmosphericAttenuationFactor(point weatherd.HourlyForecastPoint) float64 {
	codeFactor := 1.0
	visibilityFactor := 1.0
	precipitationFactor := 1.0
	if visibility := valueOrNil(point.Corrected.Visibility, point.Raw.Visibility); visibility != nil && !math.IsNaN(*visibility) {
		visibilityKM := *visibility / 1000
		switch {
		case visibilityKM < 2:
			visibilityFactor = 0.82
		case visibilityKM < 5:
			visibilityFactor = 0.88
		case visibilityKM < 10:
			visibilityFactor = 0.94
		case visibilityKM < 20:
			visibilityFactor = 0.98
		}
	}
	switch point.Condition.WeatherCode {
	case 45, 48:
		codeFactor = 0.82
	case 51, 53, 55, 56, 57:
		codeFactor = 0.90
	case 61, 63, 65, 66, 67, 80, 81, 82:
		codeFactor = 0.86
	case 71, 73, 75, 77, 85, 86:
		codeFactor = 0.90
	case 95, 96, 99:
		codeFactor = 0.80
	}
	if precipitation := valueOrNil(point.Corrected.Precipitation, point.Raw.Precipitation); precipitation != nil && !math.IsNaN(*precipitation) {
		if *precipitation > 0.5 {
			precipitationFactor = 0.95
		} else if *precipitation > 0 {
			precipitationFactor = 0.98
		}
	}
	factor := math.Min(codeFactor, math.Min(visibilityFactor, precipitationFactor))
	return clamp(factor, 0.80, 1)
}

func deriveTodayRemainingScale(
	history telemetryquery.Series,
	hourly []weatherd.HourlyForecastPoint,
	estimatedPeakWatts *float64,
	actualTodayWh float64,
	todayISO string,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) (float64, string) {
	if loc == nil || todayISO == "" || estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 || actualTodayWh <= 0 {
		return 1, ""
	}
	if nowUTC.In(loc).Hour() < minTodayProgressScaleHour {
		return 1, ""
	}
	if !todayTelemetryComplete(history, nowUTC, loc, todayISO) {
		return 1, ""
	}
	var forecastSoFarWh float64
	for _, point := range hourly {
		if localDateISO(point.Time, loc) != todayISO || point.Time.After(nowUTC) {
			continue
		}
		watts := estimateForecastWatts(point, estimatedPeakWatts, nowUTC, loc, calibrationIndex, siteCalibrationRatio)
		if watts == nil || *watts <= 0 {
			continue
		}
		forecastSoFarWh += *watts
	}
	if forecastSoFarWh < minTodayProgressForecastWh {
		return 1, ""
	}
	scale := clamp(actualTodayWh/forecastSoFarWh, minTodayProgressScale, maxTodayProgressScale)
	if trailingScale, ok := deriveTodayTrailingScale(history, hourly, estimatedPeakWatts, todayISO, nowUTC, loc, calibrationIndex, siteCalibrationRatio); ok && trailingScale < scale {
		scale = trailingScale
	}
	if saturationScale, reason := deriveSaturationBandScale(history, hourly, estimatedPeakWatts, todayISO, nowUTC, loc, calibrationIndex, siteCalibrationRatio); saturationScale < scale {
		return saturationScale, reason
	}
	return scale, ""
}

func deriveTodayTrailingScale(
	history telemetryquery.Series,
	hourly []weatherd.HourlyForecastPoint,
	estimatedPeakWatts *float64,
	todayISO string,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) (float64, bool) {
	if loc == nil || todayISO == "" || estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 {
		return 1, false
	}
	if nowUTC.In(loc).Hour() < minTodayTrailingScaleHour {
		return 1, false
	}
	actualByBucket := make(map[time.Time]telemetryquery.Point, len(history.Points))
	for _, point := range history.Points {
		if localDateISO(point.BucketStart, loc) != todayISO {
			continue
		}
		actualByBucket[point.BucketStart.UTC()] = point
	}
	windowStart := nowUTC.Add(-1 * time.Duration(todayTrailingWindowHours) * time.Hour)
	actualRecentWh := 0.0
	forecastRecentWh := 0.0
	for _, point := range hourly {
		if localDateISO(point.Time, loc) != todayISO || point.Time.After(nowUTC) || !point.Time.After(windowStart) {
			continue
		}
		forecastWh := estimateForecastWatts(point, estimatedPeakWatts, nowUTC, loc, calibrationIndex, siteCalibrationRatio)
		if forecastWh == nil || *forecastWh <= 0 {
			continue
		}
		actualPoint, ok := actualByBucket[point.Time.UTC()]
		if !ok || actualPoint.Metrics.SolarGeneratedWh == nil {
			continue
		}
		forecastRecentWh += *forecastWh
		actualRecentWh += *actualPoint.Metrics.SolarGeneratedWh
	}
	if forecastRecentWh < minTodayTrailingForecastWh || actualRecentWh <= 0 {
		return 1, false
	}
	return clamp(actualRecentWh/forecastRecentWh, minTodayTrailingScale, 1), true
}

func deriveSaturationBandScale(
	history telemetryquery.Series,
	hourly []weatherd.HourlyForecastPoint,
	estimatedPeakWatts *float64,
	todayISO string,
	nowUTC time.Time,
	loc *time.Location,
	calibrationIndex CalibrationIndex,
	siteCalibrationRatio *float64,
) (float64, string) {
	if loc == nil || todayISO == "" || estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 {
		return 1, ""
	}
	actualByBucket := make(map[time.Time]telemetryquery.Point, len(history.Points))
	for _, point := range history.Points {
		if localDateISO(point.BucketStart, loc) != todayISO {
			continue
		}
		actualByBucket[point.BucketStart.UTC()] = point
	}
	saturatedHours := 0
	ratios := make([]float64, 0, 3)
	for _, forecastPoint := range hourly {
		if localDateISO(forecastPoint.Time, loc) != todayISO || forecastPoint.Time.After(nowUTC) {
			continue
		}
		actualPoint, ok := actualByBucket[forecastPoint.Time.UTC()]
		if !ok || !isSaturatedBatteryPoint(actualPoint.Metrics) {
			continue
		}
		forecastWh := estimateForecastWatts(forecastPoint, estimatedPeakWatts, nowUTC, loc, calibrationIndex, siteCalibrationRatio)
		if forecastWh == nil || *forecastWh < minSaturatedPotentialWatts || actualPoint.Metrics.SolarGeneratedWh == nil || *actualPoint.Metrics.SolarGeneratedWh < minSaturatedPotentialWatts {
			continue
		}
		if !isChargeConstrainedSaturation(actualPoint.Metrics, *forecastWh) {
			continue
		}
		saturatedHours++
		ratios = append(ratios, clamp(*actualPoint.Metrics.SolarGeneratedWh / *forecastWh, minTodayProgressScale, 1))
	}
	if saturatedHours < 2 || len(ratios) < 2 {
		return 1, ""
	}
	if len(ratios) > 3 {
		ratios = ratios[len(ratios)-3:]
	}
	bestRecent := ratios[0]
	for _, ratio := range ratios[1:] {
		if ratio > bestRecent {
			bestRecent = ratio
		}
	}
	return bestRecent, sameDayCurtailmentReasonBatteryNearFull
}

func todayTelemetryComplete(history telemetryquery.Series, nowUTC time.Time, loc *time.Location, todayISO string) bool {
	if loc == nil || todayISO == "" {
		return false
	}
	if history.EnergyBucketCoverage.PointCount <= 0 || history.EnergyBucketCoverage.PersistedValueCount <= 0 {
		return false
	}
	nowLocal := nowUTC.In(loc)
	todayStartLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	completedUntilLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), nowLocal.Hour(), 0, 0, 0, loc)
	expectedCompletedHours := int(completedUntilLocal.Sub(todayStartLocal).Hours())
	if expectedCompletedHours <= 0 {
		return false
	}

	persistedTodayHours := 0
	var latestPersistedBucketEnd time.Time
	for _, point := range history.Points {
		if localDateISO(point.BucketStart, loc) != todayISO || point.Metrics.SolarGeneratedWh == nil {
			continue
		}
		if point.BucketEnd.After(completedUntilLocal.UTC()) {
			continue
		}
		persistedTodayHours++
		if point.BucketEnd.After(latestPersistedBucketEnd) {
			latestPersistedBucketEnd = point.BucketEnd
		}
	}
	if persistedTodayHours == 0 || latestPersistedBucketEnd.IsZero() {
		return false
	}

	allowedMissingHours := 1
	if expectedCompletedHours <= minTodayProgressScaleHour {
		allowedMissingHours = 0
	}
	if persistedTodayHours+allowedMissingHours < expectedCompletedHours {
		return false
	}
	return !latestPersistedBucketEnd.Before(completedUntilLocal.UTC().Add(-1 * time.Hour))
}

func observedPeakWatts(points []telemetryquery.Point) float64 {
	var peak float64
	for _, point := range points {
		candidates := []*float64{point.Metrics.PVMaxW, point.Metrics.PVAvgW}
		if point.Metrics.SolarGeneratedWh != nil {
			durationHours := point.BucketEnd.Sub(point.BucketStart).Hours()
			if durationHours > 0 {
				candidates = append(candidates, floatPtr(*point.Metrics.SolarGeneratedWh/durationHours))
			}
		}
		for _, candidate := range candidates {
			if candidate != nil && *candidate > peak {
				peak = *candidate
			}
		}
	}
	return peak
}

func observedPotentialWatts(points []telemetryquery.Point, loc *time.Location) potentialEvidence {
	if len(points) == 0 {
		return potentialEvidence{}
	}
	baseSamples := make([]float64, 0, len(points))
	saturatedSamples := make([]float64, 0, len(points))
	saturatedDays := map[string]struct{}{}
	for _, point := range points {
		candidate := observedPowerCandidate(point)
		if candidate <= 0 {
			continue
		}
		if isQualifiedSaturatedPotentialPoint(point, candidate) {
			saturatedSamples = append(saturatedSamples, candidate)
			dayKey := point.BucketStart.UTC().Format("2006-01-02")
			if loc != nil {
				dayKey = localDateISO(point.BucketStart, loc)
			}
			saturatedDays[dayKey] = struct{}{}
			continue
		}
		baseSamples = append(baseSamples, candidate)
	}
	baseEnvelope := percentile(baseSamples, 0.95)
	finalEnvelope := baseEnvelope
	saturatedEnvelope := 0.0
	if len(saturatedSamples) >= minQualifiedSaturatedHours && len(saturatedDays) >= minQualifiedSaturatedDays {
		filtered := make([]float64, 0, len(saturatedSamples))
		for _, sample := range saturatedSamples {
			if baseEnvelope > 0 && sample < baseEnvelope*minSaturatedRelativeStrength {
				continue
			}
			if sample < minSaturatedPotentialWatts {
				continue
			}
			filtered = append(filtered, sample)
		}
		if len(filtered) >= minQualifiedSaturatedHours {
			saturatedEnvelope = percentile(filtered, 0.95)
			switch {
			case baseEnvelope <= 0:
				finalEnvelope = saturatedEnvelope
			case saturatedEnvelope > 0:
				uplifted := math.Min(saturatedEnvelope, baseEnvelope*saturatedPotentialUpliftScale)
				finalEnvelope = math.Max(baseEnvelope, uplifted)
			}
			saturatedSamples = filtered
		}
	}
	return potentialEvidence{
		baseEnvelopeW:           baseEnvelope,
		saturatedEnvelopeW:      saturatedEnvelope,
		finalEnvelopeW:          finalEnvelope,
		qualifiedSaturatedDays:  len(saturatedDays),
		qualifiedSaturatedHours: len(saturatedSamples),
	}
}

func observedPowerCandidate(point telemetryquery.Point) float64 {
	var candidate float64
	if point.Metrics.SolarGeneratedWh != nil {
		durationHours := point.BucketEnd.Sub(point.BucketStart).Hours()
		if durationHours > 0 {
			candidate = math.Max(candidate, *point.Metrics.SolarGeneratedWh/durationHours)
		}
	}
	if point.Metrics.PVAvgW != nil {
		candidate = math.Max(candidate, *point.Metrics.PVAvgW)
	}
	if isSaturatedBatteryPoint(point.Metrics) && point.Metrics.PVMaxW != nil {
		candidate = math.Max(candidate, *point.Metrics.PVMaxW)
	}
	return candidate
}

func isChargeConstrainedSaturation(metrics telemetryquery.Metrics, modeledWh float64) bool {
	if !isSaturatedBatteryPoint(metrics) || modeledWh <= 0 {
		return false
	}
	if metrics.BatteryChargeEnergyWh == nil || *metrics.BatteryChargeEnergyWh < 0 {
		return false
	}
	chargeLimit := math.Max(minSaturatedChargeEnergyWh, modeledWh*maxSaturatedChargeEnergyRatio)
	return *metrics.BatteryChargeEnergyWh <= chargeLimit
}

func isQualifiedSaturatedPotentialPoint(point telemetryquery.Point, candidate float64) bool {
	if !isChargeConstrainedSaturation(point.Metrics, candidate) {
		return false
	}
	return candidate >= minSaturatedPotentialWatts
}

func isSaturatedBatteryPoint(metrics telemetryquery.Metrics) bool {
	if metrics.SOCMaxPct != nil && *metrics.SOCMaxPct >= 99 {
		return true
	}
	if metrics.SOCAvgPct != nil && *metrics.SOCAvgPct >= 98 {
		return true
	}
	return false
}

func percentile(values []float64, ratio float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := clamp(ratio, 0, 1) * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower] + ((sorted[upper] - sorted[lower]) * weight)
}

func todayActualWh(points []telemetryquery.Point, loc *time.Location, todayISO string) float64 {
	var total float64
	for _, point := range points {
		if localDateISO(point.BucketStart, loc) != todayISO {
			continue
		}
		if point.Metrics.SolarGeneratedWh != nil && *point.Metrics.SolarGeneratedWh > 0 {
			total += *point.Metrics.SolarGeneratedWh
		}
	}
	return total
}

func dailyActualWhByISO(points []telemetryquery.Point, loc *time.Location) map[string]float64 {
	out := make(map[string]float64)
	for _, point := range points {
		if point.Metrics.SolarGeneratedWh == nil || *point.Metrics.SolarGeneratedWh <= 0 {
			continue
		}
		out[localDateISO(point.BucketStart, loc)] += *point.Metrics.SolarGeneratedWh
	}
	return out
}

func defaultDeviceAllocationShare(deviceCount int) float64 {
	if deviceCount <= 0 {
		return 1
	}
	return 1 / float64(deviceCount)
}

func deriveDeviceAllocationShare(devicePoints, sitePoints []telemetryquery.Point, loc *time.Location, todayISO string, fallback float64) float64 {
	deviceByDay := dailyActualWhByISO(devicePoints, loc)
	siteByDay := dailyActualWhByISO(sitePoints, loc)
	var (
		deviceCompleteWh float64
		siteCompleteWh   float64
	)
	for dayISO, siteDayWh := range siteByDay {
		if dayISO == todayISO || siteDayWh < minDeviceShareDayWh {
			continue
		}
		deviceCompleteWh += deviceByDay[dayISO]
		siteCompleteWh += siteDayWh
	}
	share := fallback
	if siteCompleteWh > 0 {
		share = deviceCompleteWh / siteCompleteWh
	}
	siteTodayWh := siteByDay[todayISO]
	if siteTodayWh >= minDeviceShareDayWh {
		todayShare := clamp(deviceByDay[todayISO]/siteTodayWh, 0, 1)
		if siteCompleteWh > 0 {
			share = ((1 - deviceShareRecentDayWeight) * share) + (deviceShareRecentDayWeight * todayShare)
		} else {
			share = todayShare
		}
	}
	return clamp(share, 0, 1)
}

func allocateCapacityEstimate(site CapacityEstimate, devicePoints []telemetryquery.Point, share float64) CapacityEstimate {
	out := CapacityEstimate{
		Method: site.Method,
	}
	if site.EstimatedPeakWatts != nil && *site.EstimatedPeakWatts > 0 {
		allocated := roundWatts(*site.EstimatedPeakWatts * clamp(share, 0, 1))
		if allocated > 0 {
			out.EstimatedPeakWatts = floatPtr(allocated)
		}
	}
	if observed := observedPeakWatts(devicePoints); observed > 0 {
		out.ObservedPvWatts = floatPtr(roundWatts(observed))
		if out.EstimatedPeakWatts == nil || *out.EstimatedPeakWatts < observed {
			out.EstimatedPeakWatts = floatPtr(roundWatts(observed))
		}
	}
	if out.Method != "" {
		out.Method += "_device_share"
	}
	return out
}

func allocateGenerationDays(days []GenerationDay, actualTodayWh float64, share float64, todayISO string) []GenerationDay {
	out := make([]GenerationDay, 0, len(days))
	for _, day := range days {
		allocated := day
		if day.ForecastRemainingKWh != nil {
			remaining := round1(*day.ForecastRemainingKWh * clamp(share, 0, 1))
			allocated.ForecastRemainingKWh = floatPtr(remaining)
		}
		if day.ForecastTotalKWh != nil {
			total := round1(*day.ForecastTotalKWh * clamp(share, 0, 1))
			if day.Date.Format("2006-01-02") == todayISO {
				total = round1((actualTodayWh / 1000) + floatValue(allocated.ForecastRemainingKWh))
			}
			allocated.ForecastTotalKWh = floatPtr(total)
		}
		if day.Date.Format("2006-01-02") == todayISO {
			actual := round1(actualTodayWh / 1000)
			allocated.ActualGeneratedKWh = floatPtr(actual)
		}
		if day.EstimatedPeakWatts != nil {
			estimatedPeak := roundWatts(*day.EstimatedPeakWatts * clamp(share, 0, 1))
			allocated.EstimatedPeakWatts = floatPtr(estimatedPeak)
		}
		out = append(out, allocated)
	}
	return out
}

func allocateGenerationPoints(points []GenerationPoint, share float64) []GenerationPoint {
	out := make([]GenerationPoint, 0, len(points))
	for _, point := range points {
		allocated := point
		if point.ForecastGeneratedWh != nil {
			forecastWh := round1(*point.ForecastGeneratedWh * clamp(share, 0, 1))
			allocated.ForecastGeneratedWh = floatPtr(forecastWh)
		}
		if point.EstimatedPeakWatts != nil {
			estimatedPeak := roundWatts(*point.EstimatedPeakWatts * clamp(share, 0, 1))
			allocated.EstimatedPeakWatts = floatPtr(estimatedPeak)
		}
		out = append(out, allocated)
	}
	return out
}

func currentWeatherPoint(points []weatherd.HourlyForecastPoint, nowUTC time.Time) *weatherd.HourlyForecastPoint {
	for i := range points {
		if !points[i].Time.Before(nowUTC) {
			return &points[i]
		}
	}
	if len(points) == 0 {
		return nil
	}
	return &points[len(points)-1]
}

func firstDayForISO(days []GenerationDay, dateISO string) GenerationDay {
	for _, day := range days {
		if day.Date.Format("2006-01-02") == dateISO {
			return day
		}
	}
	if len(days) > 0 {
		return days[0]
	}
	return GenerationDay{}
}

func normalizedDeviceIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func normalizedScopeMode(mode string, deviceCount int) string {
	trimmed := strings.TrimSpace(mode)
	if trimmed != "" {
		return trimmed
	}
	if deviceCount == 1 {
		return "device"
	}
	return "all"
}

func localDateISO(value time.Time, loc *time.Location) string {
	return value.In(loc).Format("2006-01-02")
}

func loadLocation(timezone string) *time.Location {
	trimmed := strings.TrimSpace(timezone)
	if trimmed == "" || trimmed == "auto" {
		return time.UTC
	}
	loc, err := time.LoadLocation(trimmed)
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseDateISO(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func hourRangeLimit(fromUTC, toUTC time.Time) int {
	if !toUTC.After(fromUTC) {
		return 1
	}
	return int(math.Ceil(toUTC.Sub(fromUTC).Hours())) + 2
}

func targetBounds(rows []HourlyTrainingRecord) (time.Time, time.Time) {
	if len(rows) == 0 {
		return time.Time{}, time.Time{}
	}
	fromUTC := rows[0].TargetTime.UTC()
	toUTC := rows[0].TargetTime.UTC().Add(time.Hour)
	for _, row := range rows[1:] {
		target := row.TargetTime.UTC()
		if target.Before(fromUTC) {
			fromUTC = target
		}
		end := target.Add(time.Hour)
		if end.After(toUTC) {
			toUTC = end
		}
	}
	return fromUTC, toUTC
}

func telemetryByTime(points []telemetryquery.Point) map[time.Time]telemetryquery.Point {
	out := make(map[time.Time]telemetryquery.Point, len(points))
	for _, point := range points {
		out[point.BucketStart.UTC()] = point
	}
	return out
}

func deviceIDsFromRun(run *Run) []string {
	if run == nil {
		return nil
	}
	var metadata struct {
		ResolvedDeviceIDs []string `json:"resolved_device_ids"`
	}
	if len(run.SiteMetadataJSON) > 0 {
		if err := json.Unmarshal(run.SiteMetadataJSON, &metadata); err == nil {
			if ids := normalizedDeviceIDs(metadata.ResolvedDeviceIDs); len(ids) > 0 {
				return ids
			}
		}
	}
	if run.DeviceID != nil && strings.TrimSpace(*run.DeviceID) != "" {
		return []string{strings.TrimSpace(*run.DeviceID)}
	}
	return nil
}

func groupPendingByRun(rows []HourlyTrainingRecord) [][]HourlyTrainingRecord {
	if len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RunID == rows[j].RunID {
			return rows[i].TargetTime.Before(rows[j].TargetTime)
		}
		return rows[i].RunID < rows[j].RunID
	})
	out := make([][]HourlyTrainingRecord, 0, len(rows))
	start := 0
	for idx := 1; idx <= len(rows); idx++ {
		if idx < len(rows) && rows[idx].RunID == rows[start].RunID {
			continue
		}
		out = append(out, append([]HourlyTrainingRecord(nil), rows[start:idx]...))
		start = idx
	}
	return out
}

func dateRangeBounds(values map[string]time.Time) (time.Time, time.Time) {
	var from, to time.Time
	for _, value := range values {
		day := value.UTC()
		if from.IsZero() || day.Before(from) {
			from = day
		}
		if to.IsZero() || day.After(to) {
			to = day
		}
	}
	return from, to
}

func hasUsableCalibration(states []CalibrationState) bool {
	for _, state := range states {
		if state.MultiplicativeRatio != nil && state.SampleCount >= minCalibrationSamples {
			return true
		}
	}
	return false
}

func summarizeCalibration(states []CalibrationState, recentSiteCalibration RecentSiteCalibration) (int, *time.Time) {
	totalSamples := 0
	var latest time.Time
	for _, state := range states {
		if state.MultiplicativeRatio == nil || state.SampleCount < minCalibrationSamples {
			continue
		}
		totalSamples += state.SampleCount
		if latest.IsZero() || state.UpdatedAt.After(latest) {
			latest = state.UpdatedAt.UTC()
		}
	}
	if latest.IsZero() {
		if recentSiteCalibration.UpdatedAt == nil {
			return totalSamples + recentSiteCalibration.SampleCount, nil
		}
		return totalSamples + recentSiteCalibration.SampleCount, recentSiteCalibration.UpdatedAt
	}
	totalSamples += recentSiteCalibration.SampleCount
	if recentSiteCalibration.UpdatedAt != nil && recentSiteCalibration.UpdatedAt.After(latest) {
		latest = recentSiteCalibration.UpdatedAt.UTC()
	}
	return totalSamples, timePtr(latest)
}

func summarizeVerificationRollups(siteKey string, records []VerificationRecord, nowUTC time.Time) []DailyVerificationRollup {
	type rollupKey struct {
		date            string
		forecastVersion string
		servedVariant   string
		horizon         HorizonBucket
	}
	groups := make(map[rollupKey][]VerificationRecord)
	for _, record := range records {
		key := rollupKey{
			date:            record.TargetLocalDate.Format("2006-01-02"),
			forecastVersion: record.ForecastVersion,
			servedVariant:   normalizedServedVariant(record.ServedVariant),
			horizon:         record.HorizonBucket,
		}
		groups[key] = append(groups[key], record)
	}
	keys := make([]rollupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].date == keys[j].date {
			if keys[i].forecastVersion == keys[j].forecastVersion {
				if keys[i].servedVariant == keys[j].servedVariant {
					return keys[i].horizon < keys[j].horizon
				}
				return keys[i].servedVariant < keys[j].servedVariant
			}
			return keys[i].forecastVersion < keys[j].forecastVersion
		}
		return keys[i].date < keys[j].date
	})
	out := make([]DailyVerificationRollup, 0, len(keys))
	for _, key := range keys {
		rows := groups[key]
		timezone := rows[0].Timezone
		loc := loadLocation(timezone)
		day := parseDateISO(key.date)
		var deviceID *string
		if allDeviceIDsEqual(rows) {
			deviceID = rows[0].DeviceID
		}
		var (
			forecastHours            int
			verifiedHours            int
			missingTruthHours        int
			missingWeatherHours      int
			hourlyAbsError           float64
			hourlySqError            float64
			forecastTotal            float64
			baselineForecastTotal    float64
			actualTotal              float64
			forecastPeak             float64
			baselineForecastPeak     float64
			actualPeak               float64
			forecastPeakTime         time.Time
			baselineForecastPeakTime time.Time
			actualPeakTime           time.Time
		)
		for _, row := range rows {
			forecastHours++
			switch row.VerificationStatus {
			case VerificationStatusVerified:
				verifiedHours++
				forecastTotal += row.ForecastGenerationWh
				if row.BaselineForecastGenerationWh != nil {
					baselineForecastTotal += *row.BaselineForecastGenerationWh
				}
				if row.ActualGenerationWh != nil {
					actualTotal += *row.ActualGenerationWh
					if *row.ActualGenerationWh > actualPeak {
						actualPeak = *row.ActualGenerationWh
						actualPeakTime = row.TargetTime.In(loc)
					}
				}
				if row.ForecastGenerationWh > forecastPeak {
					forecastPeak = row.ForecastGenerationWh
					forecastPeakTime = row.TargetTime.In(loc)
				}
				if row.BaselineForecastGenerationWh != nil && *row.BaselineForecastGenerationWh > baselineForecastPeak {
					baselineForecastPeak = *row.BaselineForecastGenerationWh
					baselineForecastPeakTime = row.TargetTime.In(loc)
				}
				if row.AbsoluteErrorWh != nil {
					hourlyAbsError += *row.AbsoluteErrorWh
				}
				if row.SquaredErrorWh2 != nil {
					hourlySqError += *row.SquaredErrorWh2
				}
			case VerificationStatusMissingTruth:
				missingTruthHours++
			case VerificationStatusMissingWeather:
				missingWeatherHours++
			}
		}
		peakPowerError := math.Abs(forecastPeak - actualPeak)
		baselinePeakPowerError := math.Abs(baselineForecastPeak - actualPeak)
		var peakTimeError float64
		if !forecastPeakTime.IsZero() && !actualPeakTime.IsZero() {
			peakTimeError = math.Abs(forecastPeakTime.Sub(actualPeakTime).Minutes())
		}
		var baselinePeakTimeError float64
		if !baselineForecastPeakTime.IsZero() && !actualPeakTime.IsZero() {
			baselinePeakTimeError = math.Abs(baselineForecastPeakTime.Sub(actualPeakTime).Minutes())
		}
		out = append(out, DailyVerificationRollup{
			SiteKey:                            siteKey,
			DeviceID:                           deviceID,
			ServedVariant:                      key.servedVariant,
			VerificationLocalDate:              day,
			Timezone:                           timezone,
			ForecastVersion:                    key.forecastVersion,
			HorizonBucket:                      key.horizon,
			ForecastHours:                      forecastHours,
			VerifiedHours:                      verifiedHours,
			MissingTruthHours:                  missingTruthHours,
			MissingWeatherHours:                missingWeatherHours,
			HourlyAbsErrorWhSum:                round1(hourlyAbsError),
			HourlySqErrorWh2Sum:                round1(hourlySqError),
			DailyAbsErrorWhSum:                 round1(math.Abs(forecastTotal - actualTotal)),
			BaselineDailyAbsErrorWhSum:         round1(math.Abs(baselineForecastTotal - actualTotal)),
			PeakPowerAbsErrorWSum:              round1(peakPowerError),
			BaselinePeakPowerAbsErrorWSum:      round1(baselinePeakPowerError),
			PeakTimeAbsErrorMinutesSum:         round1(peakTimeError),
			BaselinePeakTimeAbsErrorMinutesSum: round1(baselinePeakTimeError),
			CreatedAt:                          nowUTC,
			UpdatedAt:                          nowUTC,
		})
	}
	return out
}

func normalizedServedVariant(value string) string {
	switch strings.TrimSpace(value) {
	case "site_calibrated":
		return "site_calibrated"
	default:
		return "baseline"
	}
}

func allDeviceIDsEqual(rows []VerificationRecord) bool {
	if len(rows) == 0 {
		return false
	}
	first := rows[0].DeviceID
	for _, row := range rows[1:] {
		if (first == nil) != (row.DeviceID == nil) {
			return false
		}
		if first != nil && row.DeviceID != nil && *first != *row.DeviceID {
			return false
		}
	}
	return first != nil
}

func buildSiteKey(canonicalLocationKey string, deviceIDs []string) string {
	if len(deviceIDs) == 0 {
		return canonicalLocationKey
	}
	return canonicalLocationKey + "|" + strings.Join(deviceIDs, ",")
}

func canonicalTrainingOutlook(outlook *Outlook, siteDeviceIDs []string) *Outlook {
	if outlook == nil {
		return nil
	}
	out := *outlook
	out.Scope = Scope{
		Mode:              "all",
		ResolvedDeviceIDs: append([]string(nil), siteDeviceIDs...),
	}
	out.Next7Days = cloneGenerationDays(outlook.Next7Days)
	out.Next24Hours = cloneGenerationPoints(outlook.Next24Hours)
	return &out
}

func cloneGenerationDays(days []GenerationDay) []GenerationDay {
	if len(days) == 0 {
		return nil
	}
	out := make([]GenerationDay, len(days))
	copy(out, days)
	return out
}

func cloneGenerationPoints(points []GenerationPoint) []GenerationPoint {
	if len(points) == 0 {
		return nil
	}
	out := make([]GenerationPoint, len(points))
	copy(out, points)
	return out
}

func alignToInterval(t time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return t.UTC()
	}
	return t.UTC().Truncate(interval)
}

func singleDeviceID(deviceIDs []string, scopeMode string) *string {
	if scopeMode != "device" || len(deviceIDs) != 1 {
		return nil
	}
	id := deviceIDs[0]
	return &id
}

func kwhToWh(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value * 1000
}

func offsetMinutes(localTime time.Time) int {
	_, offsetSeconds := localTime.Zone()
	return offsetSeconds / 60
}

func horizonBucketForHours(hours int) HorizonBucket {
	switch {
	case hours <= 24:
		return HorizonBucketSameDay
	case hours <= 48:
		return HorizonBucketDay1
	case hours <= 96:
		return HorizonBucketDay3
	default:
		return HorizonBucketDay7
	}
}

func irradianceSourceForPoint(point weatherd.HourlyForecastPoint) IrradianceSource {
	switch {
	case valueOrNil(point.Corrected.GlobalTiltedIrradiance, point.Raw.GlobalTiltedIrradiance) != nil:
		return IrradianceSourceGTI
	case valueOrNil(point.Corrected.ShortwaveRadiation, point.Raw.ShortwaveRadiation) != nil:
		return IrradianceSourceShortwave
	default:
		return IrradianceSourceUnknown
	}
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func valueOrNil(primary, fallback *float64) *float64 {
	if primary != nil {
		return primary
	}
	return fallback
}

func pointConfidence(estimatedPeakWatts *float64) Confidence {
	if estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 {
		return ConfidenceLow
	}
	return ConfidenceMedium
}

func dayConfidence(isToday bool, actualTodayWh float64, estimatedPeakWatts *float64) Confidence {
	if estimatedPeakWatts == nil || *estimatedPeakWatts <= 0 {
		return ConfidenceLow
	}
	if isToday && actualTodayWh > 0 {
		return ConfidenceHigh
	}
	return ConfidenceMedium
}

func floatPtr(value float64) *float64 {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func optionalPositiveFloat(value float64) *float64 {
	if value <= 0 || math.IsNaN(value) {
		return nil
	}
	return floatPtr(value)
}

func clamp(value, min, max float64) float64 {
	return math.Min(max, math.Max(min, value))
}

func sumFloatPtr(left, right *float64) *float64 {
	if left == nil && right == nil {
		return nil
	}
	var total float64
	if left != nil {
		total += *left
	}
	if right != nil {
		total += *right
	}
	return floatPtr(total)
}

func roundWatts(value float64) float64 {
	return math.Round(value/10) * 10
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func minInt64(left, right int64) int64 {
	if left == 0 || (right != 0 && right < left) {
		return right
	}
	return left
}

func maxInt64(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}

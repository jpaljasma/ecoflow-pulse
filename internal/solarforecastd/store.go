package solarforecastd

import (
	"context"
	"sync"
	"time"
)

type TrainingStore interface {
	InsertRun(ctx context.Context, run Run) (string, error)
	InsertHourlyRecords(ctx context.Context, rows []HourlyTrainingRecord) error
	ListPendingHourlyRecords(ctx context.Context, before time.Time, limit int) ([]HourlyTrainingRecord, error)
	ListVerificationRecords(ctx context.Context, siteKey string, fromDate, toDate time.Time) ([]VerificationRecord, error)
	ListRecentCalibrationRecords(ctx context.Context, siteKey, forecastVersion string, fromDate, toDate time.Time) ([]VerificationRecord, error)
	LoadCalibrationStates(ctx context.Context, siteKey, forecastVersion string) ([]CalibrationState, error)
	LoadServingState(ctx context.Context, siteKey, forecastVersion string) (*ServingState, error)
	UpsertCalibrationStates(ctx context.Context, states []CalibrationState) error
	UpsertServingState(ctx context.Context, state ServingState) error
	CompleteHourlyVerification(ctx context.Context, rows []HourlyTrainingRecord) error
	UpsertDailyVerificationRollup(ctx context.Context, row DailyVerificationRollup) error
	GetRun(ctx context.Context, id string) (*Run, error)
	Close() error
}

type VerificationMutationStore interface {
	LoadCalibrationStates(ctx context.Context, siteKey, forecastVersion string) ([]CalibrationState, error)
	CompleteHourlyVerification(ctx context.Context, rows []HourlyTrainingRecord) error
	UpsertCalibrationStates(ctx context.Context, states []CalibrationState) error
	UpsertDailyVerificationRollup(ctx context.Context, row DailyVerificationRollup) error
	ListVerificationRecords(ctx context.Context, siteKey string, fromDate, toDate time.Time) ([]VerificationRecord, error)
}

type PendingRunClaim struct {
	Run      *Run
	Rows     []HourlyTrainingRecord
	Store    VerificationMutationStore
	finalize func(commit bool) error
	once     sync.Once
	finalErr error
}

func (c *PendingRunClaim) Commit() error {
	if c == nil || c.finalize == nil {
		return nil
	}
	c.once.Do(func() {
		c.finalErr = c.finalize(true)
	})
	return c.finalErr
}

func (c *PendingRunClaim) Rollback() error {
	if c == nil || c.finalize == nil {
		return nil
	}
	c.once.Do(func() {
		c.finalErr = c.finalize(false)
	})
	return c.finalErr
}

func NewPendingRunClaim(run *Run, rows []HourlyTrainingRecord, store VerificationMutationStore, finalize func(commit bool) error) *PendingRunClaim {
	return &PendingRunClaim{
		Run:      run,
		Rows:     rows,
		Store:    store,
		finalize: finalize,
	}
}

type PendingRunClaimer interface {
	ClaimPendingRun(ctx context.Context, before time.Time) (*PendingRunClaim, error)
}

type CalibrationBatchApplier interface {
	ApplyCalibrationRows(ctx context.Context, siteKey, forecastVersion string, verifiedAt time.Time, rows []HourlyTrainingRecord) error
}

type DailyRollupRebuilder interface {
	RebuildDailyVerificationRollups(ctx context.Context, siteKey string, affectedDates map[string]time.Time, nowUTC time.Time) ([]DailyVerificationRollup, error)
}

type DailyRunRollupUpserter interface {
	UpsertRunDailyVerificationRollups(ctx context.Context, run *Run, rows []HourlyTrainingRecord, verifiedAt time.Time) error
}

type TrainingDataPruner interface {
	PruneRunsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	PruneDailyVerificationOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

package solarforecastd

import (
	"context"
	"time"
)

type TrainingStore interface {
	InsertRun(ctx context.Context, run Run) (string, error)
	InsertHourlyRecords(ctx context.Context, rows []HourlyTrainingRecord) error
	ListPendingHourlyRecords(ctx context.Context, before time.Time, limit int) ([]HourlyTrainingRecord, error)
	ListVerificationRecords(ctx context.Context, siteKey string, fromDate, toDate time.Time) ([]VerificationRecord, error)
	ListRecentCalibrationRecords(ctx context.Context, siteKey, forecastVersion string, fromDate, toDate time.Time) ([]VerificationRecord, error)
	LoadCalibrationStates(ctx context.Context, siteKey, forecastVersion string) ([]CalibrationState, error)
	UpsertCalibrationStates(ctx context.Context, states []CalibrationState) error
	CompleteHourlyVerification(ctx context.Context, rows []HourlyTrainingRecord) error
	UpsertDailyVerificationRollup(ctx context.Context, row DailyVerificationRollup) error
	GetRun(ctx context.Context, id string) (*Run, error)
	Close() error
}

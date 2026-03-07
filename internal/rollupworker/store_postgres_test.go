package rollupworker

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestNewPostgresStoreRejectsWhitespaceDSN(t *testing.T) {
	t.Parallel()
	if _, err := NewPostgresStore("   "); err == nil {
		t.Fatalf("expected whitespace dsn to fail")
	}
}

func TestPostgresStoreApplyEnvelopeUpsertsAllBuckets(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	store.nowFn = func() time.Time { return now }
	env := &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "R351ZABAPH331057",
		ObservedTimeUnixMs: time.Date(2026, time.February, 27, 12, 34, 56, 0, time.UTC).UnixMilli(),
		Payload:            []byte(`{"params":{"wattsInSum":259,"pv1ChargeWatts":52,"wattsOutSum":217,"f32ShowSoc":25.5}}`),
		Labels:             map[string]string{"provider": "ecoflow"},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(store.minuteUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.hourUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.dayUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.ApplyEnvelope(context.Background(), env); err != nil {
		t.Fatalf("ApplyEnvelope failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreApplyEnvelopeRollsBackOnExecFailure(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	env := &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "R351ZABAPH331057",
		ObservedTimeUnixMs: time.Date(2026, time.February, 27, 12, 34, 56, 0, time.UTC).UnixMilli(),
		Payload:            []byte(`{"params":{"wattsOutSum":217}}`),
		Labels:             map[string]string{"provider": "ecoflow"},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(store.minuteUpsertQuery)).WillReturnError(context.DeadlineExceeded)
	mock.ExpectRollback()

	if err := store.ApplyEnvelope(context.Background(), env); err == nil {
		t.Fatalf("expected ApplyEnvelope to fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreApplyEnvelopeIntegratesSolarAcrossSparseMinuteBoundary(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.March, 7, 13, 0, 0, 0, time.UTC)
	store.nowFn = func() time.Time { return now }
	first := &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "Y711ZABA9H2P0294",
		ObservedTimeUnixMs: time.Date(2026, time.March, 7, 12, 0, 30, 0, time.UTC).UnixMilli(),
		Payload:            []byte(`{"params":{"inLvMpptPwr":120}}`),
		Labels:             map[string]string{"provider": "ecoflow"},
	}
	second := &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "Y711ZABA9H2P0294",
		ObservedTimeUnixMs: time.Date(2026, time.March, 7, 12, 1, 15, 0, time.UTC).UnixMilli(),
		Payload:            []byte(`{"params":{"inLvMpptPwr":60}}`),
		Labels:             map[string]string{"provider": "ecoflow"},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(store.minuteUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.hourUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.dayUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(store.minuteUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.hourUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.dayUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.minuteSolarUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.minuteSolarUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.hourSolarUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(store.daySolarUpsertQuery)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.ApplyEnvelope(context.Background(), first); err != nil {
		t.Fatalf("first ApplyEnvelope failed: %v", err)
	}
	if err := store.ApplyEnvelope(context.Background(), second); err != nil {
		t.Fatalf("second ApplyEnvelope failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

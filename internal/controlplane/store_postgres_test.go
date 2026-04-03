package controlplane

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresStoreListUserDevicesRetriesTransientPressure(t *testing.T) {
	t.Setenv("DB_READ_RETRY_MAX_ATTEMPTS", "2")
	t.Setenv("DB_READ_RETRY_INITIAL_BACKOFF", "1ms")
	t.Setenv("DB_READ_RETRY_MAX_BACKOFF", "1ms")
	t.Setenv("DB_READ_RETRY_JITTER_FACTOR", "0")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	query := `
SELECT
	d.id::text,
	d.ecoflow_sn,
	COALESCE(d.product_name, ''),
	COALESCE(d.model, ''),
	ud.role,
	d.created_at,
	d.updated_at
FROM user_devices ud
JOIN users u ON u.id = ud.user_id
JOIN devices d ON d.id = ud.device_id
WHERE u.keycloak_subject = $1
ORDER BY d.product_name ASC, d.ecoflow_sn ASC;
`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("subject-123").
		WillReturnError(errors.New("FATAL: sorry, too many clients already (SQLSTATE 53300)"))

	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id",
		"ecoflow_sn",
		"product_name",
		"model",
		"role",
		"created_at",
		"updated_at",
	}).AddRow(
		"018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		"SN-001",
		"PowerPulse",
		"Model X",
		"viewer",
		now,
		now,
	)
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("subject-123").
		WillReturnRows(rows)

	got, err := store.ListUserDevices(context.Background(), ListUserDevicesInput{UserSubject: "subject-123"})
	if err != nil {
		t.Fatalf("ListUserDevices failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("device count mismatch: got=%d want=1", len(got))
	}
	if got[0].DeviceID != "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52" {
		t.Fatalf("device id mismatch: got=%s", got[0].DeviceID)
	}
	if got[0].Role != "viewer" {
		t.Fatalf("role mismatch: got=%s want=%s", got[0].Role, "viewer")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

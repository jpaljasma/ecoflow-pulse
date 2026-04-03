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

func TestPostgresStoreCreateProviderCredentialActiveHandlesEmptyExcludeID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.April, 3, 8, 15, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	userID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"
	credentialID := "019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111"
	writeTime := normalizeWriteTime(now)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id::text FROM users WHERE keycloak_subject = $1`)).
		WithArgs("subject-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))

	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE provider_credentials
SET is_active = FALSE,
    updated_at = $3
WHERE user_id = $1::uuid
  AND provider = $2
  AND is_active = TRUE;
`)).
		WithArgs(userID, ProviderEcoFlow, writeTime).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO provider_credentials (
	user_id,
	provider,
	access_key_ciphertext,
	secret_key_ciphertext,
	access_key_hash,
	access_key_mask,
	is_active,
	created_at,
	updated_at
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING id::text, user_id::text, provider, access_key_mask, is_active, created_at, updated_at;
`)).
		WithArgs(
			userID,
			ProviderEcoFlow,
			[]byte("ACCESSKEY123"),
			[]byte("SECRETKEY123"),
			HashAccessKey("ACCESSKEY123"),
			MaskAccessKey("ACCESSKEY123"),
			true,
			writeTime,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"provider",
			"access_key_mask",
			"is_active",
			"created_at",
			"updated_at",
		}).AddRow(
			credentialID,
			userID,
			ProviderEcoFlow,
			MaskAccessKey("ACCESSKEY123"),
			true,
			writeTime,
			writeTime,
		))

	mock.ExpectExec(regexp.QuoteMeta(`
UPDATE provider_devices pd
SET credential_id = $4::uuid,
    updated_at = $3
FROM provider_credentials pc
WHERE pd.credential_id = pc.id
  AND pc.user_id = $1::uuid
  AND pc.provider = $2
  AND pd.provider = $2
  AND pd.credential_id <> $4::uuid;
`)).
		WithArgs(userID, ProviderEcoFlow, writeTime, credentialID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectCommit()

	got, err := store.CreateProviderCredential(context.Background(), CreateProviderCredentialInput{
		UserSubject: "subject-123",
		Provider:    ProviderEcoFlow,
		AccessKey:   "ACCESSKEY123",
		SecretKey:   "SECRETKEY123",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("CreateProviderCredential failed: %v", err)
	}
	if got.ID != credentialID {
		t.Fatalf("credential id mismatch: got=%s want=%s", got.ID, credentialID)
	}
	if !got.IsActive {
		t.Fatalf("expected active credential")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

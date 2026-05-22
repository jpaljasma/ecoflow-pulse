package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresStoreRequireCurrentSchemaFailsWhenProviderConfigColumnMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(providerConfigColumnSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"data_type", "is_nullable"}))

	err = store.RequireCurrentSchema(context.Background())
	if err == nil {
		t.Fatal("expected missing provider_config column error")
	}
	if !errors.Is(err, ErrSchemaNotReady) {
		t.Fatalf("error=%v, want ErrSchemaNotReady", err)
	}
	if !strings.Contains(err.Error(), "provider_credentials.provider_config") {
		t.Fatalf("error=%q, want provider_config context", err.Error())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestProviderConfigColumnSchemaQueryUsesSearchPathResolution(t *testing.T) {
	if !strings.Contains(providerConfigColumnSchemaQuery, "to_regclass('provider_credentials')") {
		t.Fatal("schema gate should resolve provider_credentials through the active search_path")
	}
	if !strings.Contains(providerDeviceUniqueConstraintsSchemaQuery, "to_regclass('provider_devices')") {
		t.Fatal("schema gate should resolve provider_devices through the active search_path")
	}
	if !strings.Contains(deviceEcoflowSNUniqueIndexSchemaQuery, "to_regclass('devices')") {
		t.Fatal("schema gate should resolve devices through the active search_path")
	}
	if !strings.Contains(userDeviceUniqueIndexSchemaQuery, "to_regclass('user_devices')") {
		t.Fatal("schema gate should resolve user_devices through the active search_path")
	}
	if strings.Contains(providerConfigColumnSchemaQuery, "current_schema()") {
		t.Fatal("schema gate must not inspect only current_schema()")
	}
	if strings.Contains(providerDeviceUniqueConstraintsSchemaQuery, "current_schema()") {
		t.Fatal("schema gate must not inspect only current_schema()")
	}
	if strings.Contains(deviceEcoflowSNUniqueIndexSchemaQuery, "current_schema()") {
		t.Fatal("schema gate must not inspect only current_schema()")
	}
	if strings.Contains(userDeviceUniqueIndexSchemaQuery, "current_schema()") {
		t.Fatal("schema gate must not inspect only current_schema()")
	}
}

func TestPostgresStoreRequireCurrentSchemaAcceptsProviderConfigColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(providerConfigColumnSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"data_type", "is_nullable"}).AddRow("jsonb", "NO"))
	mock.ExpectQuery(regexp.QuoteMeta(providerDeviceUniqueConstraintsSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"conname"}).
			AddRow("uq_provider_devices_device_provider").
			AddRow("uq_provider_devices_provider_device_id"))
	mock.ExpectQuery(regexp.QuoteMeta(deviceEcoflowSNUniqueIndexSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(userDeviceUniqueIndexSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	if err := store.RequireCurrentSchema(context.Background()); err != nil {
		t.Fatalf("RequireCurrentSchema failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreRequireCurrentSchemaFailsWhenProviderDeviceUniqueConstraintsMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(providerConfigColumnSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"data_type", "is_nullable"}).AddRow("jsonb", "NO"))
	mock.ExpectQuery(regexp.QuoteMeta(providerDeviceUniqueConstraintsSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"conname"}).AddRow("uq_provider_devices_device_provider"))

	err = store.RequireCurrentSchema(context.Background())
	if err == nil {
		t.Fatal("expected missing provider_devices unique constraint error")
	}
	if !errors.Is(err, ErrSchemaNotReady) {
		t.Fatalf("error=%v, want ErrSchemaNotReady", err)
	}
	if !strings.Contains(err.Error(), "uq_provider_devices_provider_device_id") {
		t.Fatalf("error=%q, want missing provider device unique constraint context", err.Error())
	}
}

func TestPostgresStoreRequireCurrentSchemaFailsWhenDeviceSerialUniqueConstraintMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(providerConfigColumnSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"data_type", "is_nullable"}).AddRow("jsonb", "NO"))
	mock.ExpectQuery(regexp.QuoteMeta(providerDeviceUniqueConstraintsSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"conname"}).
			AddRow("uq_provider_devices_device_provider").
			AddRow("uq_provider_devices_provider_device_id"))
	mock.ExpectQuery(regexp.QuoteMeta(deviceEcoflowSNUniqueIndexSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err = store.RequireCurrentSchema(context.Background())
	if err == nil {
		t.Fatal("expected missing devices.ecoflow_sn unique constraint error")
	}
	if !errors.Is(err, ErrSchemaNotReady) {
		t.Fatalf("error=%v, want ErrSchemaNotReady", err)
	}
	if !strings.Contains(err.Error(), "devices.ecoflow_sn") {
		t.Fatalf("error=%q, want missing device serial unique constraint context", err.Error())
	}
}

func TestPostgresStoreRequireCurrentSchemaFailsWhenUserDeviceUniqueConstraintMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(providerConfigColumnSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"data_type", "is_nullable"}).AddRow("jsonb", "NO"))
	mock.ExpectQuery(regexp.QuoteMeta(providerDeviceUniqueConstraintsSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"conname"}).
			AddRow("uq_provider_devices_device_provider").
			AddRow("uq_provider_devices_provider_device_id"))
	mock.ExpectQuery(regexp.QuoteMeta(deviceEcoflowSNUniqueIndexSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(userDeviceUniqueIndexSchemaQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err = store.RequireCurrentSchema(context.Background())
	if err == nil {
		t.Fatal("expected missing user_devices unique constraint error")
	}
	if !errors.Is(err, ErrSchemaNotReady) {
		t.Fatalf("error=%v, want ErrSchemaNotReady", err)
	}
	if !strings.Contains(err.Error(), "user_devices(user_id, device_id)") {
		t.Fatalf("error=%q, want missing user device unique constraint context", err.Error())
	}
}

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

func TestPostgresStoreSearchAdminLogUserFiltersReturnsAllDeviceIDsForLimitedUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	deviceIDs := make([]string, 40)
	for i := range deviceIDs {
		deviceIDs[i] = "00000000-0000-7000-8000-0000000000" + string(rune('a'+i%26))
	}
	deviceIDsJSON := `["` + strings.Join(deviceIDs, `","`) + `"]`
	mock.ExpectQuery("WITH matching_users").
		WithArgs("operator", "%operator%", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "display_name", "device_ids_json"}).
			AddRow("user-1", "operator@example.invalid", "", deviceIDsJSON))

	options, err := store.SearchAdminLogFilters(context.Background(), SearchAdminLogFiltersInput{
		Query: "operator",
		Kind:  "user",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("SearchAdminLogFilters failed: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("len(options)=%d, want 1", len(options))
	}
	if got, want := len(options[0].DeviceIDs), len(deviceIDs); got != want {
		t.Fatalf("len(DeviceIDs)=%d, want %d", got, want)
	}
	if options[0].SecondaryLabel != "40 devices" {
		t.Fatalf("SecondaryLabel=%q, want 40 devices", options[0].SecondaryLabel)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreListProviderDevicesRetriesConnectionDrop(t *testing.T) {
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
		pd.id::text,
		pd.device_id::text,
		pd.provider,
		pd.provider_device_id,
		pd.credential_id::text,
		d.ecoflow_sn,
		COALESCE(pd.product_name, d.product_name, ''),
		COALESCE(pd.model, d.model, ''),
		pd.capabilities,
		pd.metadata,
		pd.is_active,
		pd.ingest_desired_state
	FROM provider_devices pd
	JOIN devices d ON d.id = pd.device_id
	JOIN user_devices ud ON ud.device_id = d.id
	JOIN users u ON u.id = ud.user_id
	WHERE u.keycloak_subject = $1
	  AND ($2 = '' OR pd.provider = $2)
	  AND (NOT $3 OR pd.is_active = TRUE)
	ORDER BY pd.provider ASC, d.product_name ASC, d.ecoflow_sn ASC;
	`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("subject-123", ProviderEcoFlow, false).
		WillReturnError(errors.New("failed to receive message: unexpected EOF"))

	rows := sqlmock.NewRows([]string{
		"id",
		"device_id",
		"provider",
		"provider_device_id",
		"credential_id",
		"ecoflow_sn",
		"product_name",
		"model",
		"capabilities",
		"metadata",
		"is_active",
		"ingest_desired_state",
	}).AddRow(
		"provider-device-1",
		"device-1",
		ProviderEcoFlow,
		"provider-device-id-1",
		"credential-1",
		"SN-001",
		"PowerPulse",
		"Model X",
		"{}",
		"{}",
		true,
		"active",
	)
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("subject-123", ProviderEcoFlow, false).
		WillReturnRows(rows)

	got, err := store.ListProviderDevices(context.Background(), ListProviderDevicesInput{
		UserSubject: "subject-123",
		Provider:    ProviderEcoFlow,
	})
	if err != nil {
		t.Fatalf("ListProviderDevices failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("provider device count mismatch: got=%d want=1", len(got))
	}
	if got[0].DeviceID != "device-1" {
		t.Fatalf("device id mismatch: got=%s", got[0].DeviceID)
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
	provider_config,
	is_active,
	created_at,
	updated_at
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id::text, user_id::text, provider, access_key_mask, provider_config, is_active, created_at, updated_at;
`)).
		WithArgs(
			userID,
			ProviderEcoFlow,
			[]byte("ACCESSKEY123"),
			[]byte("SECRETKEY123"),
			HashAccessKey("ACCESSKEY123"),
			MaskAccessKey("ACCESSKEY123"),
			[]byte("{}"),
			true,
			writeTime,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"provider",
			"access_key_mask",
			"provider_config",
			"is_active",
			"created_at",
			"updated_at",
		}).AddRow(
			credentialID,
			userID,
			ProviderEcoFlow,
			MaskAccessKey("ACCESSKEY123"),
			[]byte("{}"),
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

func TestPostgresStoreImportProviderDeviceCommitsAtomicDeviceAndProviderRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.May, 20, 18, 45, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	userID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"
	deviceID := "019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e222"
	credentialID := "019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111"
	providerDeviceRowID := "019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e333"
	writeTime := normalizeWriteTime(now)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id::text FROM users WHERE keycloak_subject = $1`)).
		WithArgs("subject-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO devices (ecoflow_sn, product_name, model, metadata, created_at, updated_at)
VALUES ($1, NULLIF(BTRIM($2), ''), NULLIF(BTRIM($3), ''), '{}'::jsonb, $4, $4)
ON CONFLICT (ecoflow_sn)
DO UPDATE
SET product_name = COALESCE(EXCLUDED.product_name, devices.product_name),
	model = COALESCE(EXCLUDED.model, devices.model),
	updated_at = EXCLUDED.updated_at
RETURNING id::text, ecoflow_sn, COALESCE(product_name, ''), COALESCE(model, ''), created_at, updated_at;
`)).
		WithArgs("PECRON-P11VXG-TESTDEVICE0001", "Pecron E1000LFP", "E1000LFP", writeTime).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"ecoflow_sn",
			"product_name",
			"model",
			"created_at",
			"updated_at",
		}).AddRow(
			deviceID,
			"PECRON-P11VXG-TESTDEVICE0001",
			"Pecron E1000LFP",
			"E1000LFP",
			writeTime,
			writeTime,
		))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, 'admin', $3, $3)
ON CONFLICT (user_id, device_id)
DO UPDATE SET role = 'admin', updated_at = EXCLUDED.updated_at;
`)).
		WithArgs(userID, deviceID, writeTime).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO provider_devices (
	device_id,
	provider,
	provider_device_id,
	credential_id,
	product_name,
	model,
	capabilities,
	metadata,
	is_active,
	ingest_desired_state,
	created_at,
	updated_at
)
VALUES (
	$1::uuid,
	$2,
	$3,
	$4::uuid,
	NULLIF(BTRIM($5), ''),
	NULLIF(BTRIM($6), ''),
	COALESCE($7::jsonb, '{}'::jsonb),
	COALESCE($8::jsonb, '{}'::jsonb),
	$9,
	$10,
	$11,
	$11
)
ON CONFLICT (provider, provider_device_id)
DO UPDATE
SET device_id = EXCLUDED.device_id,
	credential_id = EXCLUDED.credential_id,
	product_name = COALESCE(EXCLUDED.product_name, provider_devices.product_name),
	model = COALESCE(EXCLUDED.model, provider_devices.model),
	capabilities = CASE
		WHEN $7::jsonb IS NULL THEN provider_devices.capabilities
		ELSE $7::jsonb
	END,
	metadata = CASE
		WHEN $8::jsonb IS NULL THEN provider_devices.metadata
		ELSE $8::jsonb
	END,
	is_active = EXCLUDED.is_active,
	ingest_desired_state = EXCLUDED.ingest_desired_state,
	updated_at = EXCLUDED.updated_at
RETURNING id::text, device_id::text, provider, provider_device_id, credential_id::text, COALESCE(product_name, ''), COALESCE(model, ''), capabilities, metadata, is_active, ingest_desired_state;
`)).
		WithArgs(
			deviceID,
			ProviderPecron,
			"p11vxg:testdevice0001",
			credentialID,
			"Pecron E1000LFP",
			"E1000LFP",
			[]byte(`{"mqtt":true}`),
			[]byte(`{"source":"probe"}`),
			true,
			"active",
			writeTime,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"device_id",
			"provider",
			"provider_device_id",
			"credential_id",
			"product_name",
			"model",
			"capabilities",
			"metadata",
			"is_active",
			"ingest_desired_state",
		}).AddRow(
			providerDeviceRowID,
			deviceID,
			ProviderPecron,
			"p11vxg:testdevice0001",
			credentialID,
			"Pecron E1000LFP",
			"E1000LFP",
			`{"mqtt":true}`,
			`{"source":"probe"}`,
			true,
			"active",
		))
	mock.ExpectCommit()

	got, err := store.ImportProviderDevice(context.Background(), ImportProviderDeviceInput{
		UserSubject:        "subject-123",
		Provider:           ProviderPecron,
		ProviderDeviceID:   "p11vxg:testdevice0001",
		CredentialID:       credentialID,
		CanonicalSN:        "PECRON-P11VXG-TESTDEVICE0001",
		ProductName:        "Pecron E1000LFP",
		Model:              "E1000LFP",
		Capabilities:       map[string]any{"mqtt": true},
		Metadata:           map[string]any{"source": "probe"},
		IsActive:           true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("ImportProviderDevice failed: %v", err)
	}
	if got.UserDevice.DeviceID != deviceID {
		t.Fatalf("user device id mismatch: got=%s want=%s", got.UserDevice.DeviceID, deviceID)
	}
	if got.ProviderDevice.ID != providerDeviceRowID {
		t.Fatalf("provider device id mismatch: got=%s want=%s", got.ProviderDevice.ID, providerDeviceRowID)
	}
	if !got.ProviderDevice.IsActive || got.ProviderDevice.IngestDesiredState != "active" {
		t.Fatalf("expected active provider device, got active=%v state=%q", got.ProviderDevice.IsActive, got.ProviderDevice.IngestDesiredState)
	}
	if got.ProviderDevice.CanonicalSN != "PECRON-P11VXG-TESTDEVICE0001" {
		t.Fatalf("canonical sn mismatch: got=%s", got.ProviderDevice.CanonicalSN)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreReconcileUserSubjectByEmailRemapsVerifiedUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.April, 15, 12, 30, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at
FROM users
WHERE keycloak_subject = $1;
`)).
		WithArgs("cloud-subject-123").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery(regexp.QuoteMeta(`
UPDATE users
SET keycloak_subject = $2,
	updated_at = $3
WHERE lower(COALESCE(email, '')) = lower($1)
  AND email_verified = TRUE
RETURNING
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at;
`)).
		WithArgs("jpaljasma@gmail.com", "cloud-subject-123", normalizeWriteTime(now)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"keycloak_subject",
			"email",
			"email_verified",
			"display_name",
			"display_name_source",
			"avatar_url",
			"given_name",
			"family_name",
			"locale",
			"timezone",
			"weather_location_enabled",
			"weather_location_source",
			"weather_location_label",
			"weather_latitude",
			"weather_longitude",
			"last_login_at",
			"created_at",
			"updated_at",
		}).AddRow(
			"user-1",
			"cloud-subject-123",
			"jpaljasma@gmail.com",
			true,
			"JP",
			"provider",
			"",
			"Jaan",
			"Paljasma",
			"en-US",
			"America/New_York",
			false,
			"none",
			"",
			nil,
			nil,
			now,
			now,
			normalizeWriteTime(now),
		))
	mock.ExpectCommit()

	got, err := store.ReconcileUserSubjectByEmail(context.Background(), ReconcileUserSubjectByEmailInput{
		Email:       "jpaljasma@gmail.com",
		UserSubject: "cloud-subject-123",
	})
	if err != nil {
		t.Fatalf("ReconcileUserSubjectByEmail failed: %v", err)
	}
	if got.KeycloakSubject != "cloud-subject-123" {
		t.Fatalf("keycloak subject mismatch: got=%s", got.KeycloakSubject)
	}
	if got.Email != "jpaljasma@gmail.com" {
		t.Fatalf("email mismatch: got=%s", got.Email)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresStoreReconcileUserSubjectByEmailRejectsConflictingSubject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := newPostgresStore(db)
	now := time.Date(2026, time.April, 15, 12, 30, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at
FROM users
WHERE keycloak_subject = $1;
`)).
		WithArgs("cloud-subject-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"keycloak_subject",
			"email",
			"email_verified",
			"display_name",
			"display_name_source",
			"avatar_url",
			"given_name",
			"family_name",
			"locale",
			"timezone",
			"weather_location_enabled",
			"weather_location_source",
			"weather_location_label",
			"weather_latitude",
			"weather_longitude",
			"last_login_at",
			"created_at",
			"updated_at",
		}).AddRow(
			"user-2",
			"cloud-subject-123",
			"someone-else@example.com",
			true,
			"Someone Else",
			"provider",
			"",
			"Someone",
			"Else",
			"en-US",
			"America/New_York",
			false,
			"none",
			"",
			nil,
			nil,
			now,
			now,
			now,
		))
	mock.ExpectRollback()

	_, err = store.ReconcileUserSubjectByEmail(context.Background(), ReconcileUserSubjectByEmailInput{
		Email:       "jpaljasma@gmail.com",
		UserSubject: "cloud-subject-123",
	})
	if !errors.Is(err, ErrUserSubjectConflict) {
		t.Fatalf("expected ErrUserSubjectConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

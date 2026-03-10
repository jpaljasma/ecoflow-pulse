package integrationtest

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestMigrationsCycleAndE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skip migration CI integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	opts := DefaultStackOptions()
	stack, err := StartPostgresStack(ctx, opts)
	if err != nil {
		t.Fatalf("start postgres integration stack: %v", err)
	}
	t.Cleanup(func() {
		_ = stack.Terminate(context.Background())
	})

	db, err := OpenPostgres(ctx, stack.PostgresDSN)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		closeErr := db.Close()
		if closeErr != nil {
			t.Errorf("close postgres: %v", closeErr)
		}
	})

	assertSchemaState(t, ctx, db)
	if err := ApplyDownMigrations(ctx, stack.PostgresDSN, opts.MigrationsDir); err != nil {
		t.Fatalf("apply down migrations: %v", err)
	}
	assertTablesAbsent(t, ctx, db)
	if err := ApplyMigrations(ctx, stack.PostgresDSN, opts.MigrationsDir); err != nil {
		t.Fatalf("reapply up migrations: %v", err)
	}
	assertSchemaState(t, ctx, db)
	assertMigrationE2E(t, ctx, db)
}

func assertSchemaState(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	wantTables := []string{
		"archive_object_manifest",
		"devices",
		"provider_credentials",
		"provider_devices",
		"telemetry_rollup_day",
		"telemetry_rollup_hour",
		"telemetry_rollup_minute",
		"user_devices",
		"users",
	}
	gotTables := queryStrings(t, ctx, db, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','devices','user_devices','provider_credentials','provider_devices','archive_object_manifest','telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY table_name")
	assertStringsEqual(t, gotTables, wantTables, "tables")

	gotIDDefault := queryStrings(t, ctx, db, "SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef d JOIN pg_class c ON c.oid=d.adrelid JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=d.adnum WHERE c.relname='users' AND a.attname='id'")
	assertStringsEqual(t, gotIDDefault, []string{"uuidv7()"}, "users.id default")

	wantHypertables := []string{
		"telemetry_rollup_day",
		"telemetry_rollup_hour",
		"telemetry_rollup_minute",
	}
	gotHypertables := queryStrings(t, ctx, db, "SELECT hypertable_name FROM timescaledb_information.hypertables WHERE hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY hypertable_name")
	assertStringsEqual(t, gotHypertables, wantHypertables, "hypertables")

	wantRetention := []string{
		"telemetry_rollup_day|3 years|1 day",
		"telemetry_rollup_hour|3 years|1 day",
		"telemetry_rollup_minute|90 days|1 day",
	}
	gotRetention := queryStrings(t, ctx, db, "SELECT hypertable_name || '|' || (config->>'drop_after') || '|' || schedule_interval::text FROM timescaledb_information.jobs WHERE proc_name='policy_retention' AND hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY hypertable_name")
	assertStringsEqual(t, gotRetention, wantRetention, "retention policies")

	wantTimestampDefaults := []string{
		"devices.created_at|",
		"devices.updated_at|",
		"provider_credentials.created_at|",
		"provider_credentials.updated_at|",
		"provider_devices.created_at|",
		"provider_devices.updated_at|",
		"users.created_at|",
		"users.updated_at|",
	}
	gotTimestampDefaults := queryStrings(t, ctx, db, `
SELECT c.relname || '.' || a.attname || '|' || COALESCE(pg_get_expr(d.adbin,d.adrelid), '')
FROM pg_attribute a
LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
JOIN pg_class c ON c.oid=a.attrelid
WHERE c.relname IN ('users','devices','provider_credentials','provider_devices')
  AND a.attname IN ('created_at','updated_at')
ORDER BY c.relname, a.attname`)
	assertStringsEqual(t, gotTimestampDefaults, wantTimestampDefaults, "timestamp defaults")

	wantConstraints := []string{
		"chk_archive_manifest_ts_order",
		"chk_devices_ecoflow_sn_nonempty",
		"chk_rollup_day_sample_count_nonnegative",
		"chk_rollup_hour_sample_count_nonnegative",
		"chk_rollup_minute_sample_count_nonnegative",
		"chk_user_devices_role",
		"chk_users_keycloak_subject_nonempty",
		"pk_rollup_day",
		"pk_rollup_hour",
		"pk_rollup_minute",
		"uq_archive_manifest_bucket_key",
	}
	gotConstraints := queryStrings(t, ctx, db, "SELECT conname FROM pg_constraint WHERE conname IN ('chk_user_devices_role','chk_devices_ecoflow_sn_nonempty','chk_users_keycloak_subject_nonempty','uq_archive_manifest_bucket_key','chk_archive_manifest_ts_order','pk_rollup_minute','pk_rollup_hour','pk_rollup_day','chk_rollup_minute_sample_count_nonnegative','chk_rollup_hour_sample_count_nonnegative','chk_rollup_day_sample_count_nonnegative') ORDER BY conname")
	assertStringsEqual(t, gotConstraints, wantConstraints, "schema constraints")
}

func assertTablesAbsent(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	gotTables := queryStrings(t, ctx, db, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','devices','user_devices','provider_credentials','provider_devices','archive_object_manifest','telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY table_name")
	if len(gotTables) != 0 {
		t.Fatalf("expected migrated tables to be absent after down migrations, got=%v", gotTables)
	}
}

func assertMigrationE2E(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, "TRUNCATE archive_object_manifest, user_devices, provider_devices, provider_credentials, users, devices RESTART IDENTITY CASCADE;"); err != nil {
		t.Fatalf("truncate migration tables: %v", err)
	}

	seedSQL := "WITH u AS (INSERT INTO users (keycloak_subject, email, display_name, created_at, updated_at) VALUES ('kc-sub-e2e-1','e2e1@example.com','E2E User 1', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC') RETURNING id), d AS (INSERT INTO devices (ecoflow_sn, product_name, model, created_at, updated_at) VALUES ('SN-E2E-0001','DELTA Pro Ultra','dpu', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC') RETURNING id) INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at) SELECT u.id, d.id, 'viewer', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC' FROM u CROSS JOIN d;"
	if _, err := db.ExecContext(ctx, seedSQL); err != nil {
		t.Fatalf("seed e2e rows: %v", err)
	}

	gotJoin := queryStrings(t, ctx, db, "SELECT u.keycloak_subject || '|' || d.ecoflow_sn || '|' || ud.role FROM users u JOIN user_devices ud ON ud.user_id = u.id JOIN devices d ON d.id = ud.device_id WHERE u.keycloak_subject = 'kc-sub-e2e-1'")
	assertStringsEqual(t, gotJoin, []string{"kc-sub-e2e-1|SN-E2E-0001|viewer"}, "ownership join")

	expectExecError(t, ctx, db, "INSERT INTO users (keycloak_subject, created_at, updated_at) VALUES ('kc-sub-e2e-1', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC');")
	expectExecError(t, ctx, db, "INSERT INTO devices (ecoflow_sn, created_at, updated_at) VALUES ('SN-E2E-0001', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC');")
	expectExecError(t, ctx, db, "INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at) SELECT user_id, device_id, 'owner', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC' FROM user_devices LIMIT 1;")
}

func expectExecError(t *testing.T, ctx context.Context, db *sql.DB, query string) {
	t.Helper()

	if _, err := db.ExecContext(ctx, query); err == nil {
		t.Fatalf("expected query to fail: %s", query)
	}
}

func queryStrings(t *testing.T, ctx context.Context, db *sql.DB, query string) []string {
	t.Helper()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			t.Errorf("close rows: %v", closeErr)
		}
	}()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}
	return values
}

func assertStringsEqual(t *testing.T, got []string, want []string, label string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected %s length: got=%v want=%v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected %s: got=%v want=%v", label, got, want)
		}
	}
}

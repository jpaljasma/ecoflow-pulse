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
		"solar_forecast_calibration_state",
		"solar_forecast_hourly_training_records",
		"solar_forecast_runs",
		"solar_forecast_verification_daily",
		"telemetry_rollup_day",
		"telemetry_rollup_hour",
		"telemetry_rollup_minute",
		"telemetry_rollup_pv_port_day",
		"telemetry_rollup_pv_port_hour",
		"telemetry_rollup_pv_port_minute",
		"user_devices",
		"users",
	}
	gotTables := queryStrings(t, ctx, db, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','devices','user_devices','provider_credentials','provider_devices','archive_object_manifest','solar_forecast_runs','solar_forecast_hourly_training_records','solar_forecast_verification_daily','solar_forecast_calibration_state','telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day','telemetry_rollup_pv_port_minute','telemetry_rollup_pv_port_hour','telemetry_rollup_pv_port_day') ORDER BY table_name")
	assertStringsEqual(t, gotTables, wantTables, "tables")

	gotIDDefault := queryStrings(t, ctx, db, "SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef d JOIN pg_class c ON c.oid=d.adrelid JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=d.adnum WHERE c.relname='users' AND a.attname='id'")
	assertStringsEqual(t, gotIDDefault, []string{"uuidv7()"}, "users.id default")

	wantHypertables := []string{
		"telemetry_rollup_day",
		"telemetry_rollup_hour",
		"telemetry_rollup_minute",
		"telemetry_rollup_pv_port_day",
		"telemetry_rollup_pv_port_hour",
		"telemetry_rollup_pv_port_minute",
	}
	gotHypertables := queryStrings(t, ctx, db, "SELECT hypertable_name FROM timescaledb_information.hypertables WHERE hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day','telemetry_rollup_pv_port_minute','telemetry_rollup_pv_port_hour','telemetry_rollup_pv_port_day') ORDER BY hypertable_name")
	assertStringsEqual(t, gotHypertables, wantHypertables, "hypertables")

	wantRetention := []string{
		"telemetry_rollup_day|3 years|1 day",
		"telemetry_rollup_hour|3 years|1 day",
		"telemetry_rollup_minute|90 days|1 day",
		"telemetry_rollup_pv_port_day|3 years|1 day",
		"telemetry_rollup_pv_port_hour|3 years|1 day",
		"telemetry_rollup_pv_port_minute|90 days|1 day",
	}
	gotRetention := queryStrings(t, ctx, db, "SELECT hypertable_name || '|' || (config->>'drop_after') || '|' || schedule_interval::text FROM timescaledb_information.jobs WHERE proc_name='policy_retention' AND hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day','telemetry_rollup_pv_port_minute','telemetry_rollup_pv_port_hour','telemetry_rollup_pv_port_day') ORDER BY hypertable_name")
	assertStringsEqual(t, gotRetention, wantRetention, "retention policies")

	wantTimestampDefaults := []string{
		"devices.created_at|",
		"devices.updated_at|",
		"provider_credentials.created_at|",
		"provider_credentials.updated_at|",
		"provider_devices.created_at|",
		"provider_devices.updated_at|",
		"solar_forecast_calibration_state.updated_at|",
		"solar_forecast_hourly_training_records.created_at|",
		"solar_forecast_hourly_training_records.updated_at|",
		"solar_forecast_runs.created_at|",
		"solar_forecast_runs.updated_at|",
		"solar_forecast_verification_daily.created_at|",
		"solar_forecast_verification_daily.updated_at|",
		"users.created_at|",
		"users.updated_at|",
	}
	gotTimestampDefaults := queryStrings(t, ctx, db, `
SELECT c.relname || '.' || a.attname || '|' || COALESCE(pg_get_expr(d.adbin,d.adrelid), '')
FROM pg_attribute a
LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
JOIN pg_class c ON c.oid=a.attrelid
WHERE c.relname IN ('users','devices','provider_credentials','provider_devices','solar_forecast_runs','solar_forecast_hourly_training_records','solar_forecast_verification_daily','solar_forecast_calibration_state')
  AND a.attname IN ('created_at','updated_at')
ORDER BY c.relname, a.attname`)
	assertStringsEqual(t, gotTimestampDefaults, wantTimestampDefaults, "timestamp defaults")

	wantConstraints := []string{
		"chk_archive_manifest_ts_order",
		"chk_devices_ecoflow_sn_nonempty",
		"chk_rollup_day_sample_count_nonnegative",
		"chk_rollup_hour_sample_count_nonnegative",
		"chk_rollup_minute_sample_count_nonnegative",
		"chk_rollup_pv_port_day_port_id_nonempty",
		"chk_rollup_pv_port_day_port_label_nonempty",
		"chk_rollup_pv_port_day_provider_device_id_nonempty",
		"chk_rollup_pv_port_day_provider_nonempty",
		"chk_rollup_pv_port_day_sample_count_positive",
		"chk_rollup_pv_port_day_ts_order",
		"chk_rollup_pv_port_hour_port_id_nonempty",
		"chk_rollup_pv_port_hour_port_label_nonempty",
		"chk_rollup_pv_port_hour_provider_device_id_nonempty",
		"chk_rollup_pv_port_hour_provider_nonempty",
		"chk_rollup_pv_port_hour_sample_count_positive",
		"chk_rollup_pv_port_hour_ts_order",
		"chk_rollup_pv_port_minute_port_id_nonempty",
		"chk_rollup_pv_port_minute_port_label_nonempty",
		"chk_rollup_pv_port_minute_provider_device_id_nonempty",
		"chk_rollup_pv_port_minute_provider_nonempty",
		"chk_rollup_pv_port_minute_sample_count_positive",
		"chk_rollup_pv_port_minute_ts_order",
		"chk_solar_forecast_calibration_horizon_bucket",
		"chk_solar_forecast_calibration_hour",
		"chk_solar_forecast_calibration_ratio_positive",
		"chk_solar_forecast_calibration_sample_count_nonnegative",
		"chk_solar_forecast_calibration_site_key_nonempty",
		"chk_solar_forecast_calibration_version_nonempty",
		"chk_solar_forecast_hourly_horizon_bucket",
		"chk_solar_forecast_hourly_horizon_nonnegative",
		"chk_solar_forecast_hourly_irradiance_source",
		"chk_solar_forecast_hourly_site_key_nonempty",
		"chk_solar_forecast_hourly_target_hour",
		"chk_solar_forecast_hourly_verification_status",
		"chk_solar_forecast_runs_feature_version_nonempty",
		"chk_solar_forecast_runs_forecast_version_nonempty",
		"chk_solar_forecast_runs_issue_local_hour",
		"chk_solar_forecast_runs_scope_kind",
		"chk_solar_forecast_runs_served_variant",
		"chk_solar_forecast_runs_site_key_nonempty",
		"chk_solar_forecast_runs_timezone_nonempty",
		"chk_solar_forecast_verification_counts_nonnegative",
		"chk_solar_forecast_verification_horizon_bucket",
		"chk_solar_forecast_verification_served_variant",
		"chk_solar_forecast_verification_site_key_nonempty",
		"chk_solar_forecast_verification_timezone_nonempty",
		"chk_solar_forecast_verification_version_nonempty",
		"chk_user_devices_role",
		"chk_users_keycloak_subject_nonempty",
		"pk_rollup_day",
		"pk_rollup_hour",
		"pk_rollup_minute",
		"pk_rollup_pv_port_day",
		"pk_rollup_pv_port_hour",
		"pk_rollup_pv_port_minute",
		"uq_archive_manifest_bucket_key",
	}
	gotConstraints := queryStrings(t, ctx, db, "SELECT conname FROM pg_constraint WHERE conname IN ('chk_user_devices_role','chk_devices_ecoflow_sn_nonempty','chk_users_keycloak_subject_nonempty','uq_archive_manifest_bucket_key','chk_archive_manifest_ts_order','pk_rollup_minute','pk_rollup_hour','pk_rollup_day','chk_rollup_minute_sample_count_nonnegative','chk_rollup_hour_sample_count_nonnegative','chk_rollup_day_sample_count_nonnegative','pk_rollup_pv_port_minute','pk_rollup_pv_port_hour','pk_rollup_pv_port_day','chk_rollup_pv_port_minute_provider_nonempty','chk_rollup_pv_port_minute_provider_device_id_nonempty','chk_rollup_pv_port_minute_port_id_nonempty','chk_rollup_pv_port_minute_port_label_nonempty','chk_rollup_pv_port_minute_sample_count_positive','chk_rollup_pv_port_minute_ts_order','chk_rollup_pv_port_hour_provider_nonempty','chk_rollup_pv_port_hour_provider_device_id_nonempty','chk_rollup_pv_port_hour_port_id_nonempty','chk_rollup_pv_port_hour_port_label_nonempty','chk_rollup_pv_port_hour_sample_count_positive','chk_rollup_pv_port_hour_ts_order','chk_rollup_pv_port_day_provider_nonempty','chk_rollup_pv_port_day_provider_device_id_nonempty','chk_rollup_pv_port_day_port_id_nonempty','chk_rollup_pv_port_day_port_label_nonempty','chk_rollup_pv_port_day_sample_count_positive','chk_rollup_pv_port_day_ts_order','chk_solar_forecast_runs_scope_kind','chk_solar_forecast_runs_timezone_nonempty','chk_solar_forecast_runs_site_key_nonempty','chk_solar_forecast_runs_forecast_version_nonempty','chk_solar_forecast_runs_feature_version_nonempty','chk_solar_forecast_runs_issue_local_hour','chk_solar_forecast_runs_served_variant','chk_solar_forecast_hourly_site_key_nonempty','chk_solar_forecast_hourly_target_hour','chk_solar_forecast_hourly_horizon_nonnegative','chk_solar_forecast_hourly_horizon_bucket','chk_solar_forecast_hourly_irradiance_source','chk_solar_forecast_hourly_verification_status','chk_solar_forecast_verification_site_key_nonempty','chk_solar_forecast_verification_timezone_nonempty','chk_solar_forecast_verification_version_nonempty','chk_solar_forecast_verification_horizon_bucket','chk_solar_forecast_verification_counts_nonnegative','chk_solar_forecast_verification_served_variant','chk_solar_forecast_calibration_site_key_nonempty','chk_solar_forecast_calibration_version_nonempty','chk_solar_forecast_calibration_horizon_bucket','chk_solar_forecast_calibration_hour','chk_solar_forecast_calibration_sample_count_nonnegative','chk_solar_forecast_calibration_ratio_positive') ORDER BY conname")
	assertStringsEqual(t, gotConstraints, wantConstraints, "schema constraints")

	wantPVPortColumns := []string{
		"telemetry_rollup_pv_port_day.last_observed_at_unix_ms",
		"telemetry_rollup_pv_port_day.last_observed_watts",
		"telemetry_rollup_pv_port_day.max_observed_watts",
		"telemetry_rollup_pv_port_day.port_id",
		"telemetry_rollup_pv_port_day.port_label",
		"telemetry_rollup_pv_port_hour.last_observed_at_unix_ms",
		"telemetry_rollup_pv_port_hour.last_observed_watts",
		"telemetry_rollup_pv_port_hour.max_observed_watts",
		"telemetry_rollup_pv_port_hour.port_id",
		"telemetry_rollup_pv_port_hour.port_label",
		"telemetry_rollup_pv_port_minute.last_observed_at_unix_ms",
		"telemetry_rollup_pv_port_minute.last_observed_watts",
		"telemetry_rollup_pv_port_minute.max_observed_watts",
		"telemetry_rollup_pv_port_minute.port_id",
		"telemetry_rollup_pv_port_minute.port_label",
	}
	gotPVPortColumns := queryStrings(t, ctx, db, `
SELECT table_name || '.' || column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('telemetry_rollup_pv_port_minute', 'telemetry_rollup_pv_port_hour', 'telemetry_rollup_pv_port_day')
  AND column_name IN ('port_id', 'port_label', 'max_observed_watts', 'last_observed_watts', 'last_observed_at_unix_ms')
ORDER BY table_name, column_name`)
	assertStringsEqual(t, gotPVPortColumns, wantPVPortColumns, "pv-port columns")

	wantEnergyColumns := []string{
		"telemetry_rollup_day.ac_input_energy_wh",
		"telemetry_rollup_day.ac_output_energy_wh",
		"telemetry_rollup_day.battery_charge_energy_wh",
		"telemetry_rollup_day.battery_discharge_energy_wh",
		"telemetry_rollup_day.dc_output_energy_wh",
		"telemetry_rollup_day.load_energy_wh",
		"telemetry_rollup_hour.ac_input_energy_wh",
		"telemetry_rollup_hour.ac_output_energy_wh",
		"telemetry_rollup_hour.battery_charge_energy_wh",
		"telemetry_rollup_hour.battery_discharge_energy_wh",
		"telemetry_rollup_hour.dc_output_energy_wh",
		"telemetry_rollup_hour.load_energy_wh",
		"telemetry_rollup_minute.ac_input_energy_wh",
		"telemetry_rollup_minute.ac_output_energy_wh",
		"telemetry_rollup_minute.battery_charge_energy_wh",
		"telemetry_rollup_minute.battery_discharge_energy_wh",
		"telemetry_rollup_minute.dc_output_energy_wh",
		"telemetry_rollup_minute.load_energy_wh",
	}
	gotEnergyColumns := queryStrings(t, ctx, db, `
SELECT table_name || '.' || column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('telemetry_rollup_minute', 'telemetry_rollup_hour', 'telemetry_rollup_day')
  AND column_name IN ('ac_input_energy_wh', 'ac_output_energy_wh', 'dc_output_energy_wh', 'load_energy_wh', 'battery_charge_energy_wh', 'battery_discharge_energy_wh')
ORDER BY table_name, column_name`)
	assertStringsEqual(t, gotEnergyColumns, wantEnergyColumns, "explicit energy columns")

	wantSolarColumns := []string{
		"solar_forecast_calibration_state.forecast_version",
		"solar_forecast_calibration_state.horizon_bucket",
		"solar_forecast_calibration_state.multiplicative_ratio",
		"solar_forecast_calibration_state.sample_count",
		"solar_forecast_hourly_training_records.actual_cloud_cover_pct",
		"solar_forecast_hourly_training_records.actual_generation_wh",
		"solar_forecast_hourly_training_records.baseline_absolute_error_wh",
		"solar_forecast_hourly_training_records.baseline_forecast_generation_wh",
		"solar_forecast_hourly_training_records.baseline_squared_error_wh2",
		"solar_forecast_hourly_training_records.feature_snapshot_json",
		"solar_forecast_hourly_training_records.forecast_generation_wh",
		"solar_forecast_hourly_training_records.horizon_bucket",
		"solar_forecast_hourly_training_records.horizon_hours",
		"solar_forecast_hourly_training_records.weather_corrected_json",
		"solar_forecast_hourly_training_records.weather_raw_json",
		"solar_forecast_runs.capacity_estimate_w",
		"solar_forecast_runs.feature_version",
		"solar_forecast_runs.forecast_total_today_wh",
		"solar_forecast_runs.issue_local_date",
		"solar_forecast_runs.scope_kind",
		"solar_forecast_runs.served_variant",
		"solar_forecast_runs.site_metadata_json",
		"solar_forecast_verification_daily.baseline_daily_abs_error_wh_sum",
		"solar_forecast_verification_daily.baseline_peak_power_abs_error_w_sum",
		"solar_forecast_verification_daily.baseline_peak_time_abs_error_minutes_sum",
		"solar_forecast_verification_daily.daily_abs_error_wh_sum",
		"solar_forecast_verification_daily.forecast_hours",
		"solar_forecast_verification_daily.horizon_bucket",
		"solar_forecast_verification_daily.served_variant",
		"solar_forecast_verification_daily.verified_hours",
	}
	gotSolarColumns := queryStrings(t, ctx, db, `
SELECT table_name || '.' || column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (
    (table_name = 'solar_forecast_runs' AND column_name IN ('scope_kind', 'served_variant', 'issue_local_date', 'feature_version', 'capacity_estimate_w', 'forecast_total_today_wh', 'site_metadata_json')) OR
    (table_name = 'solar_forecast_calibration_state' AND column_name IN ('forecast_version', 'horizon_bucket', 'sample_count', 'multiplicative_ratio')) OR
    (table_name = 'solar_forecast_hourly_training_records' AND column_name IN ('forecast_generation_wh', 'baseline_forecast_generation_wh', 'actual_generation_wh', 'actual_cloud_cover_pct', 'baseline_absolute_error_wh', 'baseline_squared_error_wh2', 'horizon_hours', 'horizon_bucket', 'feature_snapshot_json', 'weather_raw_json', 'weather_corrected_json')) OR
    (table_name = 'solar_forecast_verification_daily' AND column_name IN ('forecast_hours', 'verified_hours', 'horizon_bucket', 'served_variant', 'daily_abs_error_wh_sum', 'baseline_daily_abs_error_wh_sum', 'baseline_peak_power_abs_error_w_sum', 'baseline_peak_time_abs_error_minutes_sum'))
  )
ORDER BY table_name, column_name`)
	assertStringsEqual(t, gotSolarColumns, wantSolarColumns, "solar forecast columns")
}

func assertTablesAbsent(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	gotTables := queryStrings(t, ctx, db, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','devices','user_devices','provider_credentials','provider_devices','archive_object_manifest','solar_forecast_runs','solar_forecast_hourly_training_records','solar_forecast_verification_daily','solar_forecast_calibration_state','telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day','telemetry_rollup_pv_port_minute','telemetry_rollup_pv_port_hour','telemetry_rollup_pv_port_day') ORDER BY table_name")
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

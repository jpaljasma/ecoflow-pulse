ALTER TABLE telemetry_rollup_minute
    ADD COLUMN IF NOT EXISTS ac_input_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS dc_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS load_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_charge_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_discharge_energy_wh DOUBLE PRECISION;

ALTER TABLE telemetry_rollup_hour
    ADD COLUMN IF NOT EXISTS ac_input_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS dc_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS load_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_charge_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_discharge_energy_wh DOUBLE PRECISION;

ALTER TABLE telemetry_rollup_day
    ADD COLUMN IF NOT EXISTS ac_input_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS dc_output_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS load_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_charge_energy_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS battery_discharge_energy_wh DOUBLE PRECISION;

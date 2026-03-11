ALTER TABLE telemetry_rollup_day
    DROP COLUMN IF EXISTS battery_discharge_energy_wh,
    DROP COLUMN IF EXISTS battery_charge_energy_wh,
    DROP COLUMN IF EXISTS load_energy_wh,
    DROP COLUMN IF EXISTS dc_output_energy_wh,
    DROP COLUMN IF EXISTS ac_output_energy_wh,
    DROP COLUMN IF EXISTS ac_input_energy_wh;

ALTER TABLE telemetry_rollup_hour
    DROP COLUMN IF EXISTS battery_discharge_energy_wh,
    DROP COLUMN IF EXISTS battery_charge_energy_wh,
    DROP COLUMN IF EXISTS load_energy_wh,
    DROP COLUMN IF EXISTS dc_output_energy_wh,
    DROP COLUMN IF EXISTS ac_output_energy_wh,
    DROP COLUMN IF EXISTS ac_input_energy_wh;

ALTER TABLE telemetry_rollup_minute
    DROP COLUMN IF EXISTS battery_discharge_energy_wh,
    DROP COLUMN IF EXISTS battery_charge_energy_wh,
    DROP COLUMN IF EXISTS load_energy_wh,
    DROP COLUMN IF EXISTS dc_output_energy_wh,
    DROP COLUMN IF EXISTS ac_output_energy_wh,
    DROP COLUMN IF EXISTS ac_input_energy_wh;

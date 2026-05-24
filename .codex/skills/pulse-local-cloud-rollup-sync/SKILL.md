---
name: pulse-local-cloud-rollup-sync
description: Use when Pulse cloud rollup/history/current-device data drifts after an ingest, MQTT, archive, projection, rollup, or replay incident and you need to compare or repair a narrow window without exposing provider IDs, credentials, or other sensitive data.
---

# Pulse Local-to-Cloud Rollup Sync

Use this only for incident recovery when local Pulse rollups are confirmed
authoritative and cloud history/solar charts still diverge after normal replay or
archive-to-rollup rebuilds.

## Guardrails

- Prefer root-cause fixes first: worker startup gates, durable migrations,
  consumer deliver policy, archive replay, and rollup rebuilds.
- Inspect raw archives before repair when device state looks physically
  impossible. Some providers may not expose an explicit offline bit; use
  aggregate evidence such as idle/pause state, sentinel remaining-time values,
  zero load/battery sink, and nonzero input that cannot be consumed.
- Keep the repair window narrow and closed. Do not mirror the current live
  minute; choose an end timestamp at least a few minutes in the past.
- Use canonical UUID device IDs only. Do not print or paste provider device IDs,
  serial numbers, emails, tokens, DSNs, access keys, or physical locations.
- Do not replay broad streams to fix a small rollup gap. Compare device by
  device, then repair only the divergent devices and bounded time window.
- Do not delete cloud rows until local and cloud targets, primary pods, devices,
  and UTC window bounds have been verified.
- After repairing history or projection state, wipe derived energy/inference
  cache keys and restart the serving path when in-process caches may hold stale
  data.

## Fast Archive-Backed Drift Repair

Use this path when the raw archive is authoritative and a device's current or
historical chart is drifting from the physical state. Keep all device IDs and
DSNs in local env files or command substitution; report only aggregate evidence.

1. Inspect the raw archive first with `pulse-raw-archive-inspector`. Confirm the
   affected provider/family, closed UTC window, object/frame counts, payload
   types, and whether an explicit offline/status signal exists.
2. Fix the model before repair. For stale current telemetry, prefer data-layer
   suppression/zeroing that applies to projection, realtime snapshots, BFF
   summaries, and rollups. If rollups flatline, check for aggregator carry
   forward: omitting stale PV is not enough when the previous bucket value can
   persist; write explicit zero current metrics for stale current frames.
3. Rebuild only the affected device/window from archive objects with
   `cmd/ecoflow-rollup-rebuild`. Avoid NATS replay for a closed historical
   repair unless the incident specifically requires stream semantics.
4. Repair current projection from a recent archive tail when only the live
   snapshot is stale. Do not replay 24 hours through NATS just to refresh one
   current snapshot; a short direct archive-to-projection rebuild is faster and
   avoids consumer backlog.
5. Verify with aggregate SQL before opening the app: bucket count, total Wh,
   latest nonzero bucket, post-cutoff Wh, post-cutoff nonzero bucket count, and
   post-cutoff max PV. The post-cutoff values should be zero for a repaired
   offline/flatlined device.
6. Wipe derived caches after repair:

```bash
VALKEY_ADDRS='<valkey-host:port>' go run .tmp/freshness_ops.go --mode wipe-caches
```

If the helper is not available, delete the same low-cardinality cache families:
`pulse:energy-calendar:*`, `pulse:energy-pv-port-history:*`, and
`pulse:inference-energy-comparison:*`. Then restart the BFF/realtime/API serving
path or redeploy locally so in-process caches are cleared.

## Preflight

Set placeholders in the shell. Keep them generic in notes and PR text.

```bash
LOCAL_CONTEXT='<local-kubernetes-context>'
CLOUD_CONTEXT='<cloud-kubernetes-context>'
PLATFORM_NAMESPACE='pulse-platform'
DB_NAME='pulse'
LOCAL_PRIMARY_POD='<local-cnpg-primary-pod>'
CLOUD_PRIMARY_POD='<cloud-cnpg-primary-pod>'
DEVICE_IDS="'<canonical-device-uuid-1>','<canonical-device-uuid-2>'"
WINDOW_START_UTC='<YYYY-MM-DD HH:MM:SS+00>'
WINDOW_END_UTC='<YYYY-MM-DD HH:MM:SS+00>'
```

Check both database clusters are healthy and that the selected pods are current
primaries:

```bash
kubectl --context "$LOCAL_CONTEXT" -n "$PLATFORM_NAMESPACE" \
  get cluster pulse-platform-core \
  -o jsonpath='{.status.phase}{" current="}{.status.currentPrimary}{" ready="}{.status.readyInstances}{"\n"}'

kubectl --context "$CLOUD_CONTEXT" -n "$PLATFORM_NAMESPACE" \
  get cluster pulse-platform-core \
  -o jsonpath='{.status.phase}{" current="}{.status.currentPrimary}{" ready="}{.status.readyInstances}{"\n"}'
```

## Compare First

Compare per-device rollup totals using only canonical UUIDs:

```bash
for DEVICE_ID in <canonical-device-uuid-1> <canonical-device-uuid-2>; do
  for TARGET in local cloud; do
    if [ "$TARGET" = local ]; then
      CTX="$LOCAL_CONTEXT"; POD="$LOCAL_PRIMARY_POD"
    else
      CTX="$CLOUD_CONTEXT"; POD="$CLOUD_PRIMARY_POD"
    fi
    echo "$TARGET $DEVICE_ID"
    kubectl --context "$CTX" -n "$PLATFORM_NAMESPACE" exec "$POD" -- \
      psql -U postgres -d "$DB_NAME" -Atc "
        WITH rows AS (
          SELECT *
          FROM telemetry_rollup_minute
          WHERE device_id = '$DEVICE_ID'
            AND provider = 'ecoflow'
            AND bucket_start >= timestamptz '$WINDOW_START_UTC'
            AND bucket_start < timestamptz '$WINDOW_END_UTC'
        )
        SELECT count(*) || '|' ||
               round(coalesce(sum(solar_generated_wh), 0)::numeric, 3) || '|' ||
               coalesce(to_char(min(bucket_start), 'HH24:MI'), '') || '|' ||
               coalesce(to_char(max(bucket_start), 'HH24:MI'), '')
        FROM rows;" | sed '/^Defaulted/d'
  done
done
```

If cloud has extra closed-window rows, an upsert is not enough. Mirror the
bounded local rollup window by deleting cloud rows for the selected canonical
devices and inserting the local rows in one transaction per table.

## Mirror Bounded Rollups

This mirrors minute/hour/day aggregate rollups and PV-port rollups for the
selected canonical devices and UTC window. It does not copy credentials, provider
secrets, raw MQTT payloads, or provider IDs into logs.

```bash
set -euo pipefail

BASE_COLS='provider,provider_device_id,device_id,bucket_start,sample_count,first_ts_unix_ms,last_ts_unix_ms,soc_avg_pct,soc_min_pct,soc_max_pct,ac_in_avg_w,ac_in_max_w,pv_avg_w,pv_max_w,dc_avg_w,dc_max_w,load_avg_w,load_max_w,net_avg_w,net_min_w,net_max_w,battery_avg_w,battery_min_w,battery_max_w,temp_avg_c,temp_min_c,temp_max_c,solar_generated_wh,created_at,updated_at,ac_input_energy_wh,ac_output_energy_wh,dc_output_energy_wh,load_energy_wh,battery_charge_energy_wh,battery_discharge_energy_wh,ac_output_avg_w,ac_output_max_w'
PV_COLS='provider,provider_device_id,device_id,port_id,port_label,bucket_start,sample_count,first_ts_unix_ms,last_ts_unix_ms,max_observed_volts,max_observed_amps,max_observed_watts,last_observed_volts,last_observed_amps,last_observed_watts,last_observed_at_unix_ms,created_at,updated_at'

mirror_table() {
  table="$1"
  cols="$2"
  echo "mirroring ${table}"

  copy_sql="COPY (
    SELECT ${cols}
    FROM ${table}
    WHERE device_id IN (${DEVICE_IDS})
      AND bucket_start >= timestamptz '${WINDOW_START_UTC}'
      AND bucket_start < timestamptz '${WINDOW_END_UTC}'
    ORDER BY provider, provider_device_id, bucket_start
  ) TO STDOUT WITH (FORMAT csv)"

  apply_sql="CREATE TEMP TABLE tmp_sync (LIKE ${table} INCLUDING DEFAULTS);
    COPY tmp_sync (${cols}) FROM STDIN WITH (FORMAT csv);
    BEGIN;
    DELETE FROM ${table}
    WHERE device_id IN (${DEVICE_IDS})
      AND bucket_start >= timestamptz '${WINDOW_START_UTC}'
      AND bucket_start < timestamptz '${WINDOW_END_UTC}';
    INSERT INTO ${table} (${cols}) SELECT ${cols} FROM tmp_sync;
    COMMIT;
    SELECT '${table} rows=' || count(*) FROM tmp_sync;"

  kubectl --context "$LOCAL_CONTEXT" -n "$PLATFORM_NAMESPACE" \
      exec -c postgres "$LOCAL_PRIMARY_POD" -- \
      psql -U postgres -d "$DB_NAME" -qAtc "$copy_sql" |
    kubectl --context "$CLOUD_CONTEXT" -n "$PLATFORM_NAMESPACE" \
      exec -i -c postgres "$CLOUD_PRIMARY_POD" -- \
      psql -U postgres -d "$DB_NAME" -v ON_ERROR_STOP=1 -qAtc "$apply_sql"
}

mirror_table telemetry_rollup_minute "$BASE_COLS"
mirror_table telemetry_rollup_hour "$BASE_COLS"
mirror_table telemetry_rollup_day "$BASE_COLS"
mirror_table telemetry_rollup_pv_port_minute "$PV_COLS"
mirror_table telemetry_rollup_pv_port_hour "$PV_COLS"
mirror_table telemetry_rollup_pv_port_day "$PV_COLS"
```

## Verify

Re-run the per-device comparison, then compare fleet totals for the same bounded
window:

```bash
for TARGET in local cloud; do
  if [ "$TARGET" = local ]; then
    CTX="$LOCAL_CONTEXT"; POD="$LOCAL_PRIMARY_POD"
  else
    CTX="$CLOUD_CONTEXT"; POD="$CLOUD_PRIMARY_POD"
  fi
  echo "$TARGET fleet"
  kubectl --context "$CTX" -n "$PLATFORM_NAMESPACE" exec "$POD" -- \
    psql -U postgres -d "$DB_NAME" -Atc "
      WITH rows AS (
        SELECT *
        FROM telemetry_rollup_minute
        WHERE provider = 'ecoflow'
          AND bucket_start >= timestamptz '$WINDOW_START_UTC'
          AND bucket_start < timestamptz '$WINDOW_END_UTC'
      )
      SELECT count(*) || '|' ||
             round(coalesce(sum(solar_generated_wh), 0)::numeric, 3) || '|' ||
             coalesce(to_char(min(bucket_start), 'HH24:MI'), '') || '|' ||
             coalesce(to_char(max(bucket_start), 'HH24:MI'), '')
      FROM rows;" | sed '/^Defaulted/d'
done
```

Record only aggregate evidence: row counts, Wh totals, UTC bounds, and whether
per-device and fleet totals match. Keep provider IDs and secrets out of all
messages, PR bodies, and logs.

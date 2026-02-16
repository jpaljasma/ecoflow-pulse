# How-to: Troubleshoot MQTT Connectivity

Use this when MQTT subscription is failing or unstable.

## Symptom: `connect rejected, return code=5`

Typical meaning: broker rejected credentials/session.

Actions:

1. verify API credentials with `go run ./cmd/ecoflow-smoke`,
1. confirm target SN is valid and online,
1. check `logs/mqtt.log` for repeated auth rejection events,
1. let fallback polling continue if configured, while retries continue.

## Symptom: `read mqtt message: EOF`

Actions:

1. confirm reconnect logic is active in logs (`reconnecting`, retry backoff),
1. increase read/write/connect timeouts if network is unstable,
1. avoid overly aggressive idle reconnect
   (`ECOFLOW_MQTT_IDLE_RECONNECT_AFTER=0s` disables idle reconnect).

## Symptom: No telemetry updates

Actions:

1. ensure topic resolves to the expected SN,
1. run smoke test and verify the same device appears online,
1. verify queue is not saturated (`MQTT` row in dashboard),
1. inspect `logs/mqtt.log` for raw payload presence.

## Checklist

- smoke test passes,
- target SN is correct,
- `logs/mqtt.log` shows raw payloads,
- dashboard `Updated` field is advancing,
- no repeated auth rejection loop without recovery.

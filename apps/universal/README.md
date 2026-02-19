# EcoFlow Pulse Universal App (Expo)

A universal telemetry dashboard scaffold for EcoFlow devices: one codebase for web, iOS, and Android.

## Stack
- Expo + Expo Router (universal routing)
- React Native + react-native-web
- Tamagui (design system + theming)
- TanStack Query (REST caching)
- Zustand (UI snapshot state)
- Zod (runtime payload validation)

## Setup
1. From repo root:
   - `npm install`
2. Configure env (shell or `.env`):
   - `EXPO_PUBLIC_API_URL=http://localhost:8080`
   - `EXPO_PUBLIC_WS_URL=ws://localhost:8080/ws`
   - For local mock REST without backend: `EXPO_PUBLIC_API_URL=mock://ecoflow`
3. Run:
   - Web: `npm run web`
   - iOS: `npm run ios`
   - Android: `npm run android`

## Mock Data Sources
In `mock://ecoflow` mode, data is sourced in this priority:
1. `telemetry_training.csv` (preferred, SN-scoped normalized telemetry)
2. `mqtt.log` (fallback only)

Default web paths:
- `/mock/telemetry_training.csv`
- `/mock/mqtt.log`

You can override with:
- `EXPO_PUBLIC_MOCK_TRAINING_URL`
- `EXPO_PUBLIC_MOCK_LOG_URL`

### SOC Rule (DELTA 2 Max)
For D2M, card SOC prefers `bp1_soc` from training CSV when available.
Reason: `soc_pct` may be weighted across packs and can diverge from the user-facing main-unit SOC.

## Telemetry Engine (ingest vs snapshot)
Telemetry rendering is decoupled from message ingest:
- WS ingest can run at high frequency (e.g. 10Hz/device, 16 devices).
- Incoming messages are validated (Zod) and written into internal per-device ring buffers.
- UI updates are emitted on a fixed snapshot clock (`200ms` default / `5fps`), not per message.
- Zustand stores **snapshots only** (`snapshotByDeviceId`, `lastUpdatedAt`, visible IDs).
- A device is marked stale when no update arrives for `>5s`.

This keeps rendering stable and avoids React churn under high throughput.

## Devices UI Behavior
- Fleet summary includes:
  - unique device-type thumbnail strip,
  - aggregate battery/SOC/AC/DC/PV/Load/Net stats,
  - load + PV trend blocks.
- Device cards include per-card freshness:
  - card becomes **inactive** after `>60s` without new data,
  - inactive cards fade to muted gray and show `(inactive)` next to title,
  - cards fade back to active immediately when telemetry resumes.
- Top-right list status indicator uses a subtle pulse animation.

## Structure
- `app/` Expo Router screens
- `src/features/devices` REST API + list/detail card logic
- `src/features/telemetry` engine, ring buffer, Zustand store, hooks
- `src/shared/ui` Tamagui components and theme

## Built-in Mock Devices
When `EXPO_PUBLIC_API_URL=mock://ecoflow`, REST routes are served in-app:
- `GET /api/devices`
- `GET /api/devices/:id`

Mock payload includes two devices from recent MQTT telemetry context:
- `DPU A 12 kWh` (`Y711ZABA9H2P0294`)
- `Kitchen Delta 2 Max` (`R351ZABAPH331057`)

Each device includes:
- `serialNumber`
- `batteryPct`
- `state`
- `etaMinutes`

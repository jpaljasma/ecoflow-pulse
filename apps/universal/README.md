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

## Telemetry Engine (ingest vs snapshot)
Telemetry rendering is decoupled from message ingest:
- WS ingest can run at high frequency (e.g. 10Hz/device, 16 devices).
- Incoming messages are validated (Zod) and written into internal per-device ring buffers.
- UI updates are emitted on a fixed snapshot clock (`200ms` default / `5fps`), not per message.
- Zustand stores **snapshots only** (`snapshotByDeviceId`, `lastUpdatedAt`, visible IDs).
- A device is marked stale when no update arrives for `>3s`.

This keeps rendering stable and avoids React churn under high throughput.

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

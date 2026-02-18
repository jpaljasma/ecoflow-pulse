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

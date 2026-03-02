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
2. Start the local cluster-served stack:
   - `make platform-up`
   - `make platform-wait`
   - `make services-up`
   - `make services-wait`
   - `make db-seed-dev-local`
3. Use the public edge:
   - app: `http://localhost/devices`
   - REST: `http://localhost/api/...`
   - websocket: `ws://localhost/ws`
4. Standalone Expo debugging is optional:
   - Web: `npm run web`
   - iOS: `npm run ios`
   - Android: `npm run android`
   - only set `EXPO_PUBLIC_API_URL` / `EXPO_PUBLIC_WS_URL` when intentionally bypassing the cluster public edge

## Telemetry Engine (ingest vs snapshot)
Telemetry rendering is decoupled from message ingest:
- WS ingest can run at high frequency (e.g. 10Hz/device, 16 devices).
- Incoming messages are validated (Zod) and written into internal per-device ring buffers.
- UI updates are emitted on a fixed snapshot clock (`200ms` default / `5fps`), not per message.
- Zustand stores **snapshots only** (`snapshotByDeviceId`, `lastUpdatedAt`, visible IDs).
- A device is marked stale when no update arrives for `>5s`.

This keeps rendering stable and avoids React churn under high throughput.

## Auth + reconnect behavior
- when OIDC is configured, REST queries and websocket connect wait for persisted auth session hydration.
- if auth is required and no valid access token exists, the telemetry engine stays in `auth_required` and the devices screen shows a sign-in-required state.
- websocket lifecycle is owned by the provider, so token refresh/reconnect does not clear active device subscriptions.

## Devices UI Behavior
- Fleet summary includes:
  - unique device-type thumbnail strip,
  - aggregate battery/SOC/AC/DC/PV/Load/Net stats,
  - load + PV trend blocks (premium chart renderer only).
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

## Routing
- `/devices` uses canonical device UUIDs from the real Node BFF.
- `/device/<uuid>` is the canonical detail route.
- `/device/<serial-number>` is accepted as a compatibility alias and is immediately resolved to the canonical UUID route before history/realtime fetches begin.

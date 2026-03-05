import type { Page, Route } from '@playwright/test';

export const DPU_DEVICE_ID = '019cab9d-bcb3-7587-8dc9-9a57deb48d30';
export const D2M_DEVICE_ID = '019cab9d-bcab-75c0-9c02-db3ae1105d61';
export const DPU_SERIAL = 'Y711ZABA9H2P0294';
export const D2M_SERIAL = 'R351ZABAPH331057';

type DevicePayload = {
  id: string;
  serialNumber: string;
  name: string;
  model: string;
  online: boolean;
  batteryPct: number;
  state: 'charging' | 'discharging' | 'idle';
  etaMinutes: number;
  pvW: number;
  acInW: number;
  dcW: number;
  loadW: number;
  netW: number;
  tempC: number;
  telemetryTsMs: number;
  capabilities?: Record<string, unknown>;
  details?: Record<string, unknown>;
};

const NOW_UNIX_MS = Date.UTC(2026, 2, 4, 15, 20, 0);

const DEVICES: DevicePayload[] = [
  {
    id: DPU_DEVICE_ID,
    serialNumber: DPU_SERIAL,
    name: 'DPU A 12 kWh',
    model: 'DELTA Pro Ultra',
    online: true,
    batteryPct: 24.6,
    state: 'charging',
    etaMinutes: 263,
    pvW: 356,
    acInW: 0,
    dcW: 0,
    loadW: 118,
    netW: 238,
    tempC: 24.1,
    telemetryTsMs: NOW_UNIX_MS,
    capabilities: {
      batteryPacks: 3,
      evCharging: true
    },
    details: {
      bpCount: 3,
      packs: [
        {
          id: 'bp1',
          socPct: 24.1,
          powerW: 55.2,
          tempC: 23.4,
          heatingOn: false,
          energyWh: 6104
        },
        {
          id: 'bp2',
          socPct: 25.0,
          powerW: 48.1,
          tempC: 23.7,
          heatingOn: false,
          energyWh: 6040
        }
      ],
      solarPorts: [
        {
          id: 'pv-low',
          name: 'PV Low',
          state: 'charging',
          volts: 66.2,
          amps: 2.34,
          watts: 155,
          maxVolts: 150,
          maxAmps: 15,
          maxWatts: 1600
        },
        {
          id: 'pv-high',
          name: 'PV High',
          state: 'charging',
          volts: 322.1,
          amps: 0.63,
          watts: 201,
          maxVolts: 450,
          maxAmps: 15,
          maxWatts: 4000
        }
      ],
      acOn: true,
      dcOn: false,
      usbOn: false,
      dc12vOn: false,
      fanOn: false,
      solarChargingOn: true
    }
  },
  {
    id: D2M_DEVICE_ID,
    serialNumber: D2M_SERIAL,
    name: 'Kitchen Delta 2 Max',
    model: 'DELTA 2 Max',
    online: true,
    batteryPct: 31.8,
    state: 'discharging',
    etaMinutes: 419,
    pvW: 0,
    acInW: 132,
    dcW: 0,
    loadW: 143,
    netW: -11,
    tempC: 20.2,
    telemetryTsMs: NOW_UNIX_MS,
    capabilities: {
      batteryPacks: 2
    },
    details: {
      bpCount: 2,
      packs: [
        {
          id: 'bp-main',
          socPct: 31.2,
          powerW: -79.0,
          tempC: 21.0,
          heatingOn: false,
          energyWh: 2102
        },
        {
          id: 'bp-extra-1',
          socPct: 32.3,
          powerW: -64.0,
          tempC: 20.6,
          heatingOn: false,
          energyWh: 2010
        }
      ],
      solarPorts: [
        {
          id: 'pv1',
          name: 'PV 1',
          state: 'idle',
          volts: 24.1,
          amps: 0,
          watts: 0,
          maxVolts: 60,
          maxAmps: 15,
          maxWatts: 500
        },
        {
          id: 'pv2',
          name: 'PV 2',
          state: 'idle',
          volts: 18.5,
          amps: 0,
          watts: 0,
          maxVolts: 60,
          maxAmps: 15,
          maxWatts: 500
        }
      ],
      acOn: true,
      dcOn: false,
      usbOn: false,
      dc12vOn: false,
      fanOn: false,
      solarChargingOn: false
    }
  }
];

const DEVICE_BY_KEY = new Map<string, DevicePayload>(
  DEVICES.flatMap((device) => [
    [device.id, device],
    [device.serialNumber, device]
  ])
);

function toUnixMsString(value: number): string {
  return String(Math.trunc(value));
}

function buildRollupPoint(bucketStartMs: number, durationMinutes: number, pvAvgW: number) {
  const bucketEndMs = bucketStartMs + durationMinutes * 60_000;
  const solarGeneratedWh = (pvAvgW * durationMinutes) / 60;

  return {
    bucketStartUnixMs: toUnixMsString(bucketStartMs),
    bucketEndUnixMs: toUnixMsString(bucketEndMs),
    sampleCount: 5,
    firstTsUnixMs: toUnixMsString(bucketStartMs),
    lastTsUnixMs: toUnixMsString(bucketEndMs - 1000),
    metrics: {
      socAvgPct: 30,
      socMinPct: 29,
      socMaxPct: 31,
      acInAvgW: 0,
      acInMaxW: 0,
      pvAvgW,
      pvMaxW: pvAvgW,
      dcAvgW: 0,
      dcMaxW: 0,
      loadAvgW: 120,
      loadMaxW: 140,
      netAvgW: pvAvgW - 120,
      netMinW: -140,
      netMaxW: pvAvgW,
      batteryAvgW: -80,
      batteryMinW: -100,
      batteryMaxW: 40,
      tempAvgC: 21,
      tempMinC: 20,
      tempMaxC: 22,
      solarGeneratedWh
    }
  };
}

function buildCompareHistory(deviceId: string) {
  const isDpu = deviceId === DPU_DEVICE_ID;
  const currentBase = Date.UTC(2026, 2, 4, 6, 0, 0);
  const previousBase = Date.UTC(2026, 2, 3, 6, 0, 0);
  const currentWatts = isDpu ? [0, 55, 110, 170, 130, 65] : [0, 20, 45, 75, 40, 10];
  const previousWatts = isDpu ? [0, 35, 82, 110, 88, 44] : [0, 15, 35, 50, 25, 8];

  const currentPoints = currentWatts.map((pvAvgW, index) =>
    buildRollupPoint(currentBase + index * 10 * 60_000, 10, pvAvgW)
  );
  const previousPoints = previousWatts.map((pvAvgW, index) =>
    buildRollupPoint(previousBase + index * 10 * 60_000, 10, pvAvgW)
  );

  return {
    current: {
      deviceId,
      resolution: 'minute',
      fromUnixMs: toUnixMsString(currentBase),
      toUnixMs: toUnixMsString(currentBase + 6 * 10 * 60_000),
      points: currentPoints
    },
    previous: {
      deviceId,
      resolution: 'minute',
      fromUnixMs: toUnixMsString(previousBase),
      toUnixMs: toUnixMsString(previousBase + 6 * 10 * 60_000),
      points: previousPoints
    }
  };
}

async function fulfillJson(route: Route, payload: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(payload)
  });
}

export async function mockApiRoutes(page: Page): Promise<void> {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const { pathname } = url;

    if (pathname === '/api/devices') {
      await fulfillJson(route, { devices: DEVICES });
      return;
    }

    if (pathname.startsWith('/api/devices/')) {
      const key = decodeURIComponent(pathname.replace('/api/devices/', ''));
      const device = DEVICE_BY_KEY.get(key);
      if (!device) {
        await fulfillJson(route, { error: 'not_found' }, 404);
        return;
      }
      await fulfillJson(route, device);
      return;
    }

    if (pathname.startsWith('/api/v1/devices/') && pathname.endsWith('/history/compare')) {
      const deviceId = decodeURIComponent(
        pathname.replace('/api/v1/devices/', '').replace('/history/compare', '')
      );
      if (!DEVICE_BY_KEY.has(deviceId)) {
        await fulfillJson(route, { error: 'not_found' }, 404);
        return;
      }
      await fulfillJson(route, buildCompareHistory(deviceId));
      return;
    }

    await fulfillJson(route, { error: `unhandled_mock_path:${pathname}` }, 404);
  });
}

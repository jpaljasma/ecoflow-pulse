import type { Page, Route } from '@playwright/test';

export const DPU_DEVICE_ID = '11111111-1111-7111-8111-111111111111';
export const D2M_DEVICE_ID = '22222222-2222-7222-8222-222222222222';
export const DPU_SERIAL = 'DEMODPU0000294';
export const D2M_SERIAL = 'DEMOD2M00001057';

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

function buildEnergyRollupPoint(bucketStartMs: number, durationMinutes: number, values: {
  solarGeneratedWh: number;
  acInputEnergyWh: number;
  loadEnergyWh: number;
  pvAvgW: number;
  acInAvgW: number;
  loadAvgW: number;
}): Record<string, unknown> {
  const bucketEndMs = bucketStartMs + durationMinutes * 60_000;
  return {
    bucketStartUnixMs: toUnixMsString(bucketStartMs),
    bucketEndUnixMs: toUnixMsString(bucketEndMs),
    sampleCount: 1,
    firstTsUnixMs: toUnixMsString(bucketStartMs),
    lastTsUnixMs: toUnixMsString(bucketEndMs - 1000),
    metrics: {
      socAvgPct: 42,
      socMinPct: 39,
      socMaxPct: 46,
      acInAvgW: values.acInAvgW,
      acInMaxW: values.acInAvgW + 18,
      acOutputAvgW: Math.max(values.loadAvgW - 18, 0),
      acOutputMaxW: Math.max(values.loadAvgW + 4, 0),
      pvAvgW: values.pvAvgW,
      pvMaxW: values.pvAvgW + 30,
      dcAvgW: 18,
      dcMaxW: 24,
      loadAvgW: values.loadAvgW,
      loadMaxW: values.loadAvgW + 22,
      netAvgW: values.pvAvgW - values.loadAvgW,
      netMinW: -120,
      netMaxW: 160,
      batteryAvgW: -36,
      batteryMinW: -82,
      batteryMaxW: 12,
      tempAvgC: 23,
      tempMinC: 22,
      tempMaxC: 24,
      solarGeneratedWh: values.solarGeneratedWh,
      acInputEnergyWh: values.acInputEnergyWh,
      acOutputEnergyWh: Math.max(values.loadEnergyWh - 18, 0),
      dcOutputEnergyWh: 18,
      loadEnergyWh: values.loadEnergyWh,
      batteryChargeEnergyWh: 0,
      batteryDischargeEnergyWh: 36
    }
  };
}

function clonePoint<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function buildEnergyDashboard({
  includeComparison = true,
  scope = 'all',
  deviceId,
  preset = 'today',
  timezone = 'UTC'
}: {
  includeComparison?: boolean;
  scope?: 'all' | 'device';
  deviceId?: string;
  preset?: string;
  timezone?: string;
}) {
  const currentBase = Date.UTC(2026, 2, 4, 0, 0, 0);
  const previousBase = Date.UTC(2026, 2, 3, 0, 0, 0);
  const currentEnergyPoints = [
    buildEnergyRollupPoint(currentBase, 60, {
      solarGeneratedWh: 280,
      acInputEnergyWh: 60,
      loadEnergyWh: 240,
      pvAvgW: 280,
      acInAvgW: 60,
      loadAvgW: 240
    }),
    buildEnergyRollupPoint(currentBase + 60 * 60_000, 60, {
      solarGeneratedWh: 520,
      acInputEnergyWh: 40,
      loadEnergyWh: 310,
      pvAvgW: 520,
      acInAvgW: 40,
      loadAvgW: 310
    }),
    buildEnergyRollupPoint(currentBase + 2 * 60 * 60_000, 60, {
      solarGeneratedWh: 460,
      acInputEnergyWh: 30,
      loadEnergyWh: 280,
      pvAvgW: 460,
      acInAvgW: 30,
      loadAvgW: 280
    })
  ];
  const previousEnergyPoints = [
    buildEnergyRollupPoint(previousBase, 60, {
      solarGeneratedWh: 140,
      acInputEnergyWh: 90,
      loadEnergyWh: 230,
      pvAvgW: 140,
      acInAvgW: 90,
      loadAvgW: 230
    }),
    buildEnergyRollupPoint(previousBase + 60 * 60_000, 60, {
      solarGeneratedWh: 260,
      acInputEnergyWh: 80,
      loadEnergyWh: 260,
      pvAvgW: 260,
      acInAvgW: 80,
      loadAvgW: 260
    })
  ];

  const resolvedDeviceIds =
    scope === 'device' && deviceId && DEVICE_BY_KEY.has(deviceId)
      ? [deviceId]
      : [DPU_DEVICE_ID, D2M_DEVICE_ID];
  const selectedDevice = resolvedDeviceIds[0];
  const scopedCurrentEnergyPoints =
    scope === 'device' && selectedDevice === D2M_DEVICE_ID
      ? currentEnergyPoints.slice(0, 2)
      : currentEnergyPoints;
  const scopedPreviousEnergyPoints =
    scope === 'device' && selectedDevice === D2M_DEVICE_ID
      ? previousEnergyPoints.slice(0, 1)
      : previousEnergyPoints;
  const scopedPVHistory =
    scope === 'device'
      ? [
          {
            deviceId: selectedDevice,
            portId: selectedDevice === D2M_DEVICE_ID ? 'pv1' : 'pv-low',
            portLabel: selectedDevice === D2M_DEVICE_ID ? 'PV 1' : 'PV Low',
            maxObservedVolts: selectedDevice === D2M_DEVICE_ID ? 29.5 : 72.5,
            maxObservedAmps: selectedDevice === D2M_DEVICE_ID ? 2.1 : 1.15,
            maxObservedWatts: selectedDevice === D2M_DEVICE_ID ? 61.9 : 83.3,
            lastObservedVolts: selectedDevice === D2M_DEVICE_ID ? 24.1 : 66.3,
            lastObservedAmps: selectedDevice === D2M_DEVICE_ID ? 0.0 : 0.79,
            lastObservedWatts: selectedDevice === D2M_DEVICE_ID ? 0.0 : 52.4,
            lastObservedUnixMs: toUnixMsString(currentBase + 2 * 60 * 60_000),
            sampleCount: selectedDevice === D2M_DEVICE_ID ? 2 : 3
          }
        ]
      : [
          {
            deviceId: DPU_DEVICE_ID,
            portId: 'pv-low',
            portLabel: 'PV Low',
            maxObservedVolts: 72.5,
            maxObservedAmps: 1.15,
            maxObservedWatts: 83.3,
            lastObservedVolts: 66.3,
            lastObservedAmps: 0.79,
            lastObservedWatts: 52.4,
            lastObservedUnixMs: toUnixMsString(currentBase + 2 * 60 * 60_000),
            sampleCount: 3
          },
          {
            deviceId: DPU_DEVICE_ID,
            portId: 'pv-high',
            portLabel: 'PV High',
            maxObservedVolts: 335.9,
            maxObservedAmps: 0.75,
            maxObservedWatts: 252.0,
            lastObservedVolts: 322.1,
            lastObservedAmps: 0.63,
            lastObservedWatts: 201.0,
            lastObservedUnixMs: toUnixMsString(currentBase + 2 * 60 * 60_000),
            sampleCount: 3
          }
        ];

  const scopedSummary =
    scope === 'device' && selectedDevice === D2M_DEVICE_ID
      ? {
          solarGeneratedKwh: { current: 0.26, previous: includeComparison ? 0.12 : 0, delta: includeComparison ? 0.14 : 0.26, deltaPct: includeComparison ? 116.7 : null },
          loadConsumedKwh: { current: 0.51, previous: includeComparison ? 0.44 : 0, delta: includeComparison ? 0.07 : 0.51, deltaPct: includeComparison ? 15.9 : null },
          selfSufficiencyPct: { current: 22.0, previous: includeComparison ? 18.5 : 0, delta: includeComparison ? 3.5 : 22.0, deltaPct: includeComparison ? 18.9 : null },
          batteryNetKwh: { current: -0.08, previous: includeComparison ? -0.04 : 0, delta: includeComparison ? -0.04 : -0.08, deltaPct: includeComparison ? -100 : null },
          estimatedValue: { current: 0.05, previous: includeComparison ? 0.02 : 0, delta: includeComparison ? 0.03 : 0.05, deltaPct: includeComparison ? 150 : null },
          estimatedAcInputCost: { current: 0.08, previous: includeComparison ? 0.07 : 0, delta: includeComparison ? 0.01 : 0.08, deltaPct: includeComparison ? 14.3 : null },
          currency: 'USD'
        }
      : {
          solarGeneratedKwh: { current: 1.26, previous: includeComparison ? 0.4 : 0, delta: includeComparison ? 0.86 : 1.26, deltaPct: includeComparison ? 215 : null },
          loadConsumedKwh: { current: 0.83, previous: includeComparison ? 0.49 : 0, delta: includeComparison ? 0.34 : 0.83, deltaPct: includeComparison ? 69.4 : null },
          selfSufficiencyPct: { current: 84.5, previous: includeComparison ? 61.3 : 0, delta: includeComparison ? 23.2 : 84.5, deltaPct: includeComparison ? 37.8 : null },
          batteryNetKwh: { current: -0.11, previous: includeComparison ? -0.05 : 0, delta: includeComparison ? -0.06 : -0.11, deltaPct: includeComparison ? -120 : null },
          estimatedValue: { current: 0.24, previous: includeComparison ? 0.09 : 0, delta: includeComparison ? 0.15 : 0.24, deltaPct: includeComparison ? 166.7 : null },
          estimatedAcInputCost: { current: 0.04, previous: includeComparison ? 0.05 : 0, delta: includeComparison ? -0.01 : 0.04, deltaPct: includeComparison ? -20 : null },
          currency: 'USD'
        };

  return {
    scope: {
      mode: scope,
      deviceId: scope === 'device' ? selectedDevice : '',
      resolvedDeviceIds
    },
    window: {
      preset,
      timezone,
      fromUnixMs: toUnixMsString(currentBase),
      toUnixMs: toUnixMsString(currentBase + 3 * 60 * 60_000),
      previousFromUnixMs: toUnixMsString(previousBase),
      previousToUnixMs: toUnixMsString(previousBase + 3 * 60 * 60_000)
    },
    summary: scopedSummary,
    battery: {
      chargeKwh: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 0.02 : 0,
      dischargeKwh: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 0.1 : 0.11,
      netKwh: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? -0.08 : -0.11,
      socStartPct: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 34 : 44,
      socEndPct: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 29 : 39,
      socMinPct: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 29 : 39,
      socMaxPct: scope === 'device' && selectedDevice === D2M_DEVICE_ID ? 35 : 46
    },
    currentEnergyPoints: clonePoint(scopedCurrentEnergyPoints),
    previousEnergyPoints: includeComparison ? clonePoint(scopedPreviousEnergyPoints) : [],
    currentPowerPoints: clonePoint(scopedCurrentEnergyPoints),
    previousPowerPoints: includeComparison ? clonePoint(scopedPreviousEnergyPoints) : [],
    pvPortHistory: scopedPVHistory
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

    if (pathname === '/api/v1/energy/dashboard') {
      const scopeParam = url.searchParams.get('scope');
      const scope = scopeParam === 'device' ? 'device' : 'all';
      await fulfillJson(
        route,
        buildEnergyDashboard({
          includeComparison: url.searchParams.get('includeComparison') !== 'false',
          scope,
          deviceId: url.searchParams.get('deviceId') ?? undefined,
          preset: url.searchParams.get('preset') ?? 'today',
          timezone: url.searchParams.get('timezone') ?? 'UTC'
        })
      );
      return;
    }

    await fulfillJson(route, { error: `unhandled_mock_path:${pathname}` }, 404);
  });
}

import type { Page, Route } from '@playwright/test';

export const DPU_DEVICE_ID = '11111111-1111-7111-8111-111111111111';
export const D2M_DEVICE_ID = '22222222-2222-7222-8222-222222222222';
export const PECRON_DEVICE_ID = '33333333-3333-7333-8333-333333333333';
export const DPU_SERIAL = 'DEMODPU0000294';
export const D2M_SERIAL = 'DEMOD2M00001057';
export const PECRON_SERIAL = 'P11VXG:DEMO-001';
const PROFILE_WEATHER_LOCATION = {
  label: 'Naples, NY',
  latitude: 42.6159,
  longitude: -77.4014
};
const PROFILE_TIMEZONE = 'America/New_York';
const WEATHER_ISSUED_AT_UNIX_MS = Date.UTC(2026, 2, 4, 15, 0, 0);

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

type WeatherMetricValuePayload = {
  raw?: number | null;
  corrected?: number | null;
  unit?: string;
};

type WeatherPointPayload = {
  timestampIso: string;
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: string;
  temperature2m?: WeatherMetricValuePayload;
  windSpeed10m?: WeatherMetricValuePayload;
  windDirection10mDegrees?: number | null;
  windDirectionErrorDegrees?: number | null;
  precipitation?: WeatherMetricValuePayload;
  cloudCover?: WeatherMetricValuePayload;
  visibility?: WeatherMetricValuePayload;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiation?: WeatherMetricValuePayload;
  uvIndex?: WeatherMetricValuePayload;
  globalTiltedIrradiance?: WeatherMetricValuePayload;
};

type WeatherDailyPointPayload = {
  dateIso: string;
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: string;
  sunriseIso?: string | null;
  sunsetIso?: string | null;
  daylightDurationSeconds?: number | null;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiationSum?: WeatherMetricValuePayload;
  uvIndexMax?: WeatherMetricValuePayload;
};

type WeatherForecastPayload = {
  issuedAtUnixMs: string;
  timezone: string;
  unitSystem: 'metric';
  panelTiltDegrees: number;
  panelAzimuthDegrees: number;
  provenance: {
    source: 'open_meteo';
    modelSelection: 'best_match';
    actualSource: 'past_days';
  };
  current: WeatherPointPayload;
  hourly: WeatherPointPayload[];
  daily: WeatherDailyPointPayload[];
};

type WeatherYesterdayVerificationPayload = {
  issuedAtUnixMs: string;
  timezone: string;
  verificationSource: 'snapshot';
  provenance: {
    source: 'open_meteo';
    modelSelection: 'best_match';
    actualSource: 'past_days';
    verificationSource: 'snapshot';
  };
  summary: {
    comparedHours: number;
    matchedHours: number;
    meanAbsoluteTemperatureError: number;
    meanAbsoluteWindSpeedError: number;
    meanAbsoluteCloudCoverError: number;
    meanAbsoluteVisibilityError: number;
    meanAbsoluteUvIndexError: number;
    meanAbsoluteRadiationError: number;
  };
  hours: Array<{
    timestampIso: string;
    forecast: WeatherPointPayload;
    actual: WeatherPointPayload;
    error: {
      temperature2m: number;
      windSpeed10m: number;
      cloudCover: number;
      visibility: number;
      uvIndex: number;
      shortwaveRadiation: number;
      windDirection: number;
    };
  }>;
};

type CurrentUserPayload = {
  user: Record<string, unknown>;
  authorization: {
    roles: string[];
    deviceCount: number;
  };
};

const NOW_UNIX_MS = Date.UTC(2026, 2, 4, 15, 20, 0);
const CURRENT_USER_BOOTSTRAP: CurrentUserPayload = {
  user: {
    id: '019d2b2c-98cd-7f33-b39d-5c8b7fd4c111',
    email: 'user@example.com',
    emailVerified: true,
    displayName: 'Pulse User',
    avatarUrl: 'https://example.com/avatar.png',
    authMethod: 'google',
    givenName: 'Pulse',
    familyName: 'User',
    locale: 'en-US',
    timezone: PROFILE_TIMEZONE,
    weatherLocationEnabled: true,
    weatherLocation: PROFILE_WEATHER_LOCATION
  },
  authorization: {
    roles: ['viewer'],
    deviceCount: 3
  }
};
let currentUserBootstrap: CurrentUserPayload = JSON.parse(JSON.stringify(CURRENT_USER_BOOTSTRAP)) as CurrentUserPayload;

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
      solarChargingOn: true,
      stormGuardActive: true,
      stormGuardEndsAtUnixMs: Date.now() + 2 * 60 * 60 * 1000
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

function buildWeatherMetric(raw: number, corrected: number, unit: string): WeatherMetricValuePayload {
  return { raw, corrected, unit };
}

function buildWeatherPoint(
  timestampIso: string,
  weatherCode: number,
  temperature: { raw: number; corrected: number },
  windSpeed: { raw: number; corrected: number },
  cloudCover: { raw: number; corrected: number },
  visibility: { raw: number; corrected: number },
  uvIndex: { raw: number; corrected: number },
  shortwaveRadiation: { raw: number; corrected: number },
  windDirectionDegrees: number,
  sunshineDurationSeconds: number,
  precipitation: { raw: number; corrected: number },
  globalTiltedIrradiance?: { raw: number; corrected: number }
): WeatherPointPayload {
  return {
    timestampIso,
    weatherCode,
    weatherLabel: undefined,
    weatherIcon: undefined,
    temperature2m: buildWeatherMetric(temperature.raw, temperature.corrected, 'celsius'),
    windSpeed10m: buildWeatherMetric(windSpeed.raw, windSpeed.corrected, 'm/s'),
    windDirection10mDegrees: windDirectionDegrees,
    precipitation: buildWeatherMetric(precipitation.raw, precipitation.corrected, 'mm'),
    cloudCover: buildWeatherMetric(cloudCover.raw, cloudCover.corrected, 'percent'),
    visibility: buildWeatherMetric(visibility.raw, visibility.corrected, 'm'),
    sunshineDurationSeconds,
    shortwaveRadiation: buildWeatherMetric(shortwaveRadiation.raw, shortwaveRadiation.corrected, 'w/m2'),
    uvIndex: buildWeatherMetric(uvIndex.raw, uvIndex.corrected, 'index'),
    ...(globalTiltedIrradiance
      ? {
          globalTiltedIrradiance: buildWeatherMetric(globalTiltedIrradiance.raw, globalTiltedIrradiance.corrected, 'w/m2')
        }
      : {})
  };
}

function buildWeatherForecast(): WeatherForecastPayload {
  const hourly = Array.from({ length: 24 }, (_unused, index) => {
    const timestamp = new Date(Date.UTC(2026, 2, 4, 0, 0, 0) + index * 60 * 60_000).toISOString();
    const baseTemp = 9.2 + index * 0.35;
    const baseWind = 3.6 + (index % 5) * 0.4;
    const baseCloud = 28 + (index % 6) * 6;
    const baseVisibility = 12000 + index * 220;
    const baseUv = Math.max(0, 2.2 + index * 0.08);
    const baseRad = 120 + index * 11;
    const basePrecip = index % 6 === 0 ? 0.6 : 0;
    return buildWeatherPoint(
      timestamp,
      index < 8 ? 63 : index < 16 ? 61 : 2,
      { raw: baseTemp, corrected: baseTemp + 0.4 },
      { raw: baseWind, corrected: baseWind - 0.2 },
      { raw: baseCloud, corrected: Math.min(100, baseCloud + 4) },
      { raw: baseVisibility, corrected: baseVisibility + 350 },
      { raw: baseUv, corrected: baseUv + 0.1 },
      { raw: baseRad, corrected: baseRad + 15 },
      210 + index * 4,
      1200 + index * 15,
      { raw: basePrecip, corrected: basePrecip }
    );
  });
  hourly[0].weatherLabel = 'Rain';
  hourly[0].weatherIcon = 'weather-rainy';

  const dayCodes = [63, 2, 0, 61, 80, 95, 45];
  const daily = dayCodes.map((code, index) => {
    const date = new Date(Date.UTC(2026, 2, 4 + index)).toISOString().slice(0, 10);
    return {
      dateIso: date,
      weatherCode: code,
      weatherLabel:
        code === 63
          ? 'Rain'
          : code === 2
            ? 'Partly cloudy'
            : code === 0
              ? 'Clear sky'
              : code === 61
                ? 'Rain'
                : code === 80
                  ? 'Rain showers'
                  : code === 95
                    ? 'Thunderstorm'
                    : 'Fog',
      weatherIcon:
        code === 0
          ? 'weather-sunny'
          : code === 2
            ? 'weather-partly-cloudy'
            : code === 95
              ? 'weather-lightning-rainy'
              : code === 45
                ? 'weather-fog'
                : 'weather-rainy',
      sunriseIso: new Date(Date.UTC(2026, 2, 4 + index, 10, 58, 0)).toISOString(),
      sunsetIso: new Date(Date.UTC(2026, 2, 4 + index, 22, 6, 0)).toISOString(),
      daylightDurationSeconds: 37_800 + index * 90,
      sunshineDurationSeconds: 12_600 + index * 300,
      shortwaveRadiationSum: buildWeatherMetric(420 + index * 18, 438 + index * 18, 'w/m2'),
      uvIndexMax: buildWeatherMetric(4.4 + index * 0.1, 4.6 + index * 0.1, 'index')
    } satisfies WeatherDailyPointPayload;
  });

  return {
    issuedAtUnixMs: String(WEATHER_ISSUED_AT_UNIX_MS),
    timezone: PROFILE_TIMEZONE,
    unitSystem: 'metric',
    panelTiltDegrees: 45,
    panelAzimuthDegrees: 0,
    provenance: {
      source: 'open_meteo',
      modelSelection: 'best_match',
      actualSource: 'past_days'
    },
    current: {
      ...hourly[10],
      timestampIso: new Date(Date.UTC(2026, 2, 4, 15, 0, 0)).toISOString(),
      weatherCode: 63,
      weatherLabel: 'Rain',
      weatherIcon: 'weather-rainy',
      temperature2m: buildWeatherMetric(12.4, 12.9, 'celsius'),
      windSpeed10m: buildWeatherMetric(4.8, 4.5, 'm/s'),
      cloudCover: buildWeatherMetric(82, 85, 'percent'),
      visibility: buildWeatherMetric(11800, 12200, 'm'),
      uvIndex: buildWeatherMetric(0.3, 0.3, 'index'),
      shortwaveRadiation: buildWeatherMetric(130, 145, 'w/m2'),
      precipitation: buildWeatherMetric(0.2, 0.2, 'mm')
    },
    hourly,
    daily
  };
}

function buildYesterdayVerification(): WeatherYesterdayVerificationPayload {
  const hours = Array.from({ length: 24 }, (_unused, index) => {
    const timestamp = new Date(Date.UTC(2026, 2, 3, index, 0, 0)).toISOString();
    const forecast = buildWeatherPoint(
      timestamp,
      index < 9 ? 61 : index < 17 ? 63 : 2,
      { raw: 10.4 + index * 0.28, corrected: 10.8 + index * 0.28 },
      { raw: 4.2 + (index % 4) * 0.3, corrected: 4.0 + (index % 4) * 0.3 },
      { raw: 42 + (index % 5) * 7, corrected: 45 + (index % 5) * 7 },
      { raw: 10200 + index * 120, corrected: 10500 + index * 120 },
      { raw: 1.9 + index * 0.04, corrected: 2.0 + index * 0.04 },
      { raw: 110 + index * 8, corrected: 124 + index * 8 },
      190 + index * 5,
      800 + index * 10,
      { raw: index % 7 === 0 ? 0.4 : 0, corrected: index % 7 === 0 ? 0.3 : 0 }
    );
    const actual = buildWeatherPoint(
      timestamp,
      index < 8 ? 61 : index < 17 ? 63 : 2,
      { raw: 10.1 + index * 0.27, corrected: 10.1 + index * 0.27 },
      { raw: 4.0 + (index % 4) * 0.35, corrected: 4.0 + (index % 4) * 0.35 },
      { raw: 40 + (index % 5) * 7, corrected: 40 + (index % 5) * 7 },
      { raw: 10100 + index * 110, corrected: 10100 + index * 110 },
      { raw: 1.8 + index * 0.03, corrected: 1.8 + index * 0.03 },
      { raw: 105 + index * 9, corrected: 105 + index * 9 },
      188 + index * 5,
      780 + index * 12,
      { raw: index % 7 === 0 ? 0.3 : 0, corrected: index % 7 === 0 ? 0.3 : 0 }
    );
    return {
      timestampIso: timestamp,
      forecast,
      actual,
      error: {
        temperature2m: 0.3,
        windSpeed10m: 0.2,
        cloudCover: 2,
        visibility: 110,
        uvIndex: 0.1,
        shortwaveRadiation: 14,
        windDirection: 2
      }
    };
  });

  return {
    issuedAtUnixMs: String(WEATHER_ISSUED_AT_UNIX_MS),
    timezone: PROFILE_TIMEZONE,
    verificationSource: 'snapshot',
    provenance: {
      source: 'open_meteo',
      modelSelection: 'best_match',
      actualSource: 'past_days',
      verificationSource: 'snapshot'
    },
    summary: {
      comparedHours: 24,
      matchedHours: 24,
      meanAbsoluteTemperatureError: 0.4,
      meanAbsoluteWindSpeedError: 0.2,
      meanAbsoluteCloudCoverError: 2.1,
      meanAbsoluteVisibilityError: 118,
      meanAbsoluteUvIndexError: 0.1,
      meanAbsoluteRadiationError: 13
    },
    hours
  };
}

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

function buildEnergyRollupPoint(
  bucketStartMs: number,
  durationMinutes: number,
  values: {
    solarGeneratedWh: number;
    acInputEnergyWh: number;
    loadEnergyWh: number;
    pvAvgW: number;
    acInAvgW: number;
    loadAvgW: number;
  }
): Record<string, unknown> {
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
  timezone = PROFILE_TIMEZONE
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

  const resolvedDeviceIds = scope === 'device' && deviceId && DEVICE_BY_KEY.has(deviceId) ? [deviceId] : [DPU_DEVICE_ID, D2M_DEVICE_ID];
  const selectedDevice = resolvedDeviceIds[0];
  const scopedCurrentEnergyPoints =
    scope === 'device' && selectedDevice === D2M_DEVICE_ID ? currentEnergyPoints.slice(0, 2) : currentEnergyPoints;
  const scopedPreviousEnergyPoints =
    scope === 'device' && selectedDevice === D2M_DEVICE_ID ? previousEnergyPoints.slice(0, 1) : previousEnergyPoints;
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
          solarGeneratedKwh: {
            current: 0.26,
            previous: includeComparison ? 0.12 : 0,
            delta: includeComparison ? 0.14 : 0.26,
            deltaPct: includeComparison ? 116.7 : null
          },
          loadConsumedKwh: {
            current: 0.51,
            previous: includeComparison ? 0.44 : 0,
            delta: includeComparison ? 0.07 : 0.51,
            deltaPct: includeComparison ? 15.9 : null
          },
          selfSufficiencyPct: {
            current: 22.0,
            previous: includeComparison ? 18.5 : 0,
            delta: includeComparison ? 3.5 : 22.0,
            deltaPct: includeComparison ? 18.9 : null
          },
          batteryNetKwh: {
            current: -0.08,
            previous: includeComparison ? -0.04 : 0,
            delta: includeComparison ? -0.04 : -0.08,
            deltaPct: includeComparison ? -100 : null
          },
          estimatedValue: {
            current: 0.05,
            previous: includeComparison ? 0.02 : 0,
            delta: includeComparison ? 0.03 : 0.05,
            deltaPct: includeComparison ? 150 : null
          },
          estimatedAcInputCost: {
            current: 0.08,
            previous: includeComparison ? 0.07 : 0,
            delta: includeComparison ? 0.01 : 0.08,
            deltaPct: includeComparison ? 14.3 : null
          },
          currency: 'USD'
        }
      : {
          solarGeneratedKwh: {
            current: 1.26,
            previous: includeComparison ? 0.4 : 0,
            delta: includeComparison ? 0.86 : 1.26,
            deltaPct: includeComparison ? 215 : null
          },
          loadConsumedKwh: {
            current: 0.83,
            previous: includeComparison ? 0.49 : 0,
            delta: includeComparison ? 0.34 : 0.83,
            deltaPct: includeComparison ? 69.4 : null
          },
          selfSufficiencyPct: {
            current: 84.5,
            previous: includeComparison ? 61.3 : 0,
            delta: includeComparison ? 23.2 : 84.5,
            deltaPct: includeComparison ? 37.8 : null
          },
          batteryNetKwh: {
            current: -0.11,
            previous: includeComparison ? -0.05 : 0,
            delta: includeComparison ? -0.06 : -0.11,
            deltaPct: includeComparison ? -120 : null
          },
          estimatedValue: {
            current: 0.24,
            previous: includeComparison ? 0.09 : 0,
            delta: includeComparison ? 0.15 : 0.24,
            deltaPct: includeComparison ? 166.7 : null
          },
          estimatedAcInputCost: {
            current: 0.04,
            previous: includeComparison ? 0.05 : 0,
            delta: includeComparison ? -0.01 : 0.04,
            deltaPct: includeComparison ? -20 : null
          },
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

  const currentPoints = currentWatts.map((pvAvgW, index) => buildRollupPoint(currentBase + index * 10 * 60_000, 10, pvAvgW));
  const previousPoints = previousWatts.map((pvAvgW, index) => buildRollupPoint(previousBase + index * 10 * 60_000, 10, pvAvgW));

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

function buildEnergyCalendar({
  scope = 'all',
  deviceId,
  year = 2026,
  month = 3,
  timezone = PROFILE_TIMEZONE
}: {
  scope?: 'all' | 'device';
  deviceId?: string;
  year?: number;
  month?: number;
  timezone?: string;
}) {
  const dashboard = buildEnergyDashboard({
    includeComparison: false,
    scope,
    deviceId,
    preset: 'today',
    timezone
  });
  const firstOfMonth = new Date(Date.UTC(year, month - 1, 1));
  const start = new Date(firstOfMonth);
  while (start.getUTCDay() !== 0) {
    start.setUTCDate(start.getUTCDate() - 1);
  }
  const lastOfMonth = new Date(Date.UTC(year, month, 0));
  const end = new Date(lastOfMonth);
  while (end.getUTCDay() !== 6) {
    end.setUTCDate(end.getUTCDate() + 1);
  }
  const visibleDayCount = Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1;

  const todayIso = new Date(NOW_UNIX_MS).toISOString().slice(0, 10);
  const visibleDays = Array.from({ length: visibleDayCount }, (_, index) => {
    const date = new Date(start);
    date.setUTCDate(start.getUTCDate() + index);
    const dateIso = date.toISOString().slice(0, 10);
    const isFuture = dateIso > todayIso;
    const dayOffset = index % 7;
    const monthScale = date.getUTCMonth() + 1 === month ? 1 : 0.36;
    const solarGeneratedKwh = isFuture ? 0 : Number((monthScale * (0.24 + dayOffset * 0.11 + (index % 5) * 0.03)).toFixed(2));
    const estimatedValue = Number((solarGeneratedKwh * 0.19).toFixed(2));
    const loadConsumedKwh = isFuture ? 0 : Number((0.62 + dayOffset * 0.08 + (index % 3) * 0.04).toFixed(2));
    const batteryNetKwh = isFuture ? 0 : Number(((dayOffset % 2 === 0 ? -0.05 : 0.07) * monthScale).toFixed(2));
    return {
      dateIso,
      year: date.getUTCFullYear(),
      month: date.getUTCMonth() + 1,
      day: date.getUTCDate(),
      solarGeneratedKwh,
      estimatedValue,
      loadConsumedKwh,
      batteryNetKwh,
      currency: 'USD',
      isCurrentMonth: date.getUTCMonth() + 1 === month && date.getUTCFullYear() === year,
      hasData: !isFuture,
      isToday: dateIso === todayIso,
      isFuture
    };
  });

  const scopeDeviceId = scope === 'device' ? (deviceId ?? DPU_DEVICE_ID) : '';
  const resolvedDeviceIds = scope === 'device' ? [scopeDeviceId] : [DPU_DEVICE_ID, D2M_DEVICE_ID];

  return {
    scope: {
      mode: scope,
      deviceId: scopeDeviceId,
      resolvedDeviceIds
    },
    selectedMonth: {
      year,
      month,
      totals: {
        solarGeneratedKwh: Number((dashboard.summary.solarGeneratedKwh.current * 30).toFixed(2)),
        estimatedValue: Number((dashboard.summary.estimatedValue.current * 30).toFixed(2)),
        loadConsumedKwh: Number((dashboard.summary.loadConsumedKwh.current * 30).toFixed(2)),
        batteryNetKwh: Number((dashboard.summary.batteryNetKwh.current * 30).toFixed(2)),
        currency: 'USD'
      }
    },
    visibleDays
  };
}

async function fulfillJson(route: Route, payload: unknown, status = 200): Promise<void> {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(payload)
  });
}

async function seedAuthenticatedSession(page: Page): Promise<void> {
  await page.addInitScript((configuredApiUrl) => {
    const defaultApiUrl = (): string => {
      const protocol = window.location.protocol === 'https:' ? 'https:' : 'http:';
      return `${protocol}//${window.location.hostname || '127.0.0.1'}`;
    };
    const deriveIssuerUrl = (apiUrl: string): string => {
      const parsed = new URL(apiUrl);
      const normalizedPath = parsed.pathname.replace(/\/+$/, '');
      const basePath = normalizedPath.endsWith('/api')
        ? normalizedPath.slice(0, normalizedPath.length - '/api'.length)
        : normalizedPath;
      const issuerPath = `${basePath}/realms/pulse`.replace(/\/{2,}/g, '/');
      const host = parsed.port ? `${parsed.hostname}:${parsed.port}` : parsed.hostname;

      return `${parsed.protocol}//${host}${issuerPath}`;
    };

    const issuerUrl = deriveIssuerUrl(configuredApiUrl || defaultApiUrl());
    localStorage.setItem(
      'pulse-oidc-session-v1',
      JSON.stringify({
        state: {
          session: {
            issuerUrl,
            clientId: 'pulse-universal-app',
            accessToken: 'web-e2e-access-token',
            refreshToken: 'web-e2e-refresh-token',
            idToken: 'web-e2e-id-token',
            tokenType: 'Bearer',
            expiresAtUnixMs: Date.now() + 60 * 60_000,
            updatedAtUnixMs: Date.now()
          }
        },
        version: 0
      })
    );
  }, process.env.EXPO_PUBLIC_API_URL?.trim() ?? '');
}

export async function mockApiRoutes(page: Page, options: { roles?: string[]; deviceCount?: number } = {}): Promise<void> {
  currentUserBootstrap = JSON.parse(JSON.stringify(CURRENT_USER_BOOTSTRAP)) as CurrentUserPayload;
  currentUserBootstrap.authorization.roles = options.roles ?? currentUserBootstrap.authorization.roles;
  currentUserBootstrap.authorization.deviceCount = options.deviceCount ?? currentUserBootstrap.authorization.deviceCount;
  await seedAuthenticatedSession(page);
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

    if (pathname === '/api/v1/me') {
      const method = route.request().method();
      if (method === 'GET') {
        await fulfillJson(route, currentUserBootstrap);
        return;
      }
      if (method === 'PATCH') {
        const body = (route.request().postDataJSON?.() ?? {}) as Partial<CurrentUserPayload['user']>;
        currentUserBootstrap = {
          ...currentUserBootstrap,
          user: {
            ...currentUserBootstrap.user,
            ...body
          }
        };
        await fulfillJson(route, { user: currentUserBootstrap.user });
        return;
      }
    }

    if (pathname === '/api/v1/me/identity-refresh') {
      await fulfillJson(route, { user: currentUserBootstrap.user });
      return;
    }

    if (pathname === '/api/v1/admin/log-filter-options') {
      const body = (route.request().postDataJSON?.() ?? {}) as { kind?: string; query?: string; provider?: string; deviceIds?: string[] };
      const isAdmin = currentUserBootstrap.authorization.roles.includes('admin');
      if (!isAdmin && body.kind === 'user') {
        await fulfillJson(route, { error: 'admin_role_required' }, 403);
        return;
      }
      const query = String(body.query ?? '').toLowerCase();
      const requestedDeviceIds = new Set(Array.isArray(body.deviceIds) ? body.deviceIds : []);
      const allOptions = [
        {
          kind: 'device',
          id: DPU_DEVICE_ID,
          label: 'DPU A 12 kWh',
          secondaryLabel: 'DELTA Pro Ultra',
          deviceIds: [DPU_DEVICE_ID],
          provider: 'ecoflow'
        },
        {
          kind: 'serial',
          id: DPU_DEVICE_ID,
          label: DPU_SERIAL,
          secondaryLabel: 'DPU A 12 kWh',
          deviceIds: [DPU_DEVICE_ID],
          provider: 'ecoflow'
        },
        {
          kind: 'device',
          id: PECRON_DEVICE_ID,
          label: 'Pecron balcony pack',
          secondaryLabel: 'E1500LFP',
          deviceIds: [PECRON_DEVICE_ID],
          provider: 'pecron'
        },
        {
          kind: 'serial',
          id: PECRON_DEVICE_ID,
          label: PECRON_SERIAL,
          secondaryLabel: 'Pecron balcony pack',
          deviceIds: [PECRON_DEVICE_ID],
          provider: 'pecron'
        },
        {
          kind: 'user',
          id: 'user-1',
          label: 'operator@example.invalid',
          secondaryLabel: 'Pulse Operator',
          deviceIds: [DPU_DEVICE_ID, D2M_DEVICE_ID]
        }
      ].filter((option) =>
        (isAdmin || option.kind !== 'user') &&
        (!body.kind || option.kind === body.kind) &&
        (!body.provider || option.kind === 'user' || option.provider === body.provider) &&
        (requestedDeviceIds.size === 0 || option.deviceIds.some((deviceId) => requestedDeviceIds.has(deviceId))) &&
        (!query || `${option.label} ${option.secondaryLabel}`.toLowerCase().includes(query))
      );
      await fulfillJson(route, { options: allOptions });
      return;
    }

    if (pathname === '/api/v1/integrations') {
      await fulfillJson(route, { integrations: [] });
      return;
    }

    if (pathname === '/api/v1/weather/forecast') {
      await fulfillJson(route, { forecast: buildWeatherForecast() });
      return;
    }

    if (pathname === '/api/v1/weather/yesterday') {
      await fulfillJson(route, { verification: buildYesterdayVerification() });
      return;
    }

    if (pathname === '/api/v1/solar/outlook') {
      await fulfillJson(route, {
        outlook: {
          scope: {
            mode: 'all',
            resolvedDeviceIds: ['device-1', 'device-2']
          },
          provenance: {
            forecastSource: 'solarforecastd',
            forecastModel: 'deterministic_baseline_v1',
            actualsSource: 'telemetry_rollups',
            weatherSource: 'open_meteo',
            weatherModelSelection: 'best_match',
            timezone: 'America/New_York',
            canonicalLocationKey: 'grid-key',
            issuedAtUnixMs: '1773430800000',
            refreshedAtUnixMs: '1773430860000'
          },
          capacity: {
            estimatedPeakWatts: 1680,
            observedPvWatts: 1230,
            method: 'live_pv_and_irradiance'
          },
          today: {
            dateIso: '2026-03-18',
            actualGeneratedKwh: 5.2,
            forecastRemainingKwh: 1.8,
            forecastTotalKwh: 7,
            estimatedPeakWatts: 1680,
            peakTimeIso: '2026-03-18T18:00:00.000Z',
            confidence: 'high'
          },
          daily: [
            {
              dateIso: '2026-03-18',
              actualGeneratedKwh: 5.2,
              forecastRemainingKwh: 1.8,
              forecastTotalKwh: 7,
              estimatedPeakWatts: 1680,
              peakTimeIso: '2026-03-18T18:00:00.000Z',
              confidence: 'high'
            }
          ],
          next24Hours: []
        }
      });
      return;
    }

    if (pathname.startsWith('/api/v1/devices/') && pathname.endsWith('/insights')) {
      const deviceId = decodeURIComponent(pathname.replace('/api/v1/devices/', '').replace('/insights', ''));
      if (!DEVICE_BY_KEY.has(deviceId)) {
        await fulfillJson(route, { error: 'not_found' }, 404);
        return;
      }
      await fulfillJson(route, {
        deviceId,
        status: 'ready',
        statusDetail: 'cached',
        refreshedAtUnixMs: String(NOW_UNIX_MS),
        insights: [
          {
            id: `${deviceId}-battery-expansion`,
            deviceId,
            kind: 'battery_expansion',
            title: 'Add extra battery capacity',
            summary: 'This system can support more stored reserve for longer outages.',
            score: 0.82,
            rank: 1,
            modelKey: 'battery-expansion-rule',
            modelVersion: 'v1',
            generatedAtUnixMs: String(NOW_UNIX_MS),
            expiresAtUnixMs: String(NOW_UNIX_MS + 60 * 60 * 1000),
            tags: ['battery', 'reserve'],
            evidence: [],
            actions: [
              {
                kind: 'learn_more',
                label: 'Review options',
                target: '/energy'
              }
            ],
            attributes: {
              recommended_additional_packs: 1,
              max_battery_packs: 5
            }
          }
        ]
      });
      return;
    }

    if (pathname.startsWith('/api/v1/devices/') && pathname.endsWith('/history/compare')) {
      const deviceId = decodeURIComponent(pathname.replace('/api/v1/devices/', '').replace('/history/compare', ''));
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
          timezone: url.searchParams.get('timezone') ?? PROFILE_TIMEZONE
        })
      );
      return;
    }

    if (pathname === '/api/v1/energy/calendar') {
      const scopeParam = url.searchParams.get('scope');
      const scope = scopeParam === 'device' ? 'device' : 'all';
      await fulfillJson(
        route,
        buildEnergyCalendar({
          scope,
          deviceId: url.searchParams.get('deviceId') ?? undefined,
          year: Number.parseInt(url.searchParams.get('year') ?? '2026', 10),
          month: Number.parseInt(url.searchParams.get('month') ?? '3', 10),
          timezone: url.searchParams.get('timezone') ?? PROFILE_TIMEZONE
        })
      );
      return;
    }

    if (pathname === '/api/v1/energy/pv-history') {
      const scopeParam = url.searchParams.get('scope');
      const scope = scopeParam === 'device' ? 'device' : 'all';
      const dashboard = buildEnergyDashboard({
        includeComparison: false,
        scope,
        deviceId: url.searchParams.get('deviceId') ?? undefined,
        preset: url.searchParams.get('preset') ?? 'today',
        timezone: url.searchParams.get('timezone') ?? PROFILE_TIMEZONE
      });
      await fulfillJson(route, { pvPortHistory: dashboard.pvPortHistory });
      return;
    }

    if (pathname === '/api/v1/energy/comparison-insight') {
      const scopeParam = url.searchParams.get('scope');
      const scope = scopeParam === 'device' ? 'device' : 'all';
      await fulfillJson(route, {
        status: 'ready',
        statusDetail: 'cached',
        insight: {
          id: 'energy-comparison-1',
          scope: {
            mode: scope,
            deviceId: scope === 'device' ? (url.searchParams.get('deviceId') ?? DPU_DEVICE_ID) : '',
            resolvedDeviceIds: scope === 'device' ? [url.searchParams.get('deviceId') ?? DPU_DEVICE_ID] : [DPU_DEVICE_ID, D2M_DEVICE_ID]
          },
          preset: url.searchParams.get('preset') ?? 'today',
          timezone: url.searchParams.get('timezone') ?? PROFILE_TIMEZONE,
          verdictClass: 'solar_freedom_up',
          headline: 'More solar freedom',
          summary: 'Self-sufficiency improved versus the previous window.',
          score: 0.52,
          confidence: 0.83,
          modelKey: 'energy-comparison-score',
          modelVersion: 'v1',
          generatedAtUnixMs: String(NOW_UNIX_MS),
          expiresAtUnixMs: String(NOW_UNIX_MS + 60 * 60 * 1000),
          tags: ['energy', 'comparison'],
          cards: [
            {
              category: 'self_sufficiency',
              title: 'Self-sufficiency',
              summary: 'Self-sufficiency changed by 12.0 percentage points.',
              recommendation: 'Keep flexible loads aligned to the solar window to preserve the self-sufficiency gain.',
              score: 0.67,
              confidence: 0.83,
              evidence: []
            },
            {
              category: 'solar',
              title: 'Solar generation',
              summary: 'Solar generation changed by 0.92kWh.',
              recommendation: 'Keep high-draw tasks inside the strongest solar window when generation climbs.',
              score: 0.44,
              confidence: 0.83,
              evidence: []
            },
            {
              category: 'grid',
              title: 'Grid dependence',
              summary: 'Estimated AC input cost changed by 0.18.',
              recommendation: 'Reduce late-grid peaks when grid cost rises.',
              score: -0.22,
              confidence: 0.83,
              evidence: []
            },
            {
              category: 'value',
              title: 'Solar value',
              summary: 'Estimated solar value changed by 0.37.',
              recommendation: 'Preserve the solar window for flexible consumption.',
              score: 0.31,
              confidence: 0.83,
              evidence: []
            }
          ],
          evidence: []
        }
      });
      return;
    }

    await fulfillJson(route, { error: `unhandled_mock_path:${pathname}` }, 404);
  });
}

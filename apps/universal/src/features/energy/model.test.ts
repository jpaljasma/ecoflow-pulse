import { describe, expect, it } from 'vitest';
import type { DeviceSummary } from '@/features/devices/api';
import {
  buildEnergyInsights,
  buildEnergyTrendSeries,
  buildEnergyRouteParams,
  buildPvEnvelopeSummary,
  buildPowerTrendSeries,
  detectDevicesTimezone,
  energyPresetLabel,
  formatDeltaPct,
  resolveEnergyRouteState
} from '@/features/energy/model';

describe('energy route model', () => {
  it('defaults to all-devices today view with a detected timezone fallback', () => {
    const state = resolveEnergyRouteState({}, [], 'America/New_York');

    expect(state).toEqual({
      scope: 'all',
      deviceId: undefined,
      preset: 'today',
      timezone: 'America/New_York',
      includeComparison: true
    });
  });

  it('falls back to the first available device when device scope is selected without a valid id', () => {
    const state = resolveEnergyRouteState(
      { scope: 'device', preset: 'last7d', timezone: 'UTC' },
      ['019c9f0e-4521-775d-873e-e80039f16d75']
    );

    expect(state.scope).toBe('device');
    expect(state.deviceId).toBe('019c9f0e-4521-775d-873e-e80039f16d75');
    expect(state.preset).toBe('last7d');
  });

  it('uses the first device-reported timezone when the route does not specify one', () => {
    const devices: DeviceSummary[] = [
      {
        id: '019c9f0e-4521-775d-873e-e80039f16d75',
        serialNumber: 'SN-1',
        name: 'DPU',
        model: 'DELTA Pro Ultra',
        online: true,
        batteryPct: 50,
        state: 'idle',
        etaMinutes: 0,
        details: {
          timezoneId: 'America/New_York'
        }
      },
      {
        id: '019c9f0e-4521-775d-873e-e80039f16d76',
        serialNumber: 'SN-2',
        name: 'D2M',
        model: 'DELTA 2 Max',
        online: true,
        batteryPct: 60,
        state: 'idle',
        etaMinutes: 0,
        details: {
          timezoneId: 'America/Los_Angeles'
        }
      }
    ];

    expect(detectDevicesTimezone(devices)).toBe('America/New_York');
    expect(
      resolveEnergyRouteState({}, devices.map((device) => device.id), detectDevicesTimezone(devices))
    ).toEqual({
      scope: 'all',
      deviceId: undefined,
      preset: 'today',
      timezone: 'America/New_York',
      includeComparison: true
    });
  });

  it('omits deviceId from route params for all-device mode', () => {
    expect(
      buildEnergyRouteParams({
        scope: 'all',
        preset: 'today',
        timezone: 'UTC',
        includeComparison: false
      })
    ).toEqual({
      device: 'all',
      preset: 'today',
      tz: 'UTC',
      compare: '0'
    });
  });

  it('accepts spec-style route params and legacy aliases', () => {
    const specState = resolveEnergyRouteState(
      { device: 'all', preset: 'today', tz: 'UTC', compare: '0' },
      ['019c9f0e-4521-775d-873e-e80039f16d75']
    );
    expect(specState).toEqual({
      scope: 'all',
      deviceId: undefined,
      preset: 'today',
      timezone: 'UTC',
      includeComparison: false
    });

    const legacyState = resolveEnergyRouteState(
      {
        scope: 'device',
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        preset: 'today',
        timezone: 'UTC',
        includeComparison: 'true'
      },
      ['019c9f0e-4521-775d-873e-e80039f16d75']
    );
    expect(legacyState.scope).toBe('device');
    expect(legacyState.deviceId).toBe('019c9f0e-4521-775d-873e-e80039f16d75');
    expect(legacyState.includeComparison).toBe(true);
  });

  it('maps rollup points into power trend arrays', () => {
    const series = buildPowerTrendSeries([
      {
        bucketStartUnixMs: '1',
        bucketEndUnixMs: '2',
        sampleCount: 1,
        firstTsUnixMs: '1',
        lastTsUnixMs: '2',
        metrics: {
          socAvgPct: 50,
          socMinPct: 40,
          socMaxPct: 60,
          acInAvgW: 10,
          acInMaxW: 12,
          acOutputAvgW: 85,
          acOutputMaxW: 93,
          pvAvgW: 120,
          pvMaxW: 140,
          dcAvgW: 5,
          dcMaxW: 7,
          loadAvgW: 90,
          loadMaxW: 100,
          netAvgW: 25,
          netMinW: 0,
          netMaxW: 50,
          batteryAvgW: 20,
          batteryMinW: -10,
          batteryMaxW: 40,
          tempAvgC: 23,
          tempMinC: 21,
          tempMaxC: 25,
          solarGeneratedWh: 30,
          acInputEnergyWh: 10,
          acOutputEnergyWh: 85,
          dcOutputEnergyWh: 5,
          loadEnergyWh: 90,
          batteryChargeEnergyWh: 20,
          batteryDischargeEnergyWh: 0
        }
      }
    ]);

    expect(series).toEqual({
      solar: [120],
      ac: [10],
      dc: [5],
      load: [90],
      battery: [20]
    });
  });

  it('maps rollup points into energy trend arrays', () => {
    const series = buildEnergyTrendSeries([
      {
        bucketStartUnixMs: '1',
        bucketEndUnixMs: '2',
        sampleCount: 1,
        firstTsUnixMs: '1',
        lastTsUnixMs: '2',
        metrics: {
          socAvgPct: 50,
          socMinPct: 40,
          socMaxPct: 60,
          acInAvgW: 10,
          acInMaxW: 12,
          acOutputAvgW: 85,
          acOutputMaxW: 93,
          pvAvgW: 120,
          pvMaxW: 140,
          dcAvgW: 5,
          dcMaxW: 7,
          loadAvgW: 90,
          loadMaxW: 100,
          netAvgW: 25,
          netMinW: 0,
          netMaxW: 50,
          batteryAvgW: 20,
          batteryMinW: -10,
          batteryMaxW: 40,
          tempAvgC: 23,
          tempMinC: 21,
          tempMaxC: 25,
          solarGeneratedWh: 300,
          acInputEnergyWh: 100,
          acOutputEnergyWh: 85,
          dcOutputEnergyWh: 5,
          loadEnergyWh: 900,
          batteryChargeEnergyWh: 20,
          batteryDischargeEnergyWh: 0
        }
      }
    ]);

    expect(series).toEqual({
      solar: [0.3],
      grid: [0.1],
      acOutput: [0.085],
      load: [0.9],
      dcOutput: [0.005],
      batteryCharge: [0.02],
      batteryDischarge: [0]
    });
  });

  it('formats preset labels and comparison text', () => {
    expect(energyPresetLabel('past24h')).toBe('Last 24h');
    expect(energyPresetLabel('last30d')).toBe('Last 30d');
    expect(energyPresetLabel('lastMonth')).toBe('Last month');
    expect(energyPresetLabel('last12m')).toBe('Last 12 months');
    expect(energyPresetLabel('previousWeek')).toBe('Previous week');
    expect(formatDeltaPct(12.34)).toBe('+12.3% vs previous');
    expect(formatDeltaPct(null)).toBe('No prior baseline');
  });

  it('builds insight cards from power buckets', () => {
    const insights = buildEnergyInsights(
      [
        {
          bucketStartUnixMs: '1710000000000',
          bucketEndUnixMs: '1710003600000',
          sampleCount: 1,
          firstTsUnixMs: '1710000000000',
          lastTsUnixMs: '1710003600000',
          metrics: {
            socAvgPct: 50,
            socMinPct: 45,
            socMaxPct: 55,
            acInAvgW: 10,
            acInMaxW: 10,
            acOutputAvgW: 45,
            acOutputMaxW: 50,
            pvAvgW: 100,
            pvMaxW: 120,
            dcAvgW: 5,
            dcMaxW: 5,
            loadAvgW: 50,
            loadMaxW: 55,
            netAvgW: 50,
            netMinW: 45,
            netMaxW: 55,
            batteryAvgW: 0,
            batteryMinW: 0,
            batteryMaxW: 0,
            tempAvgC: 20,
            tempMinC: 19,
            tempMaxC: 21,
            solarGeneratedWh: 40,
            acInputEnergyWh: 10,
            acOutputEnergyWh: 45,
            dcOutputEnergyWh: 5,
            loadEnergyWh: 50,
            batteryChargeEnergyWh: 0,
            batteryDischargeEnergyWh: 0
          }
        },
        {
          bucketStartUnixMs: '1710003600000',
          bucketEndUnixMs: '1710007200000',
          sampleCount: 1,
          firstTsUnixMs: '1710003600000',
          lastTsUnixMs: '1710007200000',
          metrics: {
            socAvgPct: 50,
            socMinPct: 45,
            socMaxPct: 55,
            acInAvgW: 12,
            acInMaxW: 12,
            acOutputAvgW: 135,
            acOutputMaxW: 145,
            pvAvgW: 180,
            pvMaxW: 200,
            dcAvgW: 5,
            dcMaxW: 5,
            loadAvgW: 140,
            loadMaxW: 150,
            netAvgW: 40,
            netMinW: 35,
            netMaxW: 45,
            batteryAvgW: 0,
            batteryMinW: 0,
            batteryMaxW: 0,
            tempAvgC: 20,
            tempMinC: 19,
            tempMaxC: 21,
            solarGeneratedWh: 60,
            acInputEnergyWh: 12,
            acOutputEnergyWh: 135,
            dcOutputEnergyWh: 5,
            loadEnergyWh: 140,
            batteryChargeEnergyWh: 0,
            batteryDischargeEnergyWh: 0
          }
        }
      ],
      'UTC',
      'today',
      [
        {
          deviceId: 'd1',
          deviceName: 'DPU A 12 kWh',
          portId: 'pv-high',
          portLabel: 'PV High',
          observedVolts: 330,
          observedAmps: 0.8,
          observedPower: 3900,
          maxVolts: 450,
          maxAmps: 15,
          maxPower: 4000,
          powerUtilizationPct: 97.5,
          voltageHeadroom: 120,
          currentHeadroom: 14.2,
          bottleneckHint: 'Near power ceiling'
        }
      ]
    );

    expect(insights).toHaveLength(5);
    expect(insights[0]?.body).toContain('180W');
    expect(insights[1]?.body).toContain('140W');
    expect(insights[4]?.title).toBe('Likely clipping / bottlenecks');
    expect(insights[4]?.body).toContain('near power ceiling');
  });

  it('builds a PV envelope summary from device port metadata', () => {
    const summary = buildPvEnvelopeSummary(
      [
        {
          id: 'd1',
          serialNumber: 'sn1',
          name: 'Alpha',
          model: 'DELTA 2 Max',
          online: true,
          batteryPct: 50,
          state: 'idle',
          etaMinutes: 0,
          details: {
            solarPorts: [
              {
                id: 'pv1',
                name: 'PV1',
                volts: 45,
                amps: 9,
                watts: 405,
                maxVolts: 60,
                maxAmps: 10,
                maxWatts: 500
              }
            ]
          }
        }
      ],
      'all'
    );

    expect(summary.observedPower).toBe(405);
    expect(summary.configuredPower).toBe(500);
    expect(summary.utilizationPct).toBeCloseTo(81);
    expect(summary.rows[0]?.bottleneckHint).toBe('Within envelope');
  });
});

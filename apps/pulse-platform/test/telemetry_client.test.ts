import { describe, expect, it } from 'vitest';

import type { RollupPoint } from '../src/grpc/telemetryClient.js';
import { buildEnergyCalendarRequest, buildEnergyDashboardRequest, fillDerivedBucketEnergy } from '../src/grpc/telemetryClient.js';

function makePoint(overrides: Partial<RollupPoint['metrics']> = {}, bucketStartUnixMs = '0', bucketEndUnixMs = '60000'): RollupPoint {
  return {
    bucketStartUnixMs,
    bucketEndUnixMs,
    sampleCount: 1,
    firstTsUnixMs: bucketStartUnixMs,
    lastTsUnixMs: bucketEndUnixMs,
    metrics: {
      socAvgPct: 0,
      socMinPct: 0,
      socMaxPct: 0,
      acInAvgW: 0,
      acInMaxW: 0,
      acOutputAvgW: 0,
      acOutputMaxW: 0,
      pvAvgW: 0,
      pvMaxW: 0,
      dcAvgW: 0,
      dcMaxW: 0,
      loadAvgW: 0,
      loadMaxW: 0,
      netAvgW: 0,
      netMinW: 0,
      netMaxW: 0,
      batteryAvgW: 0,
      batteryMinW: 0,
      batteryMaxW: 0,
      tempAvgC: 0,
      tempMinC: 0,
      tempMaxC: 0,
      solarGeneratedWh: 0,
      acInputEnergyWh: 0,
      acOutputEnergyWh: 0,
      dcOutputEnergyWh: 0,
      loadEnergyWh: 0,
      batteryChargeEnergyWh: 0,
      batteryDischargeEnergyWh: 0,
      ...overrides
    }
  };
}

describe('telemetry client bucket energy derivation', () => {
  it('derives replay bucket energy from average power when explicit energy is missing', () => {
    const point = fillDerivedBucketEnergy(
      makePoint({
        pvAvgW: 120,
        acInAvgW: 60,
        acOutputAvgW: 180,
        dcAvgW: 12,
        loadAvgW: 192,
        batteryAvgW: 48
      })
    );

    expect(point.metrics.solarGeneratedWh).toBeCloseTo(2, 6);
    expect(point.metrics.acInputEnergyWh).toBeCloseTo(1, 6);
    expect(point.metrics.acOutputEnergyWh).toBeCloseTo(3, 6);
    expect(point.metrics.dcOutputEnergyWh).toBeCloseTo(0.2, 6);
    expect(point.metrics.loadEnergyWh).toBeCloseTo(3.2, 6);
    expect(point.metrics.batteryChargeEnergyWh).toBeCloseTo(0.8, 6);
    expect(point.metrics.batteryDischargeEnergyWh).toBe(0);
  });

  it('uses load minus dc as an AC-output fallback and derives discharge from negative battery power', () => {
    const point = fillDerivedBucketEnergy(
      makePoint(
        {
          acOutputAvgW: 0,
          dcAvgW: 20,
          loadAvgW: 150,
          batteryAvgW: -90
        },
        '0',
        String(10 * 60 * 1000)
      )
    );

    expect(point.metrics.acOutputEnergyWh).toBeCloseTo((130 * 10) / 60, 6);
    expect(point.metrics.batteryChargeEnergyWh).toBe(0);
    expect(point.metrics.batteryDischargeEnergyWh).toBeCloseTo((90 * 10) / 60, 6);
  });

  it('preserves persisted explicit energy values when they are present', () => {
    const point = fillDerivedBucketEnergy(
      makePoint(
        {
          pvAvgW: 900,
          solarGeneratedWh: 240,
          acInAvgW: 500,
          acInputEnergyWh: 125,
          batteryAvgW: -200,
          batteryDischargeEnergyWh: 70
        },
        '0',
        String(60 * 60 * 1000)
      )
    );

    expect(point.metrics.solarGeneratedWh).toBe(240);
    expect(point.metrics.acInputEnergyWh).toBe(125);
    expect(point.metrics.batteryDischargeEnergyWh).toBe(70);
  });
});

describe('telemetry client energy request builders', () => {
  it('builds calendar requests with explicit month, timezone, and scope fields', () => {
    expect(
      buildEnergyCalendarRequest({
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        useAllDevices: false,
        year: 2026,
        month: 3,
        timezone: 'America/New_York',
        gridPricePerKwh: 0.3,
        currency: 'USD',
        deadlineMs: 2500
      })
    ).toEqual({
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
      useAllDevices: false,
      year: 2026,
      month: 3,
      timezone: 'America/New_York',
      gridPricePerKwh: 0.3,
      currency: 'USD'
    });
  });

  it('passes selected dashboard dates through without rewriting them', () => {
    expect(
      buildEnergyDashboardRequest({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'today',
        timezone: 'America/New_York',
        includeComparison: true,
        date: '2026-03-08',
        deadlineMs: 2500
      })
    ).toEqual({
      deviceId: '',
      useAllDevices: true,
      preset: 'today',
      timezone: 'America/New_York',
      includeComparison: true,
      date: '2026-03-08',
      gridPricePerKwh: 0,
      currency: ''
    });
  });
});

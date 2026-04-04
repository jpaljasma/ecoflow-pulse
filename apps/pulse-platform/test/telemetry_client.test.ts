import { describe, expect, it } from 'vitest';

import type { RollupPoint } from '../src/grpc/telemetryClient.js';
import { fillDerivedBucketEnergy } from '../src/grpc/telemetryClient.js';

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

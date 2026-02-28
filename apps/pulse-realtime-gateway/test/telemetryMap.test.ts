import { describe, expect, it } from 'vitest';

import { deriveTelemetryMetrics, mergeRawMetrics } from '../src/telemetryMap.js';

describe('telemetryMap', () => {
  it('derives UI telemetry metrics from flattened ecoflow metrics', () => {
    const metrics = deriveTelemetryMetrics({
      'params.f32ShowSoc': 54.8,
      'params.inLvMpptPwr': 22.63,
      'params.wattsInSum': 29.39,
      'params.wattsOutSum': 0,
      'params.batAmp': -1.6,
      'params.batVol': 54.7,
      'params.cellTemp.0': 19,
      'params.cellTemp.1': 21,
      'params.cellTemp.2': 20,
      'params.outUsb1Pwr': 3,
      'params.outUsb2Pwr': 4
    });

    expect(metrics.soc).toBeCloseTo(54.8, 6);
    expect(metrics.pvW).toBeCloseTo(22.63, 6);
    expect(metrics.acW).toBeCloseTo(6.76, 2);
    expect(metrics.dcW).toBeCloseTo(7, 6);
    expect(metrics.batteryW).toBeCloseTo(-87.52, 2);
    expect(metrics.tempC).toBe(20);
  });

  it('merges changed and cleared raw metrics', () => {
    const merged = mergeRawMetrics(
      { 'params.wattsInSum': 25, 'params.pv1ChargeWatts': 10, 'params.temp': 21 },
      { 'params.wattsInSum': 30, 'params.pv2ChargeWatts': 12 },
      ['params.temp']
    );

    expect(merged).toEqual({
      'params.wattsInSum': 30,
      'params.pv1ChargeWatts': 10,
      'params.pv2ChargeWatts': 12
    });
  });
});

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

  it('prefers canonical D2M quota PV fields over broken MPPT power fields', () => {
    const metrics = deriveTelemetryMetrics({
      'params.f32ShowSoc': 90,
      'params.pv1ChargeWatts': 158,
      'params.pv2ChargeWatts': 333,
      'params.inLvMpptPwr': 175288048,
      'params.inHvMpptPwr': 443476824,
      'params.wattsInSum': 491,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(491);
    expect(metrics.acW).toBe(0);
  });

  it('does not double-count explicit pvW with provider-specific MPPT totals', () => {
    const metrics = deriveTelemetryMetrics({
      pvW: 167.1652,
      'params.inLvMpptPwr': 167.1652,
      'params.wattsInSum': 167.1652,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBeCloseTo(167.1652, 6);
    expect(metrics.acW).toBeCloseTo(0, 6);
  });

  it('does not sum duplicate DPU MPPT power representations', () => {
    const metrics = deriveTelemetryMetrics({
      'params.inLvMpptPwr': 157.1652,
      'param.powGetPvL': 157.1652,
      'params.inHvMpptPwr': 0,
      'param.powGetPvH': 0
    });

    expect(metrics.pvW).toBeCloseTo(157.1652, 6);
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

  it('drops impossible MPPT fallback power values', () => {
    const metrics = deriveTelemetryMetrics({
      'params.f32ShowSoc': 31.5,
      'params.inLvMpptPwr': 175288048,
      'params.inHvMpptPwr': 443476824,
      'params.wattsInSum': 144,
      'params.wattsOutSum': 138
    });

    expect(metrics.pvW).toBe(0);
    expect(metrics.acW).toBe(144);
  });
});

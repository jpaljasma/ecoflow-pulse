import { describe, expect, it } from 'vitest';

import { deriveTelemetryMetrics } from '../src/telemetry/deriveMetrics.js';

describe('deriveTelemetryMetrics', () => {
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

  it('falls back to MPPT power fields when canonical quota PV fields are absent', () => {
    const metrics = deriveTelemetryMetrics({
      'params.cmsBattSoc': 19.7,
      'params.inLvMpptPwr': 720.80133,
      'params.inHvMpptPwr': 0,
      'params.wattsInSum': 721,
      'params.wattsOutSum': 140
    });

    expect(metrics.pvW).toBeCloseTo(720.80133, 6);
    expect(metrics.acW).toBeCloseTo(0.19867, 4);
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

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

  it('does not count D2M extra-battery transfer as external load', () => {
    const metrics = deriveTelemetryMetrics({
      'params.f32LcdShowSoc': 21.5,
      'params.pv2ChargeWatts': 435,
      'params.wattsInSum': 435,
      'params.wattsOutSum': 183,
      'params.XT150Watts1': 183,
      'params.inputWatts': 183,
      'params.outputWatts': 0,
      'params.bmsInputWatts': 0,
      'params.bmsOutputWatts': 0,
      'params.invOutWatts': 0,
      'params.carWatts': 0,
      'params.wireWatts': 0,
      'params.typec1Watts': 0,
      'params.typec2Watts': 0,
      'params.usb1Watts': 0,
      'params.usb2Watts': 0
    });

    expect(metrics.pvW).toBe(435);
    expect(metrics.acW).toBe(0);
    expect(metrics.loadW).toBe(0);
    expect(metrics.batteryW).toBe(183);
  });

  it('keeps real output load while subtracting D2M extra-battery transfer', () => {
    const metrics = deriveTelemetryMetrics({
      'params.pv2ChargeWatts': 360,
      'params.wattsInSum': 360,
      'params.wattsOutSum': 300,
      'params.XT150Watts1': 180,
      'params.inputWatts': 180,
      'params.outputWatts': 0,
      'params.typec1Watts': 120
    });

    expect(metrics.loadW).toBe(120);
  });

  it('derives battery charging from positive power balance when BMS counters are flat zero', () => {
    const metrics = deriveTelemetryMetrics({
      'params.f32LcdShowSoc': 35.5,
      'params.pv2ChargeWatts': 483,
      'params.wattsInSum': 483,
      'params.wattsOutSum': 207,
      'params.bmsInputWatts': 0,
      'params.bmsOutputWatts': 0,
      'params.inputWatts': 0,
      'params.outputWatts': 0
    });

    expect(metrics.pvW).toBe(483);
    expect(metrics.loadW).toBe(207);
    expect(metrics.batteryW).toBe(276);
  });

  it('derives DPU Anderson watts from backend volts and amps when appshow power is zero', () => {
    const metrics = deriveTelemetryMetrics({
      'params.outAdsPwr': 0,
      'params.outAdsAmp': 0.69193053,
      'params.outAdsVol': 12.95,
      'params.outUsb1Pwr': 3,
      'params.outTypec2Pwr': 5
    });

    expect(metrics.dcW).toBeCloseTo(16.96, 2);
  });

  it('normalizes milli-unit battery voltage and current before deriving watts', () => {
    const metrics = deriveTelemetryMetrics({
      'params.batVol': 50288,
      'params.batAmp': -316.8
    });

    expect(metrics.batteryW).toBeCloseTo(-15.93, 2);
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

  it('ignores absurd top-level pvW and uses canonical per-port values', () => {
    const metrics = deriveTelemetryMetrics({
      pvW: 1757819.6,
      'params.pv1ChargeWatts': 57,
      'params.pv2ChargeWatts': 45,
      'params.wattsInSum': 102,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(102);
    expect(metrics.acW).toBe(0);
  });

  it('does not treat chgSunPower as instantaneous PV watts', () => {
    const metrics = deriveTelemetryMetrics({
      'params.chgSunPower': 260,
      'params.wattsInSum': 0,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(0);
  });

  it('prefers explicit zero canonical PV fields over stale top-level pvW', () => {
    const metrics = deriveTelemetryMetrics({
      pvW: 260,
      'params.pv1ChargeWatts': 0,
      'params.pv2ChargeWatts': 0,
      'params.inLvMpptPwr': 0,
      'params.inHvMpptPwr': 0,
      'params.wattsInSum': 0,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(0);
    expect(metrics.acW).toBe(0);
  });

  it('prefers explicit zero DPU MPPT fields over stale pv1ChargeWatts', () => {
    const metrics = deriveTelemetryMetrics({
      'params.pv1ChargeWatts': 260,
      'params.inLvMpptPwr': 0,
      'params.inHvMpptPwr': 0,
      'params.wattsInSum': 0,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(0);
    expect(metrics.acW).toBe(0);
  });

  it('caps stale live PV against total input when cached MPPT fields drift high', () => {
    const metrics = deriveTelemetryMetrics({
      'params.inLvMpptPwr': 582,
      'params.inHvMpptPwr': 0,
      'params.wattsInSum': 324,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(324);
    expect(metrics.acW).toBe(0);
  });

  it('subtracts explicit AC input before capping stale live PV', () => {
    const metrics = deriveTelemetryMetrics({
      'params.inLvMpptPwr': 582,
      'params.inHvMpptPwr': 0,
      'params.inAcC20Pwr': 100,
      'params.wattsInSum': 424,
      'params.wattsOutSum': 0
    });

    expect(metrics.pvW).toBe(324);
    expect(metrics.acW).toBe(100);
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

  it('prefers aggregate display soc fields over main-pack soc fields', () => {
    const metrics = deriveTelemetryMetrics({
      'params.targetSoc': 22.94,
      'params.bpPowerSoc': 25,
      'params.f32LcdShowSoc': 25.49
    });

    expect(metrics.soc).toBeCloseTo(25.49, 2);
  });

  it('prefers canonical DPU soc over backup reserve soc', () => {
    const metrics = deriveTelemetryMetrics({
      'params.bpPowerSoc': 18,
      'params.soc': 73.15,
      'params.wattsInSum': 889.79,
      'params.wattsOutSum': 705.89
    });

    expect(metrics.soc).toBeCloseTo(73.15, 2);
  });

  it('does not let stale top-level soc override canonical provider soc', () => {
    const metrics = deriveTelemetryMetrics({
      soc: 98.8,
      'params.soc': 71,
      'params.wattsInSum': 889.79,
      'params.wattsOutSum': 705.89
    });

    expect(metrics.soc).toBeCloseTo(71, 2);
  });
});

import { describe, expect, it } from 'vitest';

import {
  deriveTelemetryDetail,
  deriveTelemetryMetrics,
  mergeRawMetrics
} from '../src/telemetryMap.js';

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

  it('prefers aggregate display soc fields over main-pack soc fields', () => {
    const metrics = deriveTelemetryMetrics({
      'params.targetSoc': 22.94,
      'params.bpPowerSoc': 25,
      'params.f32LcdShowSoc': 25.49
    });

    expect(metrics.soc).toBeCloseTo(25.49, 2);
  });

  it('derives live D2M system signals and solar ports for device detail realtime updates', () => {
    const detail = deriveTelemetryDetail({
      'params.pv1ChargeWatts': 82,
      'params.pv2ChargeWatts': 212,
      'params.inVol': 16400,
      'params.inAmp': 5000,
      'params.pv2InVol': 40900,
      'params.pv2InAmp': 5180,
      'params.chgState': 2,
      'params.pv2ChgState': 1,
      'params.cfgAcEnabled': 1,
      'params.dcOutState': 1,
      'params.typec1Watts': 42,
      'params.fanState': 1
    });

    expect(detail).toEqual({
      signals: {
        acOn: true,
        dcOn: true,
        usbOn: true,
        dc12vOn: true,
        fanOn: true,
        solarChargingOn: true
      },
      solarPorts: [
        { id: 'pv-1', name: 'PV 1', state: 'charging', volts: 16.4, amps: 5, watts: 82 },
        { id: 'pv-2', name: 'PV 2', state: 'charging', volts: 40.9, amps: 5.18, watts: 212 }
      ]
    });
  });

  it('prefers explicit preconditioning state over stale heat-time fallback', () => {
    const detail = deriveTelemetryDetail({
      'params.bpInfo.0.heatTime': 9,
      'params.ptcMosState': 0
    });

    expect(detail).toEqual({
      signals: {
        batteryHeatingOn: false
      }
    });
  });
});

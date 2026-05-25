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
    expect(metrics.batteryW).toBeCloseTo(29.39, 2);
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

  it('normalizes milli-unit battery voltage and current before deriving watts', () => {
    const metrics = deriveTelemetryMetrics({
      'params.batVol': 50288,
      'params.batAmp': -316.8
    });

    expect(metrics.batteryW).toBeCloseTo(-15.93, 2);
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

  it('uses positive power balance over conflicting pack-current sign', () => {
    const metrics = deriveTelemetryMetrics({
      'params.soc': 72,
      'params.inLvMpptPwr': 612,
      'params.inHvMpptPwr': 0,
      'params.wattsInSum': 612,
      'params.wattsOutSum': 535,
      'params.batAmp': -10,
      'params.batVol': 50
    });

    expect(metrics.pvW).toBe(612);
    expect(metrics.loadW).toBe(535);
    expect(metrics.batteryW).toBe(77);
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

  it('derives live D2M system signals and solar ports for device detail realtime updates', () => {
    const detail = deriveTelemetryDetail({
      'params.pv1ChargeWatts': 25,
      'params.pv2ChargeWatts': 212,
      'params.outWatts': 82,
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
        { id: 'pv-1', name: 'PV 1', state: 'charging', volts: 16.4, amps: 5, watts: 25 },
        { id: 'pv-2', name: 'PV 2', state: 'charging', volts: 40.9, amps: 5.18, watts: 212 }
      ]
    });
  });

  it('falls back to outWatts for D2M pv1 only when direct per-port watts are absent', () => {
    const detail = deriveTelemetryDetail({
      'params.outWatts': 82,
      'params.pv2ChargeWatts': 212,
      'params.inVol': 16400,
      'params.inAmp': 5000,
      'params.pv2InVol': 40900,
      'params.pv2InAmp': 5180,
      'params.chgState': 2,
      'params.pv2ChgState': 1
    });

    expect(detail?.solarPorts).toEqual([
      { id: 'pv-1', name: 'PV 1', state: 'charging', volts: 16.4, amps: 5, watts: 82 },
      { id: 'pv-2', name: 'PV 2', state: 'charging', volts: 40.9, amps: 5.18, watts: 212 }
    ]);
  });

  it('derives numbered live solar ports beyond pv-2 when future devices expose them', () => {
    const detail = deriveTelemetryDetail({
      'params.outWatts': 82,
      'params.inVol': 16400,
      'params.inAmp': 5000,
      'params.chgState': 2,
      'params.pv2InVol': 40900,
      'params.pv2InAmp': 5180,
      'params.pv2ChargeWatts': 212,
      'params.pv2ChgState': 1,
      'params.pv3InVol': 38100,
      'params.pv3InAmp': 4070,
      'params.pv3ChargeWatts': 155,
      'params.pv3ChgState': 2
    });

    expect(detail).toEqual({
      signals: {
        solarChargingOn: true
      },
      solarPorts: [
        { id: 'pv-1', name: 'PV 1', state: 'charging', volts: 16.4, amps: 5, watts: 82 },
        { id: 'pv-2', name: 'PV 2', state: 'charging', volts: 40.9, amps: 5.18, watts: 212 },
        { id: 'pv-3', name: 'PV 3', state: 'charging', volts: 38.1, amps: 4.07, watts: 155 }
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

  it('treats DPU AC output and always-on mode as AC on even when showFlag is stale', () => {
    const detail = deriveTelemetryDetail({
      'params.showFlag': 2322,
      'params.acOftenOpenFlg': 1,
      'params.outAcTtPwr': 694.04,
      'params.outAdsPwr': 10.12
    });

    expect(detail).toEqual({
      signals: {
        acOn: true,
        dcOn: true,
        dc12vOn: true
      }
    });
  });

  it('prefers DPU live MPPT ports over stray pv1ChargeWatts fields without D2M status hints', () => {
    const detail = deriveTelemetryDetail({
      'params.inLvMpptPwr': 105.03,
      'params.inHvMpptPwr': 0,
      'params.inLvMpptVol': 65.18,
      'params.inLvMpptAmp': 1.63,
      'params.inHvMpptVol': 0,
      'params.inHvMpptAmp': 0,
      'params.pv1ChargeWatts': 260
    });

    expect(detail).toEqual({
      signals: {
        solarChargingOn: true
      },
      solarPorts: [
        { id: 'pv-low', name: 'PV Low', state: 'charging', volts: 65.18, amps: 1.63, watts: 105.03 },
        { id: 'pv-high', name: 'PV High', state: 'inactive', volts: 0, amps: 0, watts: 0 }
      ]
    });
  });

  it('treats positive DPU solar watts as charging even if a stale frame still reports zero volts and amps', () => {
    const detail = deriveTelemetryDetail({
      'params.inLvMpptPwr': 121,
      'params.inLvMpptVol': 0,
      'params.inLvMpptAmp': 0,
      'params.inHvMpptPwr': 0,
      'params.inHvMpptVol': 0,
      'params.inHvMpptAmp': 0
    });

    expect(detail).toEqual({
      signals: {
        solarChargingOn: true
      },
      solarPorts: [
        { id: 'pv-low', name: 'PV Low', state: 'charging', volts: 0, amps: 0, watts: 121 },
        { id: 'pv-high', name: 'PV High', state: 'inactive', volts: 0, amps: 0, watts: 0 }
      ]
    });
  });
});

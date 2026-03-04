type RawMetrics = Record<string, number>;

export type DerivedTelemetryMetrics = {
  soc: number;
  pvW: number;
  loadW: number;
  batteryW: number;
  tempC: number;
  acW: number;
  dcW: number;
};

export function deriveTelemetryMetrics(raw: RawMetrics): DerivedTelemetryMetrics {
  const soc = firstNumber(
    raw,
    'soc',
    'params.f32LcdShowSoc',
    'params.lcdShowSoc',
    'params.bpPowerSoc',
    'params.f32ShowSoc',
    'params.f32Soc',
    'param.cmsBattSoc',
    'params.cmsBattSoc',
    'params.soc',
    'param.soc'
  ) ?? 0;

  const pv = derivePv(raw);

  const acIn =
    sumIfPresent(raw, 'acW', 'params.inAcC20Pwr', 'params.inAc5p8Pwr') ??
    firstNumber(raw, 'params.invInWatts') ??
    deriveAcFromInputMinusPv(raw, pv) ??
    0;

  const dc =
    sumIfPresent(
      raw,
      'dcW',
      'params.carWatts',
      'params.wireWatts',
      'params.usb1Watts',
      'params.usb2Watts',
      'params.qcUsb1Watts',
      'params.qcUsb2Watts',
      'params.typec1Watts',
      'params.typec2Watts',
      'params.outUsb1Pwr',
      'params.outUsb2Pwr',
      'params.outTypec1Pwr',
      'params.outTypec2Pwr',
      'params.outPrPwr',
      'params.outAdsPwr'
    ) ?? 0;

  const load =
    firstNumber(raw, 'loadW', 'params.wattsOutSum', 'param.wattsOutSum') ??
    sumIfPresent(
      raw,
      'params.outAcL11Pwr',
      'params.outAcL12Pwr',
      'params.outAcL14Pwr',
      'params.outAcL21Pwr',
      'params.outAcL22Pwr',
      'params.outAcTtPwr',
      'params.outAc5p8Pwr',
      'params.outUsb1Pwr',
      'params.outUsb2Pwr',
      'params.outTypec1Pwr',
      'params.outTypec2Pwr',
      'params.outPrPwr',
      'params.outAdsPwr'
    ) ??
    firstNumber(raw, 'params.invOutWatts') ??
    0;

  const battery = deriveBattery(raw) ?? (acIn + pv - load);
  const temp = deriveTemperature(raw);

  return {
    soc,
    pvW: pv,
    loadW: load,
    batteryW: battery,
    tempC: temp,
    acW: acIn,
    dcW: dc
  };
}

function derivePv(raw: RawMetrics): number {
  return (
    firstNumberCapped(raw, 10000, 'pvW') ??
    sumIfPresentCapped(raw, 10000, 'params.pv1ChargeWatts', 'params.pv2ChargeWatts') ??
    sumIfPresentCapped(raw, 10000, 'params.inLvMpptPwr', 'params.inHvMpptPwr') ??
    sumIfPresentCapped(raw, 10000, 'param.powGetPvL', 'param.powGetPvH') ??
    0
  );
}

function firstNumberCapped(raw: RawMetrics, maxAbs: number, ...keys: string[]): number | undefined {
  for (const key of keys) {
    const value = firstNumber(raw, key);
    if (value === undefined) {
      continue;
    }
    if (value < -maxAbs || value > maxAbs) {
      continue;
    }
    return value;
  }
  return undefined;
}

export function deriveTelemetryState(batteryW: number): 'charging' | 'discharging' | 'idle' {
  if (batteryW > 20) return 'charging';
  if (batteryW < -20) return 'discharging';
  return 'idle';
}

export function deriveTelemetryEtaMinutes(raw: RawMetrics, batteryW: number): number {
  const charge = firstNumber(raw, 'params.chgRemainTime', 'param.chgRemainTime');
  const discharge = firstNumber(raw, 'params.dsgRemainTime', 'param.dsgRemainTime');
  const remain = firstNumber(raw, 'params.remainTime', 'param.remainTime');

  if (batteryW > 20 && charge !== undefined) {
    return clampNonNegativeInt(charge);
  }
  if (batteryW < -20 && discharge !== undefined) {
    return clampNonNegativeInt(discharge);
  }
  if (remain !== undefined) {
    return clampNonNegativeInt(remain);
  }
  if (charge !== undefined) return clampNonNegativeInt(charge);
  if (discharge !== undefined) return clampNonNegativeInt(discharge);
  return 0;
}

function deriveAcFromInputMinusPv(raw: RawMetrics, pv: number): number | undefined {
  const wattsIn = firstNumber(raw, 'params.wattsInSum', 'param.wattsInSum');
  if (wattsIn === undefined) return undefined;
  return Math.max(0, wattsIn - pv);
}

function deriveBattery(raw: RawMetrics): number | undefined {
  const batteryInput = firstNumber(raw, 'batteryW', 'params.bmsInputWatts', 'params.inputWatts');
  const batteryOutput = firstNumber(raw, 'params.bmsOutputWatts', 'params.outputWatts');
  if (batteryInput !== undefined || batteryOutput !== undefined) {
    return (batteryInput ?? 0) - (batteryOutput ?? 0);
  }

  const batAmp = firstNumber(raw, 'params.batAmp');
  const batVol = firstNumber(raw, 'params.batVol');
  if (batAmp !== undefined && batVol !== undefined) {
    return batAmp * batVol;
  }
  return undefined;
}

function deriveTemperature(raw: RawMetrics): number {
  const temps: number[] = [];
  const explicit = [
    'tempC',
    'params.temp',
    'params.pdTemp',
    'params.outTemp',
    'params.mpptLvTemp',
    'params.mpptHvTemp',
    'params.pcsAcTemp',
    'params.pcsDcTemp',
    'params.carTemp',
    'params.dcInTemp',
    'params.typec1Temp',
    'params.typec2Temp'
  ];
  for (const path of explicit) {
    const value = firstNumber(raw, path);
    if (value !== undefined && saneTemperature(value)) {
      temps.push(value);
    }
  }
  for (const prefix of ['params.cellTemp.', 'param.cellTemp.']) {
    for (const value of prefixedValues(raw, prefix)) {
      if (saneTemperature(value)) {
        temps.push(value);
      }
    }
  }
  return median(temps);
}

function firstNumber(raw: RawMetrics, ...paths: string[]): number | undefined {
  for (const path of paths) {
    const value = raw[path];
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
  }
  return undefined;
}

function sumIfPresent(raw: RawMetrics, ...paths: string[]): number | undefined {
  let found = false;
  let sum = 0;
  for (const path of paths) {
    const value = raw[path];
    if (typeof value === 'number' && Number.isFinite(value)) {
      sum += value;
      found = true;
    }
  }
  return found ? sum : undefined;
}

function sumIfPresentCapped(raw: RawMetrics, max: number, ...paths: string[]): number | undefined {
  let found = false;
  let sum = 0;
  for (const path of paths) {
    const value = raw[path];
    if (typeof value === 'number' && Number.isFinite(value) && Math.abs(value) <= max) {
      sum += value;
      found = true;
    }
  }
  return found ? sum : undefined;
}

function prefixedValues(raw: RawMetrics, prefix: string): number[] {
  const values: number[] = [];
  for (const [key, value] of Object.entries(raw)) {
    if (key.startsWith(prefix) && Number.isFinite(value)) {
      values.push(value);
    }
  }
  return values;
}

function saneTemperature(value: number): boolean {
  return value >= -80 && value <= 120;
}

function median(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 1) {
    return sorted[middle] ?? 0;
  }
  return ((sorted[middle - 1] ?? 0) + (sorted[middle] ?? 0)) / 2;
}

function clampNonNegativeInt(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.round(value));
}

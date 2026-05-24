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

const ANDERSON_POWER_NOISE_FLOOR_W = 0.5;
const BATTERY_VOLTAGE_MAX_CANONICAL = 1000;
const BATTERY_CURRENT_MAX_CANONICAL = 200;
const EXTERNAL_OUTPUT_LOAD_FIELDS = [
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
  'params.outAdsPwr',
  'params.invOutWatts',
  'params.carWatts',
  'params.wireWatts',
  'params.usb1Watts',
  'params.usb2Watts',
  'params.qcUsb1Watts',
  'params.qcUsb2Watts',
  'params.typec1Watts',
  'params.typec2Watts'
] as const;
const EXTRA_BATTERY_TRANSFER_FIELDS = [
  'params.XT150Watts1',
  'params.XT150Watts2',
  'param.XT150Watts1',
  'param.XT150Watts2'
] as const;
const EXTRA_BATTERY_PACK_POWER_PATTERN = /^params\.watts\.(\d+)\.curPower$/;

export function deriveTelemetryMetrics(raw: RawMetrics): DerivedTelemetryMetrics {
  const soc = firstNumber(
    raw,
    // Provider-specific SOC fields are the freshest source in merged live snapshots.
    'params.f32LcdShowSoc',
    'params.lcdShowSoc',
    'params.f32ShowSoc',
    'params.f32Soc',
    'param.cmsBattSoc',
    'params.cmsBattSoc',
    'params.soc',
    'param.soc',
    'soc',
    'params.bpPowerSoc'
  ) ?? 0;

  const directAcIn =
    sumIfPresent(raw, 'acW', 'params.inAcC20Pwr', 'params.inAc5p8Pwr') ??
    firstNumber(raw, 'params.invInWatts');
  const pv = derivePv(raw, directAcIn);

  const acIn =
    directAcIn ??
    deriveAcFromInputMinusPv(raw, pv) ??
    0;

  const dc = deriveDc(raw);

  const load = deriveLoad(raw);

  const powerBalance = acIn + pv - load;
  const battery = deriveBatteryPower(raw, powerBalance) ?? powerBalance;
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

function derivePv(raw: RawMetrics, directAcIn?: number): number {
  const pv =
    // Prefer DPU MPPT fields first (including explicit zero), then D2M per-port
    // fields, then top-level pvW fallback.
    sumIfPresentCapped(raw, 10000, 'params.inLvMpptPwr', 'params.inHvMpptPwr') ??
    sumIfPresentCapped(raw, 10000, 'param.powGetPvL', 'param.powGetPvH') ??
    sumIfPresentCapped(raw, 10000, 'params.pv1ChargeWatts', 'params.pv2ChargeWatts') ??
    firstNumberCapped(raw, 10000, 'pvW') ??
    0;

  return capPvByTotalInput(raw, pv, directAcIn);
}

function deriveDc(raw: RawMetrics): number {
  const explicit = firstNumber(raw, 'dcW');
  if (explicit !== undefined) {
    return explicit;
  }

  const base =
    sumIfPresent(
      raw,
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
      'params.outPrPwr'
    ) ?? 0;

  return base + (deriveAndersonPower(raw) ?? 0);
}

function deriveAndersonPower(raw: RawMetrics): number | undefined {
  const explicit = firstNumber(raw, 'params.outAdsPwr');
  const amp = firstNumber(raw, 'params.outAdsAmp');
  const vol = firstNumber(raw, 'params.outAdsVol');
  if (amp !== undefined && vol !== undefined) {
    const watts = Math.max(0, amp * vol);
    if (
      watts > ANDERSON_POWER_NOISE_FLOOR_W ||
      explicit === undefined ||
      explicit <= ANDERSON_POWER_NOISE_FLOOR_W
    ) {
      return watts;
    }
  }
  return explicit;
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

function capPvByTotalInput(raw: RawMetrics, pv: number, directAcIn?: number): number {
  if (!Number.isFinite(pv) || pv <= 0) {
    return Math.max(0, pv || 0);
  }

  const wattsIn = firstNumber(raw, 'params.wattsInSum', 'param.wattsInSum');
  if (wattsIn === undefined || !Number.isFinite(wattsIn) || wattsIn < 0) {
    return pv;
  }

  const explicitAc = directAcIn !== undefined && Number.isFinite(directAcIn)
    ? Math.max(0, directAcIn)
    : 0;
  const maxPossiblePv = Math.max(0, wattsIn - explicitAc);
  const tolerance = Math.max(2, wattsIn * 0.05);
  if (pv > maxPossiblePv + tolerance) {
    return maxPossiblePv;
  }
  return pv;
}

export function deriveTelemetryState(batteryW: number): 'charging' | 'discharging' | 'idle' {
  if (batteryW > 20) return 'charging';
  if (batteryW < -20) return 'discharging';
  return 'idle';
}

function deriveLoad(raw: RawMetrics): number {
  const explicit = firstNumber(raw, 'loadW');
  if (explicit !== undefined) {
    return explicit;
  }

  const aggregate = firstNumber(raw, 'params.wattsOutSum', 'param.wattsOutSum');
  const explicitOutputs = sumNonNegativeIfPresent(raw, ...EXTERNAL_OUTPUT_LOAD_FIELDS);
  const extraBatteryCharge = deriveExtraBatteryChargeTransfer(raw);
  if (aggregate !== undefined) {
    if (extraBatteryCharge <= 0) {
      return aggregate;
    }
    const adjustedAggregate = Math.max(0, aggregate - extraBatteryCharge);
    return Math.max(explicitOutputs ?? 0, adjustedAggregate);
  }

  return explicitOutputs ?? 0;
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

export function deriveBatteryPower(raw: RawMetrics, powerBalance?: number): number | undefined {
  const explicitBattery = firstNumber(raw, 'batteryW');
  if (explicitBattery !== undefined) {
    return explicitBattery;
  }

  const bmsInput = firstNumber(raw, 'params.bmsInputWatts');
  const bmsOutput = firstNumber(raw, 'params.bmsOutputWatts');
  const extraBatteryCharge = deriveExtraBatteryChargeTransfer(raw);
  if (extraBatteryCharge > 0 && !hasNonZero(bmsInput) && !hasNonZero(bmsOutput)) {
    return extraBatteryCharge - Math.max(0, firstNumber(raw, 'params.outputWatts', 'param.outputWatts') ?? 0);
  }
  if (
    powerBalance !== undefined &&
    Math.abs(powerBalance) > 20 &&
    (bmsInput !== undefined || bmsOutput !== undefined) &&
    !hasNonZero(bmsInput) &&
    !hasNonZero(bmsOutput)
  ) {
    return powerBalance;
  }
  if (bmsInput !== undefined || bmsOutput !== undefined) {
    return (bmsInput ?? 0) - (bmsOutput ?? 0);
  }

  const batteryInput = firstNumber(raw, 'params.inputWatts', 'param.inputWatts');
  const batteryOutput = firstNumber(raw, 'params.outputWatts', 'param.outputWatts');
  if (batteryInput !== undefined || batteryOutput !== undefined) {
    return (batteryInput ?? 0) - (batteryOutput ?? 0);
  }

  const batAmp = normalizePotentialMilliUnit(firstNumber(raw, 'params.batAmp'), BATTERY_CURRENT_MAX_CANONICAL);
  const batVol = normalizePotentialMilliUnit(firstNumber(raw, 'params.batVol'), BATTERY_VOLTAGE_MAX_CANONICAL);
  if (batAmp !== undefined && batVol !== undefined) {
    return batAmp * batVol;
  }
  return undefined;
}

function deriveExtraBatteryChargeTransfer(raw: RawMetrics): number {
  const xt150Charge = sumPositiveIfPresent(raw, ...EXTRA_BATTERY_TRANSFER_FIELDS) ?? 0;
  const kitCharge = sumExtraBatteryPackCharge(raw);
  const input = positiveNumber(firstNumber(raw, 'params.inputWatts', 'param.inputWatts')) ?? 0;
  const output = firstNumber(raw, 'params.outputWatts', 'param.outputWatts') ?? 0;
  const inputCharge = output <= 0 ? input : 0;

  if (xt150Charge <= 0 && kitCharge <= 0) {
    return 0;
  }
  return Math.max(xt150Charge, kitCharge, inputCharge);
}

function sumExtraBatteryPackCharge(raw: RawMetrics): number {
  let sum = 0;
  for (const [key, value] of Object.entries(raw)) {
    const matches = EXTRA_BATTERY_PACK_POWER_PATTERN.exec(key);
    if (!matches || !Number.isFinite(value) || value <= 0) {
      continue;
    }
    const packIndex = matches[1];
    const available = firstNumber(raw, `params.watts.${packIndex}.avaFlag`);
    if (available !== undefined && available <= 0) {
      continue;
    }
    sum += value;
  }
  return sum;
}

function normalizePotentialMilliUnit(value: number | undefined, maxAbsCanonical: number): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (Math.abs(value) > maxAbsCanonical && Math.abs(value / 1000) <= maxAbsCanonical) {
    return value / 1000;
  }
  return value;
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

function sumNonNegativeIfPresent(raw: RawMetrics, ...paths: string[]): number | undefined {
  let found = false;
  let sum = 0;
  for (const path of paths) {
    const value = raw[path];
    if (typeof value === 'number' && Number.isFinite(value)) {
      sum += Math.max(0, value);
      found = true;
    }
  }
  return found ? sum : undefined;
}

function sumPositiveIfPresent(raw: RawMetrics, ...paths: string[]): number | undefined {
  let found = false;
  let sum = 0;
  for (const path of paths) {
    const value = positiveNumber(raw[path]);
    if (value !== undefined) {
      sum += value;
      found = true;
    }
  }
  return found ? sum : undefined;
}

function positiveNumber(value: number | undefined): number | undefined {
  return value !== undefined && Number.isFinite(value) && value > 0 ? value : undefined;
}

function hasNonZero(value: number | undefined): boolean {
  return value !== undefined && Math.abs(value) > 0.5;
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

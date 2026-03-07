export type RawTelemetryMetrics = Record<string, number>;

export type DerivedTelemetryMetrics = {
  soc: number;
  pvW: number;
  loadW: number;
  batteryW: number;
  tempC: number;
  acW: number;
  dcW: number;
};

export type DerivedTelemetrySignals = {
  acOn?: boolean;
  dcOn?: boolean;
  usbOn?: boolean;
  dc12vOn?: boolean;
  evChargingOn?: boolean;
  fanOn?: boolean;
  solarChargingOn?: boolean;
  batteryHeatingOn?: boolean;
};

export type DerivedTelemetrySolarPort = {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
};

export type DerivedTelemetryDetail = {
  signals?: DerivedTelemetrySignals;
  solarPorts?: DerivedTelemetrySolarPort[];
};

const ANDERSON_POWER_NOISE_FLOOR_W = 0.5;
const APP_SHOW_FLAG_AC_ON_MASK = 0x4;
const APP_SHOW_FLAG_DC_ON_MASK = 0x2;

export function mergeRawMetrics(
  current: RawTelemetryMetrics,
  changed: Record<string, number>,
  cleared: string[]
): RawTelemetryMetrics {
  const next: RawTelemetryMetrics = { ...current };
  for (const [key, value] of Object.entries(changed)) {
    next[key] = value;
  }
  for (const key of cleared) {
    delete next[key];
  }
  return next;
}

export function deriveTelemetryMetrics(raw: RawTelemetryMetrics): DerivedTelemetryMetrics {
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

  const directAcIn =
    sumIfPresent(raw, 'acW', 'params.inAcC20Pwr', 'params.inAc5p8Pwr') ??
    firstNumber(raw, 'params.invInWatts');
  const pv = derivePv(raw, directAcIn);

  const acIn =
    directAcIn ??
    deriveAcFromInputMinusPv(raw, pv) ??
    0;

  const dc = deriveDc(raw);

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

  const net = acIn + pv - load;

  const battery = deriveBattery(raw) ?? net;
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

export function deriveTelemetryDetail(raw: RawTelemetryMetrics): DerivedTelemetryDetail | undefined {
  const solarPorts = deriveSolarPorts(raw);
  const signals = deriveSignals(raw, solarPorts);
  if (!signals && solarPorts.length === 0) {
    return undefined;
  }
  return {
    ...(signals ? { signals } : {}),
    ...(solarPorts.length > 0 ? { solarPorts } : {})
  };
}

function derivePv(raw: RawTelemetryMetrics, directAcIn?: number): number {
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

function deriveDc(raw: RawTelemetryMetrics): number {
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

function deriveSignals(
  raw: RawTelemetryMetrics,
  solarPorts: DerivedTelemetrySolarPort[]
): DerivedTelemetrySignals | undefined {
  const acOn = deriveAcOn(raw);
  const usbOn = deriveUsbOn(raw);
  const dc12vOn = deriveDc12vOn(raw);
  const explicitDcOn = deriveExplicitDcOn(raw);
  const dcOn = aggregateKnownBoolean(explicitDcOn, usbOn, dc12vOn);
  const evChargingOn = deriveEvChargingOn(raw);
  const fanOn = deriveFanOn(raw);
  const solarChargingOn = deriveSolarChargingOn(raw, solarPorts);
  const batteryHeatingOn = deriveBatteryHeatingOn(raw);

  const signals: DerivedTelemetrySignals = {};
  assignDefinedSignal(signals, 'acOn', acOn);
  assignDefinedSignal(signals, 'dcOn', dcOn);
  assignDefinedSignal(signals, 'usbOn', usbOn);
  assignDefinedSignal(signals, 'dc12vOn', dc12vOn);
  assignDefinedSignal(signals, 'evChargingOn', evChargingOn);
  assignDefinedSignal(signals, 'fanOn', fanOn);
  assignDefinedSignal(signals, 'solarChargingOn', solarChargingOn);
  assignDefinedSignal(signals, 'batteryHeatingOn', batteryHeatingOn);

  return Object.keys(signals).length > 0 ? signals : undefined;
}

function deriveAcOn(raw: RawTelemetryMetrics): boolean | undefined {
  if (hasAnyMetric(raw, 'params.cfgAcEnabled')) {
    return anyPositive(raw, 'params.cfgAcEnabled');
  }

  const showFlag = firstNumber(raw, 'params.showFlag');
  if (showFlag !== undefined) {
    return decodeAppShowCircuitFlags(showFlag).acOn;
  }

  if (
    hasAnyMetric(
      raw,
      'params.outAcTtVol',
      'params.outAcL14Vol',
      'params.outAcL11Vol',
      'params.outAcL12Vol',
      'params.outAcL21Vol',
      'params.outAcL22Vol',
      'params.outAc5p8Vol'
    )
  ) {
    return anyAbove(
      raw,
      50,
      'params.outAcTtVol',
      'params.outAcL14Vol',
      'params.outAcL11Vol',
      'params.outAcL12Vol',
      'params.outAcL21Vol',
      'params.outAcL22Vol',
      'params.outAc5p8Vol'
    );
  }

  if (
    hasAnyMetric(
      raw,
      'params.outAcL11Pwr',
      'params.outAcL12Pwr',
      'params.outAcL14Pwr',
      'params.outAcL21Pwr',
      'params.outAcL22Pwr',
      'params.outAcTtPwr',
      'params.outAc5p8Pwr',
      'params.invOutWatts'
    )
  ) {
    return anyPositive(
      raw,
      'params.outAcL11Pwr',
      'params.outAcL12Pwr',
      'params.outAcL14Pwr',
      'params.outAcL21Pwr',
      'params.outAcL22Pwr',
      'params.outAcTtPwr',
      'params.outAc5p8Pwr',
      'params.invOutWatts'
    );
  }

  return undefined;
}

function deriveExplicitDcOn(raw: RawTelemetryMetrics): boolean | undefined {
  const showFlag = firstNumber(raw, 'params.showFlag');
  if (showFlag !== undefined) {
    return decodeAppShowCircuitFlags(showFlag).dcOn;
  }
  if (hasAnyMetric(raw, 'params.dcOutState', 'params.carState')) {
    return anyPositive(raw, 'params.dcOutState', 'params.carState');
  }
  return undefined;
}

function deriveUsbOn(raw: RawTelemetryMetrics): boolean | undefined {
  if (
    hasAnyMetric(
      raw,
      'params.usb1Watts',
      'params.usb2Watts',
      'params.qcUsb1Watts',
      'params.qcUsb2Watts',
      'params.typec1Watts',
      'params.typec2Watts',
      'params.outUsb1Pwr',
      'params.outUsb2Pwr',
      'params.outTypec1Pwr',
      'params.outTypec2Pwr'
    )
  ) {
    return anyPositive(
      raw,
      'params.usb1Watts',
      'params.usb2Watts',
      'params.qcUsb1Watts',
      'params.qcUsb2Watts',
      'params.typec1Watts',
      'params.typec2Watts',
      'params.outUsb1Pwr',
      'params.outUsb2Pwr',
      'params.outTypec1Pwr',
      'params.outTypec2Pwr'
    );
  }
  return undefined;
}

function deriveDc12vOn(raw: RawTelemetryMetrics): boolean | undefined {
  const hasAnyDc12Hints =
    hasAnyMetric(raw, 'params.outAdsPwr', 'params.outAdsAmp', 'params.outAdsVol', 'params.wireWatts') ||
    hasAnyMetric(raw, 'params.dcOutState');
  if (!hasAnyDc12Hints) {
    return undefined;
  }

  const andersonPower = deriveAndersonPower(raw) ?? 0;
  return andersonPower > ANDERSON_POWER_NOISE_FLOOR_W || anyPositive(raw, 'params.wireWatts', 'params.dcOutState');
}

function deriveEvChargingOn(raw: RawTelemetryMetrics): boolean | undefined {
  if (hasAnyMetric(raw, 'params.evChgManualCtrl', 'params.plugInInfoAcpRunState')) {
    return anyPositive(raw, 'params.evChgManualCtrl', 'params.plugInInfoAcpRunState');
  }
  if (hasAnyMetric(raw, 'params.outPrPwr')) {
    return anyPositive(raw, 'params.outPrPwr');
  }
  return undefined;
}

function deriveFanOn(raw: RawTelemetryMetrics): boolean | undefined {
  if (hasAnyMetric(raw, 'params.fanState')) {
    return anyPositive(raw, 'params.fanState');
  }
  if (hasAnyMetric(raw, 'params.fanLevel')) {
    return anyPositive(raw, 'params.fanLevel');
  }
  return undefined;
}

function deriveSolarChargingOn(
  raw: RawTelemetryMetrics,
  solarPorts: DerivedTelemetrySolarPort[]
): boolean | undefined {
  const portStates = solarPorts
    .map((port) => {
      if (port.state === 'charging') return true;
      if (port.state !== undefined) return false;
      if ((port.watts ?? 0) > 1 || (port.amps ?? 0) > 0.03) return true;
      if (port.watts !== undefined || port.amps !== undefined || port.volts !== undefined) return false;
      return undefined;
    })
    .filter((value): value is boolean => value !== undefined);
  if (portStates.length > 0) {
    return aggregateKnownBoolean(...portStates);
  }
  if (
    hasAnyMetric(
      raw,
      'params.inLvMpptPwr',
      'params.inHvMpptPwr',
      'params.pv1ChargeWatts',
      'params.pv2ChargeWatts',
      'param.powGetPvL',
      'param.powGetPvH'
    )
  ) {
    return anyPositive(
      raw,
      'params.inLvMpptPwr',
      'params.inHvMpptPwr',
      'params.pv1ChargeWatts',
      'params.pv2ChargeWatts',
      'param.powGetPvL',
      'param.powGetPvH'
    );
  }
  return undefined;
}

function deriveBatteryHeatingOn(raw: RawTelemetryMetrics): boolean | undefined {
  const explicitStates = valuesBySuffix(raw, 'ptcMosState');
  if (explicitStates.length > 0) {
    return explicitStates.some((value) => value > 0);
  }

  const heatTimes = valuesBySuffix(raw, 'heatTime');
  if (heatTimes.length > 0) {
    return heatTimes.some((value) => value > 0);
  }

  return undefined;
}

function deriveSolarPorts(raw: RawTelemetryMetrics): DerivedTelemetrySolarPort[] {
  const d2mHints = hasAnyMetric(
    raw,
    'params.pv1ChargeWatts',
    'params.pv2ChargeWatts',
    'params.inVol',
    'params.inAmp',
    'params.pv2InVol',
    'params.pv2InAmp',
    'params.chgState',
    'params.pv2ChgState'
  );
  if (d2mHints) {
    return [deriveD2MSolarPortLow(raw), deriveD2MSolarPortHigh(raw)].filter(hasSolarPortData);
  }

  const dpuHints = hasAnyMetric(
    raw,
    'params.inLvMpptPwr',
    'params.inHvMpptPwr',
    'params.inLvMpptVol',
    'params.inLvMpptAmp',
    'params.inHvMpptVol',
    'params.inHvMpptAmp'
  );
  if (dpuHints) {
    return [deriveDpuSolarPortLow(raw), deriveDpuSolarPortHigh(raw)].filter(hasSolarPortData);
  }

  return [];
}

function deriveD2MSolarPortLow(raw: RawTelemetryMetrics): DerivedTelemetrySolarPort {
  const volts = normalizeMillivolts(firstNumber(raw, 'params.inVol', 'params.inLvMpptVol'), 60);
  const amps = normalizeMilliamps(firstNumber(raw, 'params.inAmp', 'params.inLvMpptAmp'), 15);
  const watts = sanitizeSolarWatts(
    firstDefined(
      firstNumber(raw, 'params.pv1ChargeWatts', 'params.outWatts', 'params.inLvMpptPwr'),
      multiplyNumbers(volts, amps)
    ),
    500
  );
  const rawState = firstNumber(raw, 'params.chgState');

  return {
    id: 'pv-1',
    name: 'PV 1',
    state: deriveSolarPortState(rawState, volts, watts, amps),
    volts,
    amps,
    watts: normalizeSolarPortWatts(watts, amps, rawState)
  };
}

function deriveD2MSolarPortHigh(raw: RawTelemetryMetrics): DerivedTelemetrySolarPort {
  const volts = normalizeMillivolts(firstNumber(raw, 'params.pv2InVol', 'params.inHvMpptVol'), 60);
  const amps = normalizeMilliamps(firstNumber(raw, 'params.pv2InAmp', 'params.inHvMpptAmp'), 15);
  const watts = sanitizeSolarWatts(
    firstDefined(
      firstNumber(raw, 'params.pv2ChargeWatts', 'params.pv2InWatts', 'params.inHvMpptPwr'),
      multiplyNumbers(volts, amps)
    ),
    500
  );
  const rawState = firstNumber(raw, 'params.pv2ChgState');

  return {
    id: 'pv-2',
    name: 'PV 2',
    state: deriveSolarPortState(rawState, volts, watts, amps),
    volts,
    amps,
    watts: normalizeSolarPortWatts(watts, amps, rawState)
  };
}

function deriveDpuSolarPortLow(raw: RawTelemetryMetrics): DerivedTelemetrySolarPort {
  const volts = firstNumber(raw, 'params.inLvMpptVol');
  const amps = firstNumber(raw, 'params.inLvMpptAmp');
  const watts = sanitizeSolarWatts(
    firstDefined(firstNumber(raw, 'params.inLvMpptPwr'), multiplyNumbers(volts, amps)),
    1600
  );

  return {
    id: 'pv-low',
    name: 'PV Low',
    state: deriveSolarPortState(undefined, volts, watts, amps),
    volts,
    amps,
    watts
  };
}

function deriveDpuSolarPortHigh(raw: RawTelemetryMetrics): DerivedTelemetrySolarPort {
  const volts = firstNumber(raw, 'params.inHvMpptVol');
  const amps = firstNumber(raw, 'params.inHvMpptAmp');
  const watts = sanitizeSolarWatts(
    firstDefined(firstNumber(raw, 'params.inHvMpptPwr'), multiplyNumbers(volts, amps)),
    4000
  );

  return {
    id: 'pv-high',
    name: 'PV High',
    state: deriveSolarPortState(undefined, volts, watts, amps),
    volts,
    amps,
    watts
  };
}

function hasSolarPortData(port: DerivedTelemetrySolarPort): boolean {
  return (
    port.state !== undefined ||
    port.volts !== undefined ||
    port.amps !== undefined ||
    port.watts !== undefined
  );
}

function deriveSolarPortState(
  rawState: number | undefined,
  volts: number | undefined,
  watts: number | undefined,
  amps: number | undefined
): string | undefined {
  if (rawState === undefined && volts === undefined && watts === undefined && amps === undefined) {
    return undefined;
  }
  if (volts !== undefined && volts <= 0.1) {
    return 'inactive';
  }
  if ((amps ?? 0) <= 0.03 && (watts ?? 0) <= 1) {
    if (rawState !== undefined) {
      if (rawState === 1) {
        return 'locked';
      }
      if (rawState >= 2) {
        return 'charging';
      }
    }
    return volts !== undefined ? 'idle' : 'unknown';
  }
  if ((watts ?? 0) > 1) {
    return 'charging';
  }
  if (rawState !== undefined) {
    if (rawState >= 2) {
      return 'charging';
    }
    if (rawState === 1) {
      return 'locked';
    }
  }
  return 'idle';
}

function normalizeSolarPortWatts(
  watts: number | undefined,
  amps: number | undefined,
  rawState: number | undefined
): number | undefined {
  if (watts === undefined) {
    return undefined;
  }
  if (amps !== undefined && amps <= 0.03 && rawState !== undefined && rawState < 2) {
    return 0;
  }
  return watts;
}

function decodeAppShowCircuitFlags(showFlag: number): { acOn: boolean; dcOn: boolean } {
  return {
    acOn: (Math.trunc(showFlag) & APP_SHOW_FLAG_AC_ON_MASK) !== 0,
    dcOn: (Math.trunc(showFlag) & APP_SHOW_FLAG_DC_ON_MASK) !== 0
  };
}

function assignDefinedSignal(
  target: DerivedTelemetrySignals,
  key: keyof DerivedTelemetrySignals,
  value: boolean | undefined
): void {
  if (value !== undefined) {
    target[key] = value;
  }
}

function aggregateKnownBoolean(...values: Array<boolean | undefined>): boolean | undefined {
  if (values.some((value) => value === true)) {
    return true;
  }
  if (values.some((value) => value !== undefined)) {
    return false;
  }
  return undefined;
}

function deriveAndersonPower(raw: RawTelemetryMetrics): number | undefined {
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

function firstNumberCapped(raw: RawTelemetryMetrics, maxAbs: number, ...keys: string[]): number | undefined {
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

function capPvByTotalInput(raw: RawTelemetryMetrics, pv: number, directAcIn?: number): number {
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

function deriveAcFromInputMinusPv(raw: RawTelemetryMetrics, pv: number): number | undefined {
  const wattsIn = firstNumber(raw, 'params.wattsInSum', 'param.wattsInSum');
  if (wattsIn === undefined) {
    return undefined;
  }
  return Math.max(0, wattsIn - pv);
}

function deriveBattery(raw: RawTelemetryMetrics): number | undefined {
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

function deriveTemperature(raw: RawTelemetryMetrics): number {
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

function firstNumber(raw: RawTelemetryMetrics, ...paths: string[]): number | undefined {
  for (const path of paths) {
    const value = raw[path];
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
  }
  return undefined;
}

function hasAnyMetric(raw: RawTelemetryMetrics, ...paths: string[]): boolean {
  return paths.some((path) => firstNumber(raw, path) !== undefined);
}

function anyPositive(raw: RawTelemetryMetrics, ...paths: string[]): boolean {
  return paths.some((path) => (firstNumber(raw, path) ?? 0) > 0);
}

function anyAbove(raw: RawTelemetryMetrics, threshold: number, ...paths: string[]): boolean {
  return paths.some((path) => (firstNumber(raw, path) ?? Number.NEGATIVE_INFINITY) >= threshold);
}

function sumIfPresent(raw: RawTelemetryMetrics, ...paths: string[]): number | undefined {
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

function sumIfPresentCapped(raw: RawTelemetryMetrics, max: number, ...paths: string[]): number | undefined {
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

function firstDefined<T>(...values: Array<T | undefined>): T | undefined {
  return values.find((value) => value !== undefined);
}

function multiplyNumbers(left: number | undefined, right: number | undefined): number | undefined {
  if (left === undefined || right === undefined) {
    return undefined;
  }
  return left * right;
}

function normalizeMillivolts(value: number | undefined, maxVolts: number): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value > maxVolts * 2 ? value / 1000 : value;
}

function normalizeMilliamps(value: number | undefined, maxAmps: number): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value > maxAmps * 2 ? value / 1000 : value;
}

function sanitizeSolarWatts(value: number | undefined, maxWatts: number): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value > maxWatts * 2 ? undefined : value;
}

function prefixedValues(raw: RawTelemetryMetrics, prefix: string): number[] {
  const values: number[] = [];
  for (const [key, value] of Object.entries(raw)) {
    if (key.startsWith(prefix) && Number.isFinite(value)) {
      values.push(value);
    }
  }
  return values;
}

function valuesBySuffix(raw: RawTelemetryMetrics, suffix: string): number[] {
  const values: number[] = [];
  for (const [key, value] of Object.entries(raw)) {
    if (key === suffix || key.endsWith(`.${suffix}`)) {
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

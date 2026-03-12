import type { ProviderDevice } from '../grpc/controlPlaneClient.js';

export type DeviceCapabilities = Record<string, unknown>;

export type BatteryPackDetail = {
  id: string;
  socPct?: number;
  powerW?: number;
  tempC?: number;
  heatingOn?: boolean;
  energyWh?: number;
  remainMinutes?: number;
  socMinPct?: number;
  socMaxPct?: number;
};

export type SolarPortDetail = {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
  maxVolts?: number;
  maxAmps?: number;
  maxWatts?: number;
};

export type DeviceTelemetryDetails = {
  bpCount?: number;
  packs?: BatteryPackDetail[];
  solarPorts?: SolarPortDetail[];
  overallSocPct?: number;
  socWindowMinPct?: number;
  socWindowMaxPct?: number;
  backupReservePct?: number;
  acOn?: boolean;
  dcOn?: boolean;
  usbOn?: boolean;
  dc12vOn?: boolean;
  evChargingOn?: boolean;
  fanOn?: boolean;
  solarChargingOn?: boolean;
  batteryHeatingOn?: boolean;
  stormGuardActive?: boolean;
  stormGuardEndsAtUnixMs?: number;
};

export type ProviderDevicePresentation = {
  serialNumber: string;
  capabilities?: DeviceCapabilities;
  details?: DeviceTelemetryDetails;
};

type GenericRecord = Record<string, unknown>;

export function buildProviderDevicePresentation(device: ProviderDevice): ProviderDevicePresentation {
  const serialNumber = device.canonicalSn || device.providerDeviceId;
  const capabilities = buildCapabilities(device);
  const details = buildDetails(device, capabilities);
  return {
    serialNumber,
    capabilities: Object.keys(capabilities).length > 0 ? capabilities : undefined,
    details: hasDetailValues(details) ? details : undefined
  };
}

function buildCapabilities(device: ProviderDevice): DeviceCapabilities {
  const raw = device.capabilities ?? {};
  const modelLower = device.model.toLowerCase();
  const batteryPacks = toPositiveNumber(raw.battery_pack_count) ?? deriveBatteryPacksFromMetadata(device.metadata);
  const pvInputCount = toPositiveNumber(raw.pv_input_count) ?? derivePVInputCountFromMetadata(device.metadata);

  const out: DeviceCapabilities = {
    ...raw
  };
  if (batteryPacks !== undefined) {
    out.batteryPacks = batteryPacks;
  }
  if (pvInputCount !== undefined) {
    out.pvInputCount = pvInputCount;
  }

  const batteryCapacityKWh = deriveBatteryCapacityKWh(modelLower, batteryPacks);
  if (batteryCapacityKWh !== undefined) {
    out.batteryCapacityKWh = batteryCapacityKWh;
  }

  // UI aliases. Keep them conservative so a field match in quota metadata does not
  // overclaim support for a product that does not expose the feature to users.
  if (modelLower.includes('delta pro ultra')) {
    out.evCharging = true;
    out.batteryHeating = true;
    out.preconditioning = true;
  }

  if (toBoolean(raw.supports_ac_output)) {
    out.acOutput = true;
  }
  if (toBoolean(raw.supports_dc_output)) {
    out.dcOutput = true;
  }
  if (toBoolean(raw.supports_usb_output)) {
    out.usbOutput = true;
  }
  if (toBoolean(raw.supports_parallel)) {
    out.parallel = true;
  }
  if (toBoolean(raw.supports_extra_battery)) {
    out.extraBattery = true;
  }

  return out;
}

function buildDetails(device: ProviderDevice, capabilities: DeviceCapabilities): DeviceTelemetryDetails {
  const modelLower = device.model.toLowerCase();
  const metadata = device.metadata ?? {};
  const groups = asRecord(metadata.groups);
  const baseStormGuard = deriveStormGuardDetails(groups);
  const details: DeviceTelemetryDetails = {
    bpCount:
      toPositiveNumber(capabilities.batteryPacks) ??
      toPositiveNumber(device.capabilities?.battery_pack_count) ??
      deriveBatteryPacksFromMetadata(metadata),
    ...baseStormGuard
  };

  if (modelLower.includes('delta pro ultra')) {
    const dpu = buildDpuDetails(groups, details.bpCount);
    return { ...details, ...dpu };
  }

  if (modelLower.includes('delta 2 max')) {
    const d2m = buildD2mDetails(groups, details.bpCount);
    return { ...details, ...d2m };
  }

  if (modelLower.includes('delta 2')) {
    const d2 = buildDelta2Details(groups, details.bpCount);
    return { ...details, ...d2 };
  }

  return details;
}

function buildDpuDetails(groups: GenericRecord, bpCount?: number): DeviceTelemetryDetails {
  const appshow = asRecord(groups.hs_yj751_pd_appshow_addr);
  const backend = asRecord(groups.hs_yj751_pd_backend_addr);
  const appset = asRecord(groups.hs_yj751_pd_app_set_info_addr);
  const bpAddr = asRecord(groups.hs_yj751_pd_bp_addr);
  const bpInfo = asArray(bpAddr.bpInfo);

  const packs: BatteryPackDetail[] = [];
  for (const [idx, row] of bpInfo.entries()) {
    const pack = asRecord(row);
    if (Object.keys(pack).length === 0) {
      continue;
    }
    packs.push({
      id: stringOr(pack.bpNo, `bp${idx + 1}`),
      socPct: toNumber(pack.bpSoc),
      powerW: toNumber(pack.bpPwr),
      tempC: toNumber(pack.bpTemp),
      heatingOn: toNumber(pack.heatTime) !== undefined && (toNumber(pack.heatTime) ?? 0) > 0,
      energyWh: toNumber(pack.bpEnergy),
      remainMinutes: toNumber(pack.remainTime),
      socMinPct: toNumber(pack.bpSocMin),
      socMaxPct: toNumber(pack.bpSocMax)
    });
  }

  const lowWatts = firstDefined(toNumber(appshow.inLvMpptPwr), multiplyNumbers(backend.inLvMpptVol, backend.inLvMpptAmp));
  const highWatts = firstDefined(toNumber(appshow.inHvMpptPwr), multiplyNumbers(backend.inHvMpptVol, backend.inHvMpptAmp));
  const lowPort = makeSolarPort({
    id: 'pv-low',
    name: 'PV Low',
    volts: toNumber(backend.inLvMpptVol),
    amps: toNumber(backend.inLvMpptAmp),
    watts: lowWatts,
    maxWatts: 1600,
    maxVolts: 150,
    maxAmps: 15
  });
  const highPort = makeSolarPort({
    id: 'pv-high',
    name: 'PV High',
    volts: toNumber(backend.inHvMpptVol),
    amps: toNumber(backend.inHvMpptAmp),
    watts: highWatts,
    maxWatts: 4000,
    maxVolts: 450,
    maxAmps: 15
  });

  const usbOn =
    anyPositive(appshow.outUsb1Pwr, appshow.outUsb2Pwr, appshow.outTypec1Pwr, appshow.outTypec2Pwr);
  const dc12vOn = anyPositive(appshow.outAdsPwr);
  const fanOn = truthyNumber(backend.fanState);
  const batteryHeatingOn =
    packs.some((pack) => pack.heatingOn === true) ||
    truthyNumber(appset.bmsModeSet) ||
    truthyNumber(appset.batteryHeatMode);
  const solarChargingOn = [lowPort, highPort].some((port) => port.state === 'charging');

  return {
    bpCount: bpCount ?? packs.length,
    packs,
    solarPorts: [lowPort, highPort],
    socWindowMinPct: toNumber(appset.dsgMinSoc),
    socWindowMaxPct: toNumber(appset.chgMaxSoc),
    backupReservePct: firstDefined(toNumber(appset.sysBackupSoc), toNumber(appset.backupRatio)),
    acOn: anyPositive(
      appshow.outAcTtPwr,
      appshow.outAcL11Pwr,
      appshow.outAcL12Pwr,
      appshow.outAcL21Pwr,
      appshow.outAcL22Pwr,
      appshow.outAcL14Pwr,
      appshow.outAc_5p8Pwr
    ),
    dcOn: dc12vOn,
    usbOn,
    dc12vOn,
    evChargingOn: anyPositive(appshow.outPrPwr, backend.outPrPwr),
    fanOn,
    solarChargingOn,
    batteryHeatingOn
  };
}

function buildD2mDetails(groups: GenericRecord, bpCount?: number): DeviceTelemetryDetails {
  const pd = asRecord(groups.pd);
  const inv = asRecord(groups.inv);
  const mppt = asRecord(groups.mppt);
  const bmsStatus = asRecord(groups.bms_bmsStatus);
  const bmsEmsStatus = asRecord(groups.bms_emsStatus);
  const bmsKitInfo = asRecord(groups.bms_kitInfo);

  const packs: BatteryPackDetail[] = [];
  const mainSoc = firstDefined(toNumber(bmsStatus.targetSoc), toNumber(pd.soc));
  const overallSocPct = firstDefined(
    toNumber(bmsEmsStatus.f32LcdShowSoc),
    toNumber(bmsEmsStatus.lcdShowSoc),
    toNumber(pd.bpPowerSoc),
    toNumber(pd.soc),
    toNumber(bmsStatus.targetSoc)
  );
  const mainPower = deriveBatteryNetPower(bmsStatus);
  const mainTemp = firstDefined(toNumber(bmsStatus.temp), toNumber(bmsStatus.cellTemp), toNumber(bmsStatus.maxCellTemp));
  if (mainSoc !== undefined || mainPower !== undefined || mainTemp !== undefined) {
    packs.push({
      id: 'main',
      socPct: mainSoc,
      powerW: mainPower,
      tempC: mainTemp,
      heatingOn: false,
      energyWh: toNumber(bmsStatus.fullCap),
      remainMinutes: firstDefined(toNumber(bmsStatus.remainTime), toNumber(pd.remainTime)),
      socMinPct: firstDefined(toNumber(bmsStatus.minSoc), toNumber(bmsStatus.socMin), toNumber(pd.minAcSoc), toNumber(bmsEmsStatus.minDsgSoc)),
      socMaxPct: firstDefined(toNumber(bmsStatus.maxSoc), toNumber(bmsStatus.socMax), toNumber(bmsEmsStatus.maxChargeSoc))
    });
  }
  for (const entry of asArray(bmsKitInfo.watts)) {
    const pack = asRecord(entry);
    if (toNumber(pack.avaFlag) === 0) {
      continue;
    }
    packs.push({
      id: stringOr(pack.sn, `bp${packs.length + 1}`),
      socPct: firstDefined(toNumber(pack.soc), toNumber(pack.targetSoc)),
      powerW: firstDefined(toNumber(pack.curPower), deriveBatteryNetPower(pack)),
      tempC: firstDefined(toNumber(pack.temp), toNumber(pack.cellTemp)),
      heatingOn: false,
      energyWh: toNumber(pack.energy),
      remainMinutes: toNumber(pack.remainTime),
      socMinPct: firstDefined(toNumber(pack.socMin), toNumber(pack.minSoc)),
      socMaxPct: firstDefined(toNumber(pack.socMax), toNumber(pack.maxSoc))
    });
  }

  const pv1Volts = normalizeMillivolts(mppt.inVol, 60);
  const pv1Amps = normalizeMilliamps(mppt.inAmp, 15);
  const pv2Volts = normalizeMillivolts(mppt.pv2InVol, 60);
  const pv2Amps = normalizeMilliamps(mppt.pv2InAmp, 15);
  const pv1Watts = sanitizeSolarWatts(firstDefined(toNumber(mppt.outWatts), multiplyNumbers(pv1Volts, pv1Amps)), 500);
  const pv2Watts = sanitizeSolarWatts(firstDefined(toNumber(pd.pv2ChargeWatts), toNumber(mppt.pv2InWatts), multiplyNumbers(pv2Volts, pv2Amps)), 500);
  const pv1 = makeSolarPort({
    id: 'pv-1',
    name: 'PV 1',
    volts: pv1Volts,
    amps: pv1Amps,
    watts: pv1Watts,
    maxWatts: 500,
    maxVolts: 60,
    maxAmps: 15,
    rawState: toNumber(mppt.chgState)
  });
  const pv2 = makeSolarPort({
    id: 'pv-2',
    name: 'PV 2',
    volts: pv2Volts,
    amps: pv2Amps,
    watts: pv2Watts,
    maxWatts: 500,
    maxVolts: 60,
    maxAmps: 15,
    rawState: toNumber(mppt.pv2ChgState)
  });

  const usbOn = anyPositive(pd.typec1Watts, pd.typec2Watts, pd.usb1Watts, pd.usb2Watts, pd.qcUsb1Watts, pd.qcUsb2Watts);
  const dc12vOn = truthyNumber(pd.dcOutState) || anyPositive(pd.wireWatts);
  const solarChargingOn = [pv1, pv2].some((port) => port.state === 'charging');

  return {
    bpCount: bpCount ?? packs.length,
    packs,
    solarPorts: [pv1, pv2],
    overallSocPct,
    socWindowMinPct: firstDefined(toNumber(bmsEmsStatus.minDsgSoc), toNumber(pd.minAcSoc), toNumber(bmsStatus.minSoc), toNumber(bmsKitInfo.minSoc)),
    socWindowMaxPct: firstDefined(toNumber(bmsEmsStatus.maxChargeSoc), toNumber(bmsStatus.maxSoc), toNumber(bmsKitInfo.maxSoc)),
    backupReservePct: firstDefined(toNumber(pd.minAcSoc), toNumber(bmsEmsStatus.minOpenOilEb)),
    acOn: truthyNumber(inv.cfgAcEnabled) || anyPositive(inv.outputWatts),
    dcOn: dc12vOn || truthyNumber(pd.carState),
    usbOn,
    dc12vOn,
    evChargingOn: false,
    fanOn: truthyNumber(inv.fanState),
    solarChargingOn,
    batteryHeatingOn: false
  };
}

function buildDelta2Details(groups: GenericRecord, bpCount?: number): DeviceTelemetryDetails {
  const pd = asRecord(groups.pd);
  const inv = asRecord(groups.inv);
  const mppt = asRecord(groups.mppt);
  const ems = asRecord(groups.ems);
  const bmsMaster = asRecord(groups.bmsMaster);
  const bmsSlave1 = asRecord(groups.bmsSlave1);

  const packs: BatteryPackDetail[] = [];
  const mainSoc = firstDefined(toNumber(bmsMaster.soc), toNumber(pd.soc));
  const mainPower = deriveBatteryNetPower(bmsMaster);
  const mainTemp = firstDefined(
    toNumber(bmsMaster.temp),
    toNumber(bmsMaster.maxCellTemp),
    toNumber(bmsMaster.minCellTemp)
  );
  if (mainSoc !== undefined || mainPower !== undefined || mainTemp !== undefined) {
    packs.push({
      id: 'main',
      socPct: mainSoc,
      powerW: mainPower,
      tempC: mainTemp,
      heatingOn: false,
      energyWh: toNumber(bmsMaster.fullCap),
      remainMinutes: firstDefined(toNumber(bmsMaster.remainTime), toNumber(pd.remainTime)),
      socMinPct: firstDefined(
        toNumber(ems.minDsgSoc),
        toNumber(pd.bpPowerSoc),
        toNumber(pd.minAcSoc),
        toNumber(pd.minAcoutSoc)
      ),
      socMaxPct: firstDefined(toNumber(ems.maxChargeSoc))
    });
  }

  const extraSoc = toNumber(bmsSlave1.soc);
  const extraPower = deriveBatteryNetPower(bmsSlave1);
  const extraTemp = firstDefined(
    toNumber(bmsSlave1.temp),
    toNumber(bmsSlave1.maxCellTemp),
    toNumber(bmsSlave1.minCellTemp)
  );
  if (extraSoc !== undefined || extraPower !== undefined || extraTemp !== undefined) {
    packs.push({
      id: 'bp1',
      socPct: extraSoc,
      powerW: extraPower,
      tempC: extraTemp,
      heatingOn: false,
      energyWh: toNumber(bmsSlave1.fullCap),
      remainMinutes: toNumber(bmsSlave1.remainTime),
      socMinPct: firstDefined(
        toNumber(ems.minDsgSoc),
        toNumber(pd.bpPowerSoc),
        toNumber(pd.minAcSoc),
        toNumber(pd.minAcoutSoc)
      ),
      socMaxPct: firstDefined(toNumber(ems.maxChargeSoc))
    });
  }

  const pv1Volts = normalizeMillivolts(mppt.inVol, 60);
  const pv1Amps = normalizeMilliamps(mppt.inAmp, 15);
  const pv1Watts = sanitizeSolarWatts(
    firstDefined(toNumber(mppt.outWatts), toNumber(mppt.inWatts), multiplyNumbers(pv1Volts, pv1Amps)),
    500
  );
  const pv1 = makeSolarPort({
    id: 'pv-1',
    name: 'PV 1',
    volts: pv1Volts,
    amps: pv1Amps,
    watts: pv1Watts,
    maxWatts: 500,
    maxVolts: 60,
    maxAmps: 15,
    rawState: toNumber(mppt.chgState)
  });

  const usbOn = anyPositive(
    pd.typec1Watts,
    pd.typec2Watts,
    pd.usb1Watts,
    pd.usb2Watts,
    pd.qcUsb1Watts,
    pd.qcUsb2Watts
  );
  const dc12vOn = truthyNumber(pd.dcOutState) || anyPositive(pd.wireWatts, pd.carWatts);
  const solarChargingOn = pv1.state === 'charging';

  return {
    bpCount: bpCount ?? Math.max(1, packs.length),
    packs,
    solarPorts: [pv1],
    overallSocPct: firstDefined(
      toNumber(ems.f32LcdShowSoc),
      toNumber(ems.lcdShowSoc),
      toNumber(pd.soc),
      toNumber(bmsMaster.soc)
    ),
    socWindowMinPct: firstDefined(
      toNumber(ems.minDsgSoc),
      toNumber(pd.bpPowerSoc),
      toNumber(pd.minAcSoc),
      toNumber(pd.minAcoutSoc)
    ),
    socWindowMaxPct: firstDefined(toNumber(ems.maxChargeSoc)),
    backupReservePct: firstDefined(
      toNumber(pd.bpPowerSoc),
      toNumber(ems.minOpenOilEb),
      toNumber(pd.minAcSoc),
      toNumber(pd.minAcoutSoc)
    ),
    acOn: truthyNumber(inv.cfgAcEnabled) || truthyNumber(pd.acEnabled) || anyPositive(inv.outputWatts, pd.outputWatts),
    dcOn: dc12vOn || truthyNumber(pd.carState),
    usbOn,
    dc12vOn,
    evChargingOn: false,
    fanOn: truthyNumber(inv.fanState) || truthyNumber(ems.fanLevel),
    solarChargingOn,
    batteryHeatingOn: false
  };
}

function makeSolarPort(input: {
  id: string;
  name: string;
  volts?: number;
  amps?: number;
  watts?: number;
  maxWatts: number;
  maxVolts: number;
  maxAmps: number;
  rawState?: number;
}): SolarPortDetail {
  const watts = normalizeSolarPortWatts(input.watts, input.amps, input.rawState);
  return {
    id: input.id,
    name: input.name,
    state: deriveSolarState(input.rawState, input.volts, watts, input.amps),
    volts: input.volts,
    amps: input.amps,
    watts,
    maxWatts: input.maxWatts,
    maxVolts: input.maxVolts,
    maxAmps: input.maxAmps
  };
}

function deriveSolarState(
  rawState: number | undefined,
  volts: number | undefined,
  watts: number | undefined,
  amps: number | undefined
): string {
  if ((volts ?? 0) <= 0.1) {
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
    return 'idle';
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
  if ((amps ?? 0) <= 0.03 && rawState !== undefined && rawState < 2) {
    return 0;
  }
  return watts;
}

function deriveBatteryCapacityKWh(modelLower: string, batteryPacks?: number): number | undefined {
  if (modelLower.includes('delta pro ultra')) {
    return 6.0 * Math.max(1, batteryPacks ?? 2);
  }
  if (modelLower.includes('delta 2 max')) {
    const packs = Math.max(1, batteryPacks ?? 2);
    return packs * 2.048;
  }
  if (modelLower.includes('delta 2')) {
    const packs = Math.max(1, batteryPacks ?? 1);
    return packs * 1.024;
  }
  return undefined;
}

function deriveBatteryPacksFromMetadata(metadata?: GenericRecord): number | undefined {
  const groups = asRecord(metadata?.groups);
  const bpInfo = asArray(asRecord(groups.hs_yj751_pd_bp_addr).bpInfo);
  if (bpInfo.length > 0) {
    return bpInfo.length;
  }
  const d2mKit = asRecord(groups.bms_kitInfo);
  const derived = firstDefined(
    toPositiveNumber(d2mKit.kitNum),
    toPositiveNumber(asRecord(metadata?.capabilities).battery_pack_count)
  );
  if (derived !== undefined) {
    return derived;
  }
  if (Object.keys(asRecord(groups.bmsMaster)).length > 0) {
    return 1;
  }
  return undefined;
}

function derivePVInputCountFromMetadata(metadata?: GenericRecord): number | undefined {
  const groups = asRecord(metadata?.groups);
  const dpu = asRecord(groups.hs_yj751_pd_backend_addr);
  if (Object.keys(dpu).length > 0) {
    return 2;
  }
  const mppt = asRecord(groups.mppt);
  if (Object.keys(mppt).length > 0) {
    return toNumber(mppt.pv2InVol) !== undefined || toNumber(mppt.pv2InAmp) !== undefined ? 2 : 1;
  }
  return undefined;
}

function deriveBatteryNetPower(record: GenericRecord): number | undefined {
  const input = toNumber(record.inputWatts);
  const output = toNumber(record.outputWatts);
  if (input === undefined && output === undefined) {
    return undefined;
  }
  return (input ?? 0) - (output ?? 0);
}

function hasDetailValues(details: DeviceTelemetryDetails): boolean {
  return Object.values(details).some((value) => {
    if (value === undefined) {
      return false;
    }
    if (Array.isArray(value)) {
      return value.length > 0;
    }
    return true;
  });
}

function asRecord(value: unknown): GenericRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as GenericRecord) : {};
}

function asArray(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }
  if (value && typeof value === 'object') {
    const wrapped = (value as GenericRecord).values;
    if (Array.isArray(wrapped)) {
      return wrapped;
    }
  }
  return [];
}

function toNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return undefined;
}

function toPositiveNumber(value: unknown): number | undefined {
  const parsed = toNumber(value);
  return parsed !== undefined && parsed > 0 ? parsed : undefined;
}

function toBoolean(value: unknown): boolean {
  return value === true || value === 1 || value === '1';
}

function toBooleanFlag(value: unknown): boolean | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  return toBoolean(value);
}

function truthyNumber(value: unknown): boolean {
  const parsed = toNumber(value);
  return parsed !== undefined && parsed > 0;
}

function anyPositive(...values: unknown[]): boolean {
  return values.some((value) => truthyNumber(value));
}

function multiplyNumbers(left: unknown, right: unknown): number | undefined {
  const l = toNumber(left);
  const r = toNumber(right);
  if (l === undefined || r === undefined) {
    return undefined;
  }
  return l * r;
}

function normalizeMillivolts(value: unknown, maxVolts: number): number | undefined {
  const parsed = toNumber(value);
  if (parsed === undefined) {
    return undefined;
  }
  return parsed > maxVolts*2 ? parsed / 1000 : parsed;
}

function normalizeMilliamps(value: unknown, maxAmps: number): number | undefined {
  const parsed = toNumber(value);
  if (parsed === undefined) {
    return undefined;
  }
  return parsed > maxAmps*2 ? parsed / 1000 : parsed;
}

function sanitizeSolarWatts(value: number | undefined, maxWatts: number): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value > maxWatts * 2 ? undefined : value;
}

function deriveStormGuardDetails(groups: GenericRecord): Pick<DeviceTelemetryDetails, 'stormGuardActive' | 'stormGuardEndsAtUnixMs'> {
  const groupRecords = Object.values(groups).map((value) => asRecord(value));
  // EcoFlow currently exposes Storm Guard with at least two field families across
  // supported products. Scan all normalized metadata groups so new supported
  // models do not require another model-specific mapper patch.
  const stormPatternEnable = firstDefined(
    ...groupRecords.map((group) => toBooleanFlag(group.stormPatternEnable)),
    ...groupRecords.map((group) => toBooleanFlag(group.stormIsEnable))
  );
  const stormPatternOpen = firstDefined(
    ...groupRecords.map((group) => toBooleanFlag(group.stormPatternOpenFlag)),
    ...groupRecords.map((group) => toBooleanFlag(group.inStormMode))
  );
  const stormPatternEndTimeSeconds = firstDefined(
    ...groupRecords.map((group) => toPositiveNumber(group.stormPatternEndTime)),
    ...groupRecords.map((group) => toPositiveNumber(group.stormEndTimestamp))
  );

  return {
    stormGuardActive: stormPatternEnable === true && stormPatternOpen === true,
    stormGuardEndsAtUnixMs:
      stormPatternEndTimeSeconds !== undefined ? stormPatternEndTimeSeconds * 1000 : undefined
  };
}

function firstDefined<T>(...values: (T | undefined)[]): T | undefined {
  return values.find((value) => value !== undefined);
}

function stringOr(value: unknown, fallback: string): string {
  return typeof value === 'string' && value.trim() !== '' ? value : fallback;
}

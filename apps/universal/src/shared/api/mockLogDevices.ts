import { env } from '@/shared/config/env';
import * as Linking from 'expo-linking';
import { Platform } from 'react-native';
import { type MockDevice, mockDevices } from '@/features/devices/mockData';

type MutableDevice = MockDevice & {
  updatedAtMs: number;
};

const REFRESH_MS = 1_500;
const CSV_POLL_MS = 2_000;
const LOG_POLL_MS = 10_000;
const SOLAR_TODAY_UPDATE_MS = 10_000;
const SOLAR_GENERATED_SERIES_POINTS = 72; // 6:00 -> 18:00 in 10-minute slots
const SOLAR_GENERATED_START_HOUR = 6;
const SOLAR_GENERATED_END_HOUR = 18;
const CSV_TAIL_BYTES = 256 * 1024;
const LOG_TAIL_BYTES = 192 * 1024;
const MAX_PARSED_LINES = 3_000;
const MAX_PARSED_CSV_LINES = 4_000;
const SERIAL_CONTEXT_TTL_LINES = 240;
const SERIAL_FROM_TOPIC = /\/([A-Z0-9]{8,})\/quota/;
const SERIAL_HINT = /\b(?:device_sn|sn)=([A-Z0-9]{8,})\b/;
const BPINFO_QUOTA_PREFIX = 'quota_raw[hs_yj751_pd_bp_addr.bpInfo]=';
const SESSION_START = /\bsession_start\b/i;
const ENERGY_SOC = /soc=([0-9]+(?:\.[0-9]+)?)%/;
const ENERGY_IN = /in=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_AC_IN = /in_ac=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_DC_OUT = /out_dc=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_OUT = /out=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_PV = /in_pv=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_NET = /net=([-+]?[0-9]+(?:\.[0-9]+)?)W/;
const ENERGY_REMAIN_LEGACY = /remain=([0-9]+)/;
const ENERGY_REMAIN_TYPED = /remain=(charging|discharging|active):\s*([0-9]+)min/i;
const ENERGY_STATE = /state=(charging|discharging|idle)/i;
const ETA_TYPED = /(charging|discharging|active):\s*([0-9]+)min/i;
const ISO_PREFIX_TS = /^(\d{4}-\d{2}-\d{2}T[^\s]+)/;
const VALID_SERIAL = /^[A-Z0-9]{8,}$/i;

let cachedAt = 0;
let cachedDevices: MutableDevice[] = mockDevices.map((d) => ({
  ...d,
  updatedAtMs: Date.now()
}));
let cachedCsvParsed: MockDevice[] = [];
let cachedLogParsed: MockDevice[] = [];
let lastCsvSig = '';
let lastLogSig = '';
let lastCsvPollAt = 0;
let lastLogPollAt = 0;
let preferredCsvUrl: string | null = null;
let preferredLogUrl: string | null = null;
let cachedCsvHeaderLine = '';
let csvBootstrapDone = false;
let logBootstrapDone = false;
let solarTodayDayKey = '';
let solarTodayLastUpdateAt = 0;
let solarTodayInitialized = false;
let solarTodayWhBySerial = new Map<string, number>();
let solarTodayLastPointBySerial = new Map<string, { ts: number; watts: number }>();
let solarGeneratedBinsWhBySerial = new Map<string, number[]>();

function cloneCachedDevices(): MockDevice[] {
  const nowMs = Date.now();
  return cachedDevices.map((device) => ({
    id: device.id,
    serialNumber: device.serialNumber,
    name: device.name,
    model: device.model,
    online: device.online,
    batteryPct: device.batteryPct,
    state: device.state,
    etaMinutes: device.etaMinutes,
    pvW: device.pvW,
    acInW: device.acInW,
    dcW: device.dcW,
    loadW: device.loadW,
    netW: device.netW,
    solarTodayWh: getSolarTodayWh(device.serialNumber, nowMs) ?? device.solarTodayWh,
    solarGeneratedSeriesWh:
      getSolarGeneratedSeriesWh(device.serialNumber, nowMs) ?? device.solarGeneratedSeriesWh,
    tempC: device.tempC,
    telemetryTsMs: device.telemetryTsMs,
    capabilities: device.capabilities,
    details: device.details
  }));
}

function textSig(text: string | null): string {
  if (!text) return '';
  const len = text.length;
  const tail = text.slice(Math.max(0, len - 512));
  return `${len}:${tail}`;
}

function isValidSerial(serial: string | undefined | null): boolean {
  if (!serial) return false;
  const s = serial.trim();
  if (!VALID_SERIAL.test(s)) return false;
  if (/^0+$/.test(s)) return false;
  return true;
}

function chooseDefined<T>(next: T | undefined, prev: T | undefined): T | undefined {
  return next !== undefined ? next : prev;
}

function mergeDetails(
  prev: MockDevice['details'] | undefined,
  next: MockDevice['details'] | undefined
): MockDevice['details'] | undefined {
  if (!prev) return next;
  if (!next) return prev;
  return {
    bpCount: chooseDefined(next.bpCount, prev.bpCount),
    packs: next.packs && next.packs.length > 0 ? next.packs : prev.packs,
    solarPorts:
      next.solarPorts && next.solarPorts.length > 0 ? next.solarPorts : prev.solarPorts,
    estimateMode: chooseDefined(next.estimateMode, prev.estimateMode),
    estimateSource: chooseDefined(next.estimateSource, prev.estimateSource),
    estimateEtaMin: chooseDefined(next.estimateEtaMin, prev.estimateEtaMin),
    remainChargeMin: chooseDefined(next.remainChargeMin, prev.remainChargeMin),
    remainDischargeMin: chooseDefined(next.remainDischargeMin, prev.remainDischargeMin),
    remainGlobalMin: chooseDefined(next.remainGlobalMin, prev.remainGlobalMin),
    mpptLowState: chooseDefined(next.mpptLowState, prev.mpptLowState),
    mpptHighState: chooseDefined(next.mpptHighState, prev.mpptHighState),
    acOn: chooseDefined(next.acOn, prev.acOn),
    dcOn: chooseDefined(next.dcOn, prev.dcOn),
    usbOn: chooseDefined(next.usbOn, prev.usbOn),
    dc12vOn: chooseDefined(next.dc12vOn, prev.dc12vOn),
    evChargingOn: chooseDefined(next.evChargingOn, prev.evChargingOn),
    fanOn: chooseDefined(next.fanOn, prev.fanOn),
    solarChargingOn: chooseDefined(next.solarChargingOn, prev.solarChargingOn),
    batteryHeatingOn: chooseDefined(next.batteryHeatingOn, prev.batteryHeatingOn),
    mqttQueueDepth: chooseDefined(next.mqttQueueDepth, prev.mqttQueueDepth),
    mqttQueueDroppedOldest: chooseDefined(
      next.mqttQueueDroppedOldest,
      prev.mqttQueueDroppedOldest
    )
  };
}

function mergeWithLastKnown(prev: MutableDevice | undefined, next: MockDevice): MutableDevice {
  if (!prev) {
    return { ...next, updatedAtMs: Date.now() };
  }
  return {
    ...prev,
    ...next,
    name: next.name || prev.name,
    model: next.model || prev.model,
    online: next.online ?? prev.online,
    batteryPct: Number.isFinite(next.batteryPct) ? next.batteryPct : prev.batteryPct,
    state: next.state || prev.state,
    etaMinutes: Number.isFinite(next.etaMinutes) ? next.etaMinutes : prev.etaMinutes,
    pvW: Number.isFinite(next.pvW as number) ? next.pvW : prev.pvW,
    acInW: Number.isFinite(next.acInW as number) ? next.acInW : prev.acInW,
    dcW: Number.isFinite(next.dcW as number) ? next.dcW : prev.dcW,
    loadW: Number.isFinite(next.loadW as number) ? next.loadW : prev.loadW,
    netW: Number.isFinite(next.netW as number) ? next.netW : prev.netW,
    solarTodayWh: Number.isFinite(next.solarTodayWh as number)
      ? next.solarTodayWh
      : prev.solarTodayWh,
    solarGeneratedSeriesWh:
      next.solarGeneratedSeriesWh && next.solarGeneratedSeriesWh.length > 0
        ? next.solarGeneratedSeriesWh
        : prev.solarGeneratedSeriesWh,
    tempC: Number.isFinite(next.tempC as number) ? next.tempC : prev.tempC,
    telemetryTsMs: chooseDefined(next.telemetryTsMs, prev.telemetryTsMs),
    capabilities: chooseDefined(next.capabilities, prev.capabilities),
    details: mergeDetails(prev.details, next.details),
    updatedAtMs: Date.now()
  };
}

function nativeMockCandidates(pathSuffix: string): string[] {
  if (Platform.OS === 'web') return [];
  const candidates: string[] = [];
  try {
    const created = Linking.createURL('/');
    const parsed = new URL(created);
    const host = parsed.hostname || '127.0.0.1';
    const port = parsed.port || '8081';
    candidates.push(`http://${host}:${port}${pathSuffix}`);
  } catch {
    // ignore
  }
  candidates.push(`http://127.0.0.1:8081${pathSuffix}`);
  candidates.push(`http://localhost:8081${pathSuffix}`);
  return candidates;
}

function parseJsonFromLine(line: string): unknown | null {
  const marker = 'payload_raw=';
  const idx = line.indexOf(marker);
  if (idx === -1) return null;
  const raw = line.slice(idx + marker.length).trim();
  if (!raw.startsWith('{') || !raw.endsWith('}')) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

function parseBpInfoFromQuotaLine(line: string): any[] | null {
  const idx = line.indexOf(BPINFO_QUOTA_PREFIX);
  if (idx === -1) return null;
  const raw = line.slice(idx + BPINFO_QUOTA_PREFIX.length).trim();
  if (!raw.startsWith('[') || !raw.endsWith(']')) return null;
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function clampPercent(value: number): number {
  return Math.max(0, Math.min(100, value));
}

function parseLineTimestampMs(line: string): number | null {
  const iso = line.match(ISO_PREFIX_TS)?.[1];
  if (!iso) return null;
  const ts = Date.parse(iso);
  return Number.isFinite(ts) ? ts : null;
}

function parseStateFromPower(inW: number | null, outW: number | null): MockDevice['state'] | null {
  if (inW === null || outW === null) return null;
  if (inW > outW + 5) return 'charging';
  if (outW > inW + 5) return 'discharging';
  return 'idle';
}

function getOrCreateDevice(
  serial: string,
  map: Map<string, MutableDevice>
): MutableDevice {
  const existing = map.get(serial);
  if (existing) return existing;

  const created: MutableDevice = {
    id: serial,
    serialNumber: serial,
    name: `EcoFlow ${serial.slice(-6)}`,
    model: 'EcoFlow Device',
    online: true,
    batteryPct: 0,
    state: 'idle',
    etaMinutes: 0,
    pvW: 0,
    acInW: 0,
    dcW: 0,
    loadW: 0,
    netW: 0,
    tempC: 0,
    telemetryTsMs: Date.now(),
    updatedAtMs: Date.now(),
    details: {
      packs: [],
      solarPorts: []
    }
  };
  map.set(serial, created);
  return created;
}

function updateFromBpInfo(
  bpInfo: any[],
  serial: string | null,
  map: Map<string, MutableDevice>,
  nowMs: number
): void {
  if (!serial || !Array.isArray(bpInfo) || bpInfo.length === 0) return;
  const device = getOrCreateDevice(serial, map);
  const prevPacks = device.details?.packs ?? [];
  const prevById = new Map(prevPacks.map((p) => [p.id, p] as const));
  const nextPacks = bpInfo
    .map((bp: any) => {
      const idx = Number(bp?.bpNo);
      if (!Number.isFinite(idx) || idx <= 0) return null;
      const id = `bp${Math.round(idx)}`;
      const prev = prevById.get(id);
      const socPct = typeof bp?.bpSoc === 'number' ? clampPercent(bp.bpSoc) : prev?.socPct;
      const powerW = typeof bp?.bpPwr === 'number' ? bp.bpPwr : prev?.powerW;
      const tempC = typeof bp?.bpTemp === 'number' ? bp.bpTemp : prev?.tempC;
      const heatingOn =
        typeof bp?.heatTime === 'number'
          ? bp.heatTime > 0
          : prev?.heatingOn;
      return {
        id,
        socPct,
        powerW,
        tempC,
        heatingOn
      };
    })
    .filter((v): v is NonNullable<typeof v> => v !== null);

  if (nextPacks.length === 0) return;
  const batteryHeatingOn = nextPacks.some((p) => p.heatingOn === true);
  const prevDetails = device.details ?? {};
  device.details = {
    ...prevDetails,
    bpCount: prevDetails.bpCount ?? nextPacks.length,
    packs: nextPacks,
    batteryHeatingOn
  };
  device.telemetryTsMs = nowMs;
  device.updatedAtMs = nowMs;
}

function updateFromPayload(
  payload: any,
  serial: string | null,
  map: Map<string, MutableDevice>,
  nowMs: number
): void {
  if (!serial) return;
  const device = getOrCreateDevice(serial, map);

  if (payload?.cmdId === 21 && typeof payload?.param?.cmsBattSoc === 'number') {
    device.batteryPct = clampPercent(payload.param.cmsBattSoc);
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }

  if (
    payload?.cmdId === 1 &&
    payload?.addr === 'hs_yj751_pd_appshow_addr' &&
    payload?.params
  ) {
    const soc = payload.params.soc;
    const remain = payload.params.remainTime;
    if (typeof soc === 'number') {
      device.batteryPct = clampPercent(soc);
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
    if (typeof remain === 'number' && Number.isFinite(remain)) {
      device.etaMinutes = Math.max(0, Math.round(remain));
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
  }

  if (payload?.moduleType === 2 && payload?.typeCode === 'kitInfo') {
    const watts = Array.isArray(payload?.params?.watts) ? payload.params.watts : [];
    const primary = watts.find((w: any) => typeof w?.f32Soc === 'number' || typeof w?.soc === 'number');
    if (primary) {
      const soc = typeof primary.f32Soc === 'number' ? primary.f32Soc : primary.soc;
      if (typeof soc === 'number') {
        device.batteryPct = clampPercent(soc);
        device.telemetryTsMs = nowMs;
        device.updatedAtMs = nowMs;
      }
    }
  }

  if (payload?.typeCode === 'emsStatus' && payload?.params) {
    const chgRemain = payload.params.chgRemainTime;
    const dsgRemain = payload.params.dsgRemainTime;
    if (typeof chgRemain === 'number' && Number.isFinite(chgRemain) && chgRemain >= 0) {
      device.state = 'charging';
      device.etaMinutes = Math.round(chgRemain);
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
    if (typeof dsgRemain === 'number' && Number.isFinite(dsgRemain) && dsgRemain >= 0) {
      device.state = 'discharging';
      device.etaMinutes = Math.round(dsgRemain);
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
  }

  if (payload?.addr === 'hs_yj751_pd_bp_addr' && Array.isArray(payload?.param?.bpInfo)) {
    updateFromBpInfo(payload.param.bpInfo, serial, map, nowMs);
  }
}

function updateFromEnergySummary(
  line: string,
  serial: string | null,
  map: Map<string, MutableDevice>,
  nowMs: number
): void {
  if (!serial || !line.includes('energy_summary')) return;
  const device = getOrCreateDevice(serial, map);

  const socMatch = line.match(ENERGY_SOC);
  if (socMatch?.[1]) {
    const soc = Number(socMatch[1]);
    if (Number.isFinite(soc)) {
      device.batteryPct = clampPercent(soc);
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
  }

  const typedRemainMatch = line.match(ENERGY_REMAIN_TYPED) ?? line.match(ETA_TYPED);
  if (typedRemainMatch?.[2]) {
    const typedState = typedRemainMatch[1]?.toLowerCase();
    const remain = Number(typedRemainMatch[2]);
    if (Number.isFinite(remain)) {
      device.etaMinutes = Math.max(0, Math.round(remain));
      if (typedState === 'charging' || typedState === 'discharging') {
        device.state = typedState;
      }
      device.telemetryTsMs = nowMs;
      device.updatedAtMs = nowMs;
    }
  } else {
    const remainMatch = line.match(ENERGY_REMAIN_LEGACY);
    if (remainMatch?.[1]) {
      const remain = Number(remainMatch[1]);
      if (Number.isFinite(remain)) {
        device.etaMinutes = Math.max(0, Math.round(remain));
        device.telemetryTsMs = nowMs;
        device.updatedAtMs = nowMs;
      }
    }
  }

  const energyState = line.match(ENERGY_STATE)?.[1]?.toLowerCase();
  if (energyState === 'charging' || energyState === 'discharging' || energyState === 'idle') {
    device.state = energyState;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }

  const inMatch = line.match(ENERGY_IN);
  const acInMatch = line.match(ENERGY_AC_IN);
  const outMatch = line.match(ENERGY_OUT);
  const dcOutMatch = line.match(ENERGY_DC_OUT);
  const pvMatch = line.match(ENERGY_PV);
  const netMatch = line.match(ENERGY_NET);
  const inW = inMatch?.[1] ? Number(inMatch[1]) : null;
  const acInW = acInMatch?.[1] ? Number(acInMatch[1]) : null;
  const outW = outMatch?.[1] ? Number(outMatch[1]) : null;
  const dcOutW = dcOutMatch?.[1] ? Number(dcOutMatch[1]) : null;
  const pvW = pvMatch?.[1] ? Number(pvMatch[1]) : null;
  const netW = netMatch?.[1] ? Number(netMatch[1]) : null;

  if (Number.isFinite(pvW as number)) {
    device.pvW = pvW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }
  if (Number.isFinite(outW as number)) {
    device.loadW = outW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }
  if (Number.isFinite(dcOutW as number)) {
    device.dcW = dcOutW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }
  if (Number.isFinite(netW as number)) {
    device.netW = netW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }
  if (Number.isFinite(acInW as number)) {
    device.acInW = acInW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  } else if (Number.isFinite(inW as number)) {
    device.acInW = inW as number;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }

  if (!Number.isFinite(netW as number) && Number.isFinite(inW as number) && Number.isFinite(outW as number)) {
    device.netW = (inW as number) - (outW as number);
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }

  const derivedState = parseStateFromPower(
    Number.isFinite(inW as number) ? (inW as number) : null,
    Number.isFinite(outW as number) ? (outW as number) : null
  );
  if (derivedState) {
    device.state = derivedState;
    device.telemetryTsMs = nowMs;
    device.updatedAtMs = nowMs;
  }
}

function parseDevicesFromLog(logText: string): MockDevice[] {
  const map = new Map<string, MutableDevice>(
    mockDevices.map((d) => [
      d.serialNumber,
      {
        ...d,
        updatedAtMs: Date.now()
      }
    ])
  );
  let activeSerial: string | null = null;
  const sessionSerials = new Set<string>();
  const nowMs = Date.now();
  const lines = logText.split('\n');
  const start = Math.max(0, lines.length - MAX_PARSED_LINES);
  let lastSerialAnchorLine = -1;

  for (let i = start; i < lines.length; i += 1) {
    const line = lines[i] ?? '';
    if (!line) continue;

    if (SESSION_START.test(line)) {
      // New emitter session boundary; avoid carrying serial context across sessions.
      activeSerial = null;
      lastSerialAnchorLine = -1;
      sessionSerials.clear();
    }

    const serialHint = line.match(SERIAL_HINT)?.[1];
    if (serialHint && isValidSerial(serialHint)) {
      activeSerial = serialHint;
      lastSerialAnchorLine = i;
      sessionSerials.add(serialHint);
      getOrCreateDevice(activeSerial, map).online = true;
    }

    const topicMatch = line.match(SERIAL_FROM_TOPIC);
    if (topicMatch?.[1] && isValidSerial(topicMatch[1])) {
      activeSerial = topicMatch[1];
      lastSerialAnchorLine = i;
      sessionSerials.add(topicMatch[1]);
      getOrCreateDevice(activeSerial, map).online = true;
    }

    const hasReliableSerialContext =
      activeSerial !== null &&
      lastSerialAnchorLine >= 0 &&
      i - lastSerialAnchorLine <= SERIAL_CONTEXT_TTL_LINES &&
      sessionSerials.size === 1;

    if (!hasReliableSerialContext) {
      continue;
    }

    const lineTsMs = parseLineTimestampMs(line) ?? nowMs;
    updateFromEnergySummary(line, activeSerial, map, lineTsMs);
    const payload = parseJsonFromLine(line);
    if (payload) {
      updateFromPayload(payload, activeSerial, map, lineTsMs);
    }
    const bpInfoQuota = parseBpInfoFromQuotaLine(line);
    if (bpInfoQuota) {
      updateFromBpInfo(bpInfoQuota, activeSerial, map, lineTsMs);
    }
  }

  const baseOrder = new Map(mockDevices.map((d, i) => [d.serialNumber, i]));
  return Array.from(map.values())
    .sort((a, b) => (baseOrder.get(a.serialNumber) ?? 9999) - (baseOrder.get(b.serialNumber) ?? 9999))
    .map((device) => ({
      id: device.id,
      serialNumber: device.serialNumber,
      name: device.name,
      model: device.model,
      online: device.online,
      batteryPct: device.batteryPct,
      state: device.state,
      etaMinutes: device.etaMinutes,
      pvW: device.pvW,
      acInW: device.acInW,
      dcW: device.dcW,
      loadW: device.loadW,
      netW: device.netW,
      solarTodayWh: getSolarTodayWh(device.serialNumber, Date.now()),
      solarGeneratedSeriesWh: getSolarGeneratedSeriesWh(device.serialNumber, Date.now()),
      tempC: device.tempC,
      telemetryTsMs: device.telemetryTsMs,
      capabilities: device.capabilities,
      details: device.details
    }));
}

function parseNumber(value: string | undefined): number | null {
  if (!value || value === 'n/a') return null;
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}

function dayKeyLocal(ms: number): string {
  const d = new Date(ms);
  const y = d.getFullYear();
  const m = `${d.getMonth() + 1}`.padStart(2, '0');
  const day = `${d.getDate()}`.padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function dayBoundsLocal(ms: number): { startMs: number; endMs: number } {
  const d = new Date(ms);
  const start = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  return {
    startMs: start,
    endMs: start + 24 * 60 * 60 * 1000
  };
}

function solarWindowBoundsLocal(ms: number): { startMs: number; endMs: number } {
  const d = new Date(ms);
  const start = new Date(
    d.getFullYear(),
    d.getMonth(),
    d.getDate(),
    SOLAR_GENERATED_START_HOUR,
    0,
    0,
    0
  ).getTime();
  const end = new Date(
    d.getFullYear(),
    d.getMonth(),
    d.getDate(),
    SOLAR_GENERATED_END_HOUR,
    0,
    0,
    0
  ).getTime();
  return { startMs: start, endMs: end };
}

function resetSolarTodayState(dayKey: string): void {
  solarTodayDayKey = dayKey;
  solarTodayWhBySerial = new Map<string, number>();
  solarTodayLastPointBySerial = new Map<string, { ts: number; watts: number }>();
  solarGeneratedBinsWhBySerial = new Map<string, number[]>();
  solarTodayInitialized = false;
}

function ensureSolarBins(serial: string): number[] {
  const existing = solarGeneratedBinsWhBySerial.get(serial);
  if (existing) return existing;
  const created = Array.from({ length: SOLAR_GENERATED_SERIES_POINTS }, () => 0);
  solarGeneratedBinsWhBySerial.set(serial, created);
  return created;
}

function addEnergyToSolarBins(
  bins: number[],
  fromMs: number,
  toMs: number,
  watts: number,
  windowStartMs: number,
  windowEndMs: number
): void {
  const clippedFrom = Math.max(fromMs, windowStartMs);
  const clippedTo = Math.min(toMs, windowEndMs);
  if (clippedTo <= clippedFrom) return;
  const slotMs = (windowEndMs - windowStartMs) / SOLAR_GENERATED_SERIES_POINTS;
  if (!(slotMs > 0)) return;

  let cursor = clippedFrom;
  while (cursor < clippedTo) {
    const idx = Math.min(
      SOLAR_GENERATED_SERIES_POINTS - 1,
      Math.max(0, Math.floor((cursor - windowStartMs) / slotMs))
    );
    const slotEnd = Math.min(clippedTo, windowStartMs + (idx + 1) * slotMs);
    if (slotEnd <= cursor) break;
    const dtHours = (slotEnd - cursor) / 3_600_000;
    bins[idx] = (bins[idx] ?? 0) + Math.max(0, watts) * dtHours;
    cursor = slotEnd;
  }
}

function addSolarTodaySample(
  serial: string,
  tsMs: number,
  watts: number,
  dayStartMs: number,
  dayEndMs: number,
  solarWindowStartMs: number,
  solarWindowEndMs: number
): void {
  const prev = solarTodayLastPointBySerial.get(serial);
  if (prev && tsMs <= prev.ts) {
    return;
  }

  if (prev) {
    const from = Math.max(prev.ts, dayStartMs);
    const to = Math.min(tsMs, dayEndMs);
    if (to > from) {
      const dtHours = (to - from) / 3_600_000;
      const avgWatts = Math.max(0, (prev.watts + watts) / 2);
      const currentWh = solarTodayWhBySerial.get(serial) ?? 0;
      solarTodayWhBySerial.set(serial, currentWh + avgWatts * dtHours);
      const bins = ensureSolarBins(serial);
      addEnergyToSolarBins(
        bins,
        prev.ts,
        tsMs,
        avgWatts,
        solarWindowStartMs,
        solarWindowEndMs
      );
    }
  }

  solarTodayLastPointBySerial.set(serial, { ts: tsMs, watts });
}

function updateSolarTodayFromCsv(
  csvText: string,
  {
    nowMs,
    fullRebuild
  }: {
    nowMs: number;
    fullRebuild: boolean;
  }
): void {
  const lines = csvText.split('\n').filter(Boolean);
  if (lines.length < 2) return;

  const headers = lines[0]?.split(',') ?? [];
  const idx = Object.fromEntries(headers.map((h, i) => [h, i])) as Record<string, number>;
  if (idx.device_sn === undefined || idx.ts_unix_ms === undefined) return;

  const currentDayKey = dayKeyLocal(nowMs);
  const { startMs: dayStartMs, endMs: dayEndMs } = dayBoundsLocal(nowMs);
  const { startMs: solarWindowStartMs, endMs: solarWindowEndMs } = solarWindowBoundsLocal(nowMs);
  if (fullRebuild || !solarTodayInitialized || solarTodayDayKey !== currentDayKey) {
    solarTodayWhBySerial = new Map<string, number>();
    solarTodayLastPointBySerial = new Map<string, { ts: number; watts: number }>();
    solarGeneratedBinsWhBySerial = new Map<string, number[]>();
    solarTodayDayKey = currentDayKey;
    solarTodayInitialized = true;
  }

  const col = (cols: string[], name: string): string | undefined => {
    const i = idx[name];
    if (i === undefined) return undefined;
    return cols[i];
  };

  for (let i = 1; i < lines.length; i += 1) {
    const line = lines[i];
    if (!line) continue;
    const cols = line.split(',');
    if (cols.length < headers.length) continue;

    const serial = col(cols, 'device_sn') ?? '';
    if (!isValidSerial(serial)) continue;

    const ts = parseNumber(col(cols, 'ts_unix_ms'));
    if (ts === null) continue;
    const tsMs = Math.round(ts);
    if (tsMs < dayStartMs || tsMs >= dayEndMs) continue;

    const solarIn = parseNumber(col(cols, 'solar_in_w'));
    const solarLowIn = parseNumber(col(cols, 'solar_low_in_w')) ?? 0;
    const solarHighIn = parseNumber(col(cols, 'solar_high_in_w')) ?? 0;
    const watts = Math.max(0, solarIn ?? (solarLowIn + solarHighIn));
    addSolarTodaySample(
      serial,
      tsMs,
      watts,
      dayStartMs,
      dayEndMs,
      solarWindowStartMs,
      solarWindowEndMs
    );
  }

  solarTodayLastUpdateAt = nowMs;
}

function getSolarTodayWh(serial: string, nowMs: number): number | undefined {
  const currentDayKey = dayKeyLocal(nowMs);
  if (solarTodayDayKey !== currentDayKey) return undefined;
  const base = solarTodayWhBySerial.get(serial) ?? 0;
  const last = solarTodayLastPointBySerial.get(serial);
  if (!last) return base;
  const { endMs } = dayBoundsLocal(nowMs);
  const to = Math.min(nowMs, endMs);
  if (to <= last.ts) return base;
  const dtHours = (to - last.ts) / 3_600_000;
  const extra = Math.max(0, last.watts) * dtHours;
  return base + extra;
}

function getSolarGeneratedSeriesWh(
  serial: string,
  nowMs: number
): number[] | undefined {
  const currentDayKey = dayKeyLocal(nowMs);
  if (solarTodayDayKey !== currentDayKey) return undefined;

  const baseBins = solarGeneratedBinsWhBySerial.get(serial);
  const bins = baseBins
    ? [...baseBins]
    : Array.from({ length: SOLAR_GENERATED_SERIES_POINTS }, () => 0);

  const last = solarTodayLastPointBySerial.get(serial);
  const { startMs: solarWindowStartMs, endMs: solarWindowEndMs } = solarWindowBoundsLocal(nowMs);
  if (last && nowMs > last.ts) {
    const extrapolatedTo = Math.min(nowMs, last.ts + 15 * 60_000, solarWindowEndMs);
    if (extrapolatedTo > last.ts) {
      addEnergyToSolarBins(
        bins,
        last.ts,
        extrapolatedTo,
        Math.max(0, last.watts),
        solarWindowStartMs,
        solarWindowEndMs
      );
    }
  }

  return bins.map((value) => Math.max(0, value));
}

function parseBoolLike(value: string | undefined): boolean | undefined {
  const n = parseNumber(value);
  if (n === null) return undefined;
  return n > 0;
}

function getPvPortCaps(
  modelName: string
): { low: { maxWatts: number; maxVolts: number; maxAmps: number }; high: { maxWatts: number; maxVolts: number; maxAmps: number } } {
  const model = modelName.toLowerCase();
  if (model.includes('delta pro ultra')) {
    return {
      low: { maxWatts: 1600, maxVolts: 150, maxAmps: 15 },
      high: { maxWatts: 4000, maxVolts: 450, maxAmps: 15 }
    };
  }
  if (model.includes('delta 2 max')) {
    return {
      low: { maxWatts: 500, maxVolts: 60, maxAmps: 15 },
      high: { maxWatts: 500, maxVolts: 60, maxAmps: 15 }
    };
  }
  return {
    low: { maxWatts: 0, maxVolts: 0, maxAmps: 0 },
    high: { maxWatts: 0, maxVolts: 0, maxAmps: 0 }
  };
}

function median(values: Array<number | null>): number | null {
  const nums = values.filter((v): v is number => v !== null && Number.isFinite(v));
  if (!nums.length) return null;
  nums.sort((a, b) => a - b);
  const mid = Math.floor(nums.length / 2);
  if (nums.length % 2 === 1) return nums[mid] ?? null;
  const left = nums[mid - 1];
  const right = nums[mid];
  if (left === undefined || right === undefined) return null;
  return (left + right) / 2;
}

function parseDevicesFromTrainingCsv(csvText: string): MockDevice[] {
  const lines = csvText.split('\n').filter(Boolean);
  if (lines.length < 2) return [];
  const headers = lines[0]?.split(',') ?? [];
  const idx = Object.fromEntries(headers.map((h, i) => [h, i])) as Record<string, number>;

  if (idx.device_sn === undefined) {
    return [];
  }

  const map = new Map<string, MutableDevice>(
    mockDevices.map((d) => [d.serialNumber, { ...d, updatedAtMs: Date.now() }])
  );
  const col = (cols: string[], name: string): string | undefined => {
    const i = idx[name];
    if (i === undefined) return undefined;
    return cols[i];
  };

  const start = Math.max(1, lines.length - MAX_PARSED_CSV_LINES);
  const nowMs = Date.now();

  for (let i = start; i < lines.length; i += 1) {
    const line = lines[i];
    if (!line) continue;
    const cols = line.split(',');
    if (cols.length < headers.length) continue;
    const serial = col(cols, 'device_sn') ?? '';
    if (!isValidSerial(serial)) continue;

    const device = getOrCreateDevice(serial, map);
    device.online = true;
    const tsMs = parseNumber(col(cols, 'ts_unix_ms'));
    if (tsMs !== null) {
      device.telemetryTsMs = Math.round(tsMs);
    }

    const productName = col(cols, 'product_name') ?? '';
    if (productName) {
      device.model = productName;
    }

    const modelName = (productName || device.model || '').toLowerCase();
    const socBlended = parseNumber(col(cols, 'soc_pct'));
    const socBp1 = parseNumber(col(cols, 'bp1_soc'));
    // D2M commonly reports the user-facing SOC in bp1_soc (main-unit pack),
    // while soc_pct can be weighted across packs and appear lower during asymmetry.
    // Prefer bp1_soc for D2M card SOC to match device/app display expectations.
    const soc =
      modelName.includes('delta 2 max') && socBp1 !== null
        ? socBp1
        : socBlended;
    if (soc !== null) device.batteryPct = clampPercent(soc);

    const systemState = (col(cols, 'system_state') ?? '').toLowerCase();
    if (systemState === 'charging' || systemState === 'discharging' || systemState === 'idle') {
      device.state = systemState;
    }

    const solarIn = parseNumber(col(cols, 'solar_in_w')) ?? 0;
    const solarLowIn = parseNumber(col(cols, 'solar_low_in_w')) ?? 0;
    const solarHighIn = parseNumber(col(cols, 'solar_high_in_w')) ?? 0;
    const acIn = parseNumber(col(cols, 'ac_in_w')) ?? 0;
    const dcIn = parseNumber(col(cols, 'dc_in_w')) ?? 0;
    const acOut = parseNumber(col(cols, 'ac_out_w')) ?? 0;
    const dcOut = parseNumber(col(cols, 'dc_out_w')) ?? 0;
    const battNet = parseNumber(col(cols, 'battery_net_w'));
    const avgPackTemp = median([
      parseNumber(col(cols, 'bp1_temp_c')),
      parseNumber(col(cols, 'bp2_temp_c')),
      parseNumber(col(cols, 'bp3_temp_c')),
      parseNumber(col(cols, 'bp4_temp_c')),
      parseNumber(col(cols, 'bp5_temp_c'))
    ]);

    device.acInW = acIn;
    device.dcW = dcOut;
    device.pvW = solarIn;
    device.loadW = acOut + dcOut;
    device.netW = battNet ?? (acIn + dcIn + solarIn - acOut - dcOut);
    if (avgPackTemp !== null) {
      device.tempC = avgPackTemp;
    }

    const estimateMode = col(cols, 'estimate_mode') || undefined;
    const estimateSource = col(cols, 'estimate_source') || undefined;
    const etaEstimate = parseNumber(col(cols, 'estimate_eta_min'));
    const etaCharge = parseNumber(col(cols, 'remain_charge_min'));
    const etaDischarge = parseNumber(col(cols, 'remain_discharge_min'));
    const etaGlobal = parseNumber(col(cols, 'remain_global_min'));
    const chosenEta =
      etaEstimate ??
      (device.state === 'charging' ? etaCharge : null) ??
      (device.state === 'discharging' ? etaDischarge : null) ??
      etaGlobal;
    if (chosenEta !== null) {
      device.etaMinutes = Math.max(0, Math.round(chosenEta));
    }

    const bpCount = parseNumber(col(cols, 'bp_count'));
    const packs = Array.from({ length: 5 }, (_, idxPack) => {
      const n = idxPack + 1;
      const soc = parseNumber(col(cols, `bp${n}_soc`));
      const power = parseNumber(col(cols, `bp${n}_power_w`));
      const temp = parseNumber(col(cols, `bp${n}_temp_c`));
      if (soc === null && power === null && temp === null) return null;
      return {
        id: `bp${n}`,
        socPct: soc ?? undefined,
        powerW: power ?? undefined,
        tempC: temp ?? undefined
      };
    }).filter((v): v is NonNullable<typeof v> => v !== null);

    const caps = getPvPortCaps(modelName);
    const solarPorts = [
      {
        id: 'low',
        name: modelName.includes('delta 2 max') ? 'PV 1' : 'PV Low',
        state: col(cols, 'mppt_low_state') || undefined,
        volts: parseNumber(col(cols, 'solar_low_v')) ?? undefined,
        amps: parseNumber(col(cols, 'solar_low_a')) ?? undefined,
        watts: solarLowIn || undefined,
        maxWatts: caps.low.maxWatts || undefined,
        maxVolts: caps.low.maxVolts || undefined,
        maxAmps: caps.low.maxAmps || undefined
      },
      {
        id: 'high',
        name: modelName.includes('delta 2 max') ? 'PV 2' : 'PV High',
        state: col(cols, 'mppt_high_state') || undefined,
        volts: parseNumber(col(cols, 'solar_high_v')) ?? undefined,
        amps: parseNumber(col(cols, 'solar_high_a')) ?? undefined,
        watts: solarHighIn || undefined,
        maxWatts: caps.high.maxWatts || undefined,
        maxVolts: caps.high.maxVolts || undefined,
        maxAmps: caps.high.maxAmps || undefined
      }
    ];

    const acOn = parseBoolLike(col(cols, 'ac_on'));
    const usbOn = parseBoolLike(col(cols, 'usb_on'));
    const dc12vOn = parseBoolLike(col(cols, 'dc12v_on'));
    const dcOnRaw = parseBoolLike(col(cols, 'dc_on'));
    const hasDcSignal = dcOnRaw === true || usbOn === true || dc12vOn === true;
    const hasAnyDcSignal =
      dcOnRaw !== undefined || usbOn !== undefined || dc12vOn !== undefined;
    const dcOn = hasDcSignal ? true : hasAnyDcSignal ? false : undefined;

    device.details = {
      bpCount: bpCount !== null ? Math.round(bpCount) : undefined,
      packs,
      solarPorts,
      estimateMode,
      estimateSource,
      estimateEtaMin: etaEstimate ?? undefined,
      remainChargeMin: etaCharge ?? undefined,
      remainDischargeMin: etaDischarge ?? undefined,
      remainGlobalMin: etaGlobal ?? undefined,
      mpptLowState: col(cols, 'mppt_low_state') || undefined,
      mpptHighState: col(cols, 'mppt_high_state') || undefined,
      acOn,
      dcOn,
      usbOn,
      dc12vOn,
      evChargingOn: parseBoolLike(col(cols, 'ev_charging_on')),
      fanOn: parseBoolLike(col(cols, 'fan_on')),
      solarChargingOn: parseBoolLike(col(cols, 'solar_charging_on')),
      mqttQueueDepth: parseNumber(col(cols, 'mqtt_queue_depth')) ?? undefined,
      mqttQueueDroppedOldest:
        parseNumber(col(cols, 'mqtt_queue_dropped_oldest')) ?? undefined
    };

    device.updatedAtMs = nowMs;
  }

  const baseOrder = new Map(mockDevices.map((d, i) => [d.serialNumber, i]));
  return Array.from(map.values())
    .sort((a, b) => (baseOrder.get(a.serialNumber) ?? 9999) - (baseOrder.get(b.serialNumber) ?? 9999))
    .map((device) => ({
      id: device.id,
      serialNumber: device.serialNumber,
      name: device.name,
      model: device.model,
      online: device.online,
      batteryPct: device.batteryPct,
      state: device.state,
      etaMinutes: device.etaMinutes,
      pvW: device.pvW,
      acInW: device.acInW,
      dcW: device.dcW,
      loadW: device.loadW,
      netW: device.netW,
      solarTodayWh: getSolarTodayWh(device.serialNumber, nowMs),
      solarGeneratedSeriesWh: getSolarGeneratedSeriesWh(device.serialNumber, nowMs),
      tempC: device.tempC,
      telemetryTsMs: device.telemetryTsMs,
      capabilities: device.capabilities,
      details: device.details
    }));
}

function orderedCandidates(primary: string | null, rest: string[]): string[] {
  const out: string[] = [];
  if (primary) out.push(primary);
  for (const item of rest) {
    if (!out.includes(item)) out.push(item);
  }
  return out;
}

async function fetchTailText(url: string, tailBytes: number): Promise<string | null> {
  // Best-effort incremental fetch: request only tail bytes.
  // If server ignores Range and returns 200, we still trim client-side for parsing cost.
  const response = await fetch(url, {
    cache: 'no-store',
    headers: {
      Range: `bytes=-${tailBytes}`
    }
  });
  if (!response.ok) return null;
  let text = await response.text();
  if (text.length > 0 && !text.startsWith('\n')) {
    const firstNl = text.indexOf('\n');
    if (firstNl >= 0) {
      text = text.slice(firstNl + 1);
    }
  }
  if (text.length > tailBytes * 2) {
    return text.slice(-tailBytes * 2);
  }
  return text;
}

async function fetchFullText(url: string): Promise<string | null> {
  const response = await fetch(url, { cache: 'no-store' });
  if (!response.ok) return null;
  return await response.text();
}

async function fetchMockTrainingCsvText(nowMs: number): Promise<string | null> {
  if (csvBootstrapDone && nowMs - lastCsvPollAt < CSV_POLL_MS) {
    return null;
  }
  lastCsvPollAt = nowMs;

  const candidates = orderedCandidates(preferredCsvUrl, [
    env.mockTrainingUrl,
    '/logs/telemetry_training.csv',
    '/mock/telemetry_training.csv',
    ...nativeMockCandidates('/logs/telemetry_training.csv'),
    ...nativeMockCandidates('/mock/telemetry_training.csv')
  ]);
  for (const url of candidates) {
    try {
      let text = csvBootstrapDone
        ? await fetchTailText(url, CSV_TAIL_BYTES)
        : await fetchFullText(url);
      if (!text) continue;
      if (text.includes('device_sn') && text.includes('soc_pct')) {
        const firstLine = text.split('\n', 1)[0] ?? '';
        if (firstLine.includes('device_sn')) {
          cachedCsvHeaderLine = firstLine;
        }
      } else if (cachedCsvHeaderLine) {
        text = `${cachedCsvHeaderLine}\n${text}`;
      }
      if (text.includes('device_sn') && text.includes('soc_pct')) {
        preferredCsvUrl = url;
        csvBootstrapDone = true;
        return text;
      }
    } catch {
      // Try next candidate.
    }
  }
  return null;
}

async function fetchMockLogText(nowMs: number): Promise<string | null> {
  if (logBootstrapDone && nowMs - lastLogPollAt < LOG_POLL_MS) {
    return null;
  }
  lastLogPollAt = nowMs;

  const candidates = orderedCandidates(preferredLogUrl, [
    env.mockLogUrl,
    '/logs/mqtt.log',
    '/mock/mqtt.log',
    ...nativeMockCandidates('/logs/mqtt.log'),
    ...nativeMockCandidates('/mock/mqtt.log')
  ]);
  for (const url of candidates) {
    try {
      const text = logBootstrapDone
        ? await fetchTailText(url, LOG_TAIL_BYTES)
        : await fetchFullText(url);
      if (!text) continue;
      preferredLogUrl = url;
      logBootstrapDone = true;
      return text;
    } catch {
      // Try next candidate.
    }
  }
  return null;
}

export async function getMockDevices(): Promise<MockDevice[]> {
  const now = Date.now();
  const currentDay = dayKeyLocal(now);
  if (!solarTodayDayKey) {
    solarTodayDayKey = currentDay;
  } else if (solarTodayDayKey !== currentDay) {
    resetSolarTodayState(currentDay);
    csvBootstrapDone = false;
    lastCsvSig = '';
  }
  if (now - cachedAt < REFRESH_MS) {
    return cloneCachedDevices();
  }

  cachedAt = now;
  const csvText = await fetchMockTrainingCsvText(now);
  const logText = await fetchMockLogText(now);
  const csvSig = textSig(csvText);
  const logSig = textSig(logText);

  let changed = false;
  if (csvText && csvSig !== lastCsvSig) {
    cachedCsvParsed = parseDevicesFromTrainingCsv(csvText);
    lastCsvSig = csvSig;
    changed = true;
  }
  if (logText && logSig !== lastLogSig) {
    cachedLogParsed = parseDevicesFromLog(logText);
    lastLogSig = logSig;
    changed = true;
  }

  if (changed && (cachedCsvParsed.length > 0 || cachedLogParsed.length > 0)) {
    const csvBySerial = new Map(cachedCsvParsed.map((d) => [d.serialNumber, d] as const));
    const logBySerial = new Map(cachedLogParsed.map((d) => [d.serialNumber, d] as const));
    const serials = new Set<string>([
      ...Array.from(csvBySerial.keys()),
      ...Array.from(logBySerial.keys())
    ]);

    const mergedDevices: MockDevice[] = [];
    for (const serial of serials) {
      const csvDevice = csvBySerial.get(serial);
      const logDevice = logBySerial.get(serial);
      if (csvDevice) {
        // CSV is authoritative for core telemetry; log enriches details/signals
        // that are not fully represented in training CSV (for example bp heatTime).
        mergedDevices.push({
          ...csvDevice,
          details: mergeDetails(csvDevice.details, logDevice?.details)
        });
      } else if (logDevice) {
        mergedDevices.push(logDevice);
      }
    }

    if (mergedDevices.length > 0) {
      const baseOrder = new Map(mockDevices.map((d, i) => [d.serialNumber, i]));
      mergedDevices.sort(
        (a, b) => (baseOrder.get(a.serialNumber) ?? 9999) - (baseOrder.get(b.serialNumber) ?? 9999)
      );
      const prevBySerial = new Map(cachedDevices.map((d) => [d.serialNumber, d] as const));
      cachedDevices = mergedDevices.map((d) => mergeWithLastKnown(prevBySerial.get(d.serialNumber), d));
    }
  }

  if (
    csvText &&
    (now - solarTodayLastUpdateAt >= SOLAR_TODAY_UPDATE_MS || !solarTodayInitialized)
  ) {
    const fullRebuild = !solarTodayInitialized || !csvBootstrapDone;
    updateSolarTodayFromCsv(csvText, { nowMs: now, fullRebuild });
  }

  if (cachedDevices.length > 0) {
    cachedDevices = cachedDevices.map((device) => ({
      ...device,
      solarTodayWh: getSolarTodayWh(device.serialNumber, now) ?? device.solarTodayWh,
      solarGeneratedSeriesWh:
        getSolarGeneratedSeriesWh(device.serialNumber, now) ?? device.solarGeneratedSeriesWh
    }));
  }
  return cloneCachedDevices();
}

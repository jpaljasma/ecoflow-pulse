import { env } from '@/shared/config/env';
import * as Linking from 'expo-linking';
import { Platform } from 'react-native';
import { type MockDevice, mockDevices } from '@/features/devices/mockData';

type MutableDevice = MockDevice & {
  updatedAtMs: number;
};

const REFRESH_MS = 1_000;
const MAX_PARSED_LINES = 8_000;
const MAX_PARSED_CSV_LINES = 20_000;
const SERIAL_CONTEXT_TTL_LINES = 240;
const SERIAL_FROM_TOPIC = /\/([A-Z0-9]{8,})\/quota/;
const SERIAL_HINT = /\b(?:device_sn|sn)=([A-Z0-9]{8,})\b/;
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

let cachedAt = 0;
let cachedDevices: MutableDevice[] = mockDevices.map((d) => ({
  ...d,
  updatedAtMs: Date.now()
}));

function cloneCachedDevices(): MockDevice[] {
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
    tempC: device.tempC,
    telemetryTsMs: device.telemetryTsMs,
    capabilities: device.capabilities,
    details: device.details
  }));
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
    if (serialHint) {
      activeSerial = serialHint;
      lastSerialAnchorLine = i;
      sessionSerials.add(serialHint);
      getOrCreateDevice(activeSerial, map).online = true;
    }

    const topicMatch = line.match(SERIAL_FROM_TOPIC);
    if (topicMatch?.[1]) {
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
    const serial = col(cols, 'device_sn') ?? '';
    if (!serial) continue;

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
      acOn: parseBoolLike(col(cols, 'ac_on')),
      dcOn: parseBoolLike(col(cols, 'dc_on')),
      usbOn: parseBoolLike(col(cols, 'usb_on')),
      dc12vOn: parseBoolLike(col(cols, 'dc12v_on')),
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
      tempC: device.tempC,
      telemetryTsMs: device.telemetryTsMs,
      capabilities: device.capabilities,
      details: device.details
    }));
}

async function fetchMockTrainingCsvText(): Promise<string | null> {
  const candidates = [
    env.mockTrainingUrl,
    '/logs/telemetry_training.csv',
    '/mock/telemetry_training.csv',
    ...nativeMockCandidates('/logs/telemetry_training.csv'),
    ...nativeMockCandidates('/mock/telemetry_training.csv')
  ];
  for (const url of candidates) {
    try {
      const response = await fetch(url, { cache: 'no-store' });
      if (!response.ok) continue;
      const text = await response.text();
      if (text.includes('device_sn') && text.includes('soc_pct')) {
        return text;
      }
    } catch {
      // Try next candidate.
    }
  }
  return null;
}

async function fetchMockLogText(): Promise<string | null> {
  const candidates = [
    env.mockLogUrl,
    '/logs/mqtt.log',
    '/mock/mqtt.log',
    ...nativeMockCandidates('/logs/mqtt.log'),
    ...nativeMockCandidates('/mock/mqtt.log')
  ];
  for (const url of candidates) {
    try {
      const response = await fetch(url, { cache: 'no-store' });
      if (!response.ok) continue;
      return await response.text();
    } catch {
      // Try next candidate.
    }
  }
  return null;
}

export async function getMockDevices(): Promise<MockDevice[]> {
  const now = Date.now();
  if (now - cachedAt < REFRESH_MS) {
    return cloneCachedDevices();
  }

  cachedAt = now;
  const csvText = await fetchMockTrainingCsvText();
  const logText = await fetchMockLogText();
  const parsedCsv = csvText ? parseDevicesFromTrainingCsv(csvText) : [];
  const parsedLog = logText ? parseDevicesFromLog(logText) : [];

  if (parsedCsv.length > 0 || parsedLog.length > 0) {
    const merged = new Map<string, MockDevice>();
    const consider = (device: MockDevice) => {
      const prev = merged.get(device.serialNumber);
      if (!prev) {
        merged.set(device.serialNumber, device);
        return;
      }
      const prevTs = prev.telemetryTsMs ?? 0;
      const nextTs = device.telemetryTsMs ?? 0;
      if (nextTs >= prevTs) {
        merged.set(device.serialNumber, device);
      }
    };
    parsedCsv.forEach(consider);
    parsedLog.forEach(consider);

    const mergedDevices = Array.from(merged.values());
    if (mergedDevices.length > 0) {
      cachedDevices = mergedDevices.map((d) => ({ ...d, updatedAtMs: Date.now() }));
    }
  }
  return cloneCachedDevices();
}

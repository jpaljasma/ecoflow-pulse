import { isPulseGlobalAdmin } from '@/shared/authz/pulseRoles';

export type LogStatus = 'ok' | 'warning' | 'error';

export type AdminLogEntry = {
  id: string;
  ts: number;
  receivedTs: number;
  deviceId: string;
  status: LogStatus;
  source: string;
  sourceKind: string;
  typeCode: string;
  summary: string;
  labels: Record<string, string>;
  detail: Record<string, unknown>;
};

export type BufferedAdminLogEntry = AdminLogEntry & {
  rowKey: string;
};

export type AdminLogFilterOption = {
  kind: 'device' | 'serial' | 'user';
  id: string;
  label: string;
  secondaryLabel: string;
  deviceIds: string[];
  provider?: string;
};

export type AdminLogSubscribeFilters = {
  deviceIds: string[];
  statuses: LogStatus[];
  providers: string[];
  sources: string[];
  typeCodes: string[];
};

export type AdminLogTypeFilterValue = '' | 'quota' | 'status' | 'telemetry' | 'info';

export type AdminLogTypeFilterOption = {
  value: AdminLogTypeFilterValue;
  label: string;
  typeCodes: readonly string[];
};

export const ADMIN_LOG_TYPE_FILTER_OPTIONS: readonly AdminLogTypeFilterOption[] = [
  { value: '', label: 'All', typeCodes: [] },
  { value: 'quota', label: 'Quota', typeCodes: ['quota'] },
  { value: 'status', label: 'Status', typeCodes: ['pdStatus', 'invStatus', 'emsStatus', 'bmsStatus', 'mpptStatus'] },
  { value: 'telemetry', label: 'Telemetry', typeCodes: ['telemetry'] },
  { value: 'info', label: 'Info', typeCodes: ['kitInfo', 'bms_kitInfo'] }
];

export type AppendLogState = {
  entries: BufferedAdminLogEntry[];
  pending: BufferedAdminLogEntry[];
  pendingCount: number;
  generation: number;
  nextRowOrdinal: number;
};

export const DEFAULT_LOG_KEEP_LIMIT = 50;
const DEFAULT_MAX_PENDING = 200;
const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const searchableTextCache = new WeakMap<AdminLogEntry, string>();

export function createInitialLogState(): AppendLogState {
  return { entries: [], pending: [], pendingCount: 0, generation: 0, nextRowOrdinal: 0 };
}

export function resetLogState(state: AppendLogState): AppendLogState {
  return {
    entries: [],
    pending: [],
    pendingCount: 0,
    generation: state.generation + 1,
    nextRowOrdinal: 0
  };
}

export function trimLogState(state: AppendLogState, maxEntries = DEFAULT_LOG_KEEP_LIMIT): AppendLogState {
  return {
    ...state,
    entries: state.entries.slice(0, Math.max(0, maxEntries))
  };
}

export function isGlobalAdmin(roles: readonly string[] | undefined): boolean {
  return isPulseGlobalAdmin(roles);
}

export function buildSubscribeFilters(input: {
  selectedOptions: AdminLogFilterOption[];
  deviceIds?: readonly string[];
  statuses: LogStatus[];
  provider?: string;
  source?: string;
  typeCode?: string;
  typeCodes?: readonly string[];
}): AdminLogSubscribeFilters {
  return {
    deviceIds: unique([...(input.deviceIds ?? []), ...input.selectedOptions.flatMap((option) => option.deviceIds)]),
    statuses: unique(input.statuses),
    providers: input.provider ? [input.provider] : [],
    sources: input.source ? [input.source] : [],
    typeCodes: unique([...(input.typeCodes ?? []), ...(input.typeCode ? [input.typeCode] : [])])
  };
}

export function toggleExclusiveStatusFilter(current: readonly LogStatus[], next: LogStatus): LogStatus[] {
  return current.length === 1 && current[0] === next ? [] : [next];
}

export function resolveAdminLogsRouteState(params: Record<string, string | string[] | undefined>): {
  deviceId?: string;
} {
  const requestedDeviceId = normalizeScalar(params.deviceId) ?? normalizeScalar(params.device);
  return isCanonicalDeviceId(requestedDeviceId) ? { deviceId: requestedDeviceId } : {};
}

export function buildAdminLogsRouteParams(input: { deviceId?: string }): Record<string, string> {
  return isCanonicalDeviceId(input.deviceId) ? { deviceId: input.deviceId } : {};
}

export function mergeAdminLogFilterOptions(options: readonly AdminLogFilterOption[]): AdminLogFilterOption[] {
  const merged = new Map<string, AdminLogFilterOption>();
  for (const option of options) {
    const key = `${option.kind}:${option.id}`;
    const existing = merged.get(key);
    if (!existing) {
      merged.set(key, { ...option, deviceIds: unique(option.deviceIds) });
      continue;
    }
    merged.set(key, {
      ...existing,
      label: existing.label || option.label,
      secondaryLabel: existing.secondaryLabel || option.secondaryLabel,
      provider: existing.provider ?? option.provider,
      deviceIds: unique([...existing.deviceIds, ...option.deviceIds])
    });
  }
  return [...merged.values()];
}

export function appendLogEntry(
  state: AppendLogState,
  entry: AdminLogEntry,
  options: { paused: boolean; maxEntries?: number; maxPending?: number }
): AppendLogState {
  const maxEntries = options.maxEntries ?? DEFAULT_LOG_KEEP_LIMIT;
  const maxPending = options.maxPending ?? DEFAULT_MAX_PENDING;
  const bufferedEntry = bufferEntry(state, entry);
  const identity = getLogEntryIdentity(entry);
  const pending = removeLogIdentity(state.pending, identity);
  const baseState = { ...state, pending, pendingCount: pending.length, nextRowOrdinal: state.nextRowOrdinal + 1 };
  if (options.paused) {
    const nextPending = prependBounded(bufferedEntry, pending, maxPending);
    return {
      ...baseState,
      pending: nextPending,
      pendingCount: nextPending.length
    };
  }
  const entries = removeLogIdentity(state.entries, identity);
  return {
    ...baseState,
    entries: prependBounded(bufferedEntry, entries, maxEntries),
    pending: [],
    pendingCount: 0
  };
}

export function resumePending(state: AppendLogState, maxEntries = DEFAULT_LOG_KEEP_LIMIT): AppendLogState {
  if (state.pending.length === 0) {
    return { ...state, pendingCount: 0 };
  }
  const pendingIdentities = new Set<string>();
  for (const entry of state.pending) {
    pendingIdentities.add(getLogEntryIdentity(entry));
  }
  return {
    ...state,
    entries: [...state.pending, ...state.entries.filter((entry) => !pendingIdentities.has(getLogEntryIdentity(entry)))].slice(0, maxEntries),
    pending: [],
    pendingCount: 0
  };
}

export function fuzzyFilterLogEntries<T extends AdminLogEntry>(entries: T[], query: string): T[] {
  const tokens = normalizeText(query).split(/\s+/).filter(Boolean);
  if (tokens.length === 0) {
    return entries;
  }
  return entries.filter((entry) => {
    const haystack = getSearchableLogText(entry);
    return tokens.every((token) => haystack.includes(token));
  });
}

export function redactEntryForCopy(entry: AdminLogEntry): AdminLogEntry {
  const baseEntry = { ...entry } as AdminLogEntry & { rowKey?: string };
  delete baseEntry.rowKey;
  return {
    ...baseEntry,
    labels: redactRecord(baseEntry.labels),
    detail: redactUnknown(baseEntry.detail) as Record<string, unknown>
  };
}

function searchableLogText(entry: AdminLogEntry): string {
  let text = [
    entry.id,
    entry.deviceId,
    entry.status,
    entry.source,
    entry.sourceKind,
    entry.typeCode,
    entry.summary
  ].join(' ');
  for (const [key, value] of Object.entries(entry.labels)) {
    text += ` ${key} ${value}`;
  }
  return text;
}

function getSearchableLogText(entry: AdminLogEntry): string {
  const cached = searchableTextCache.get(entry);
  if (cached !== undefined) {
    return cached;
  }
  const text = normalizeText(searchableLogText(entry));
  searchableTextCache.set(entry, text);
  return text;
}

function bufferEntry(state: AppendLogState, entry: AdminLogEntry): BufferedAdminLogEntry {
  return {
    ...entry,
    rowKey: `${state.generation}:${state.nextRowOrdinal}:${entry.id}:${entry.receivedTs}`
  };
}

function getLogEntryIdentity(entry: AdminLogEntry): string {
  const id = entry.id.trim();
  if (id) {
    return `id:${id}`;
  }
  return [
    'fallback',
    entry.deviceId,
    String(entry.receivedTs),
    entry.source,
    entry.typeCode,
    entry.summary
  ].join(':');
}

function removeLogIdentity<T extends AdminLogEntry>(items: readonly T[], identity: string): readonly T[] {
  const index = items.findIndex((item) => getLogEntryIdentity(item) === identity);
  if (index < 0) {
    return items;
  }
  return [...items.slice(0, index), ...items.slice(index + 1)];
}

function prependBounded<T>(item: T, items: readonly T[], maxItems: number): T[] {
  if (maxItems <= 0) {
    return [];
  }
  if (items.length >= maxItems) {
    return [item, ...items.slice(0, maxItems - 1)];
  }
  return [item, ...items];
}

function redactRecord(record: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(record)) {
    out[key] = isSensitiveKey(key) ? '<redacted>' : value;
  }
  return out;
}

function redactUnknown(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(redactUnknown);
  }
  if (!value || typeof value !== 'object') {
    return value;
  }
  const out: Record<string, unknown> = {};
  for (const [key, raw] of Object.entries(value)) {
    out[key] = isSensitiveKey(key) ? '<redacted>' : redactUnknown(raw);
  }
  return out;
}

function isSensitiveKey(key: string): boolean {
  return /(access.?key|secret|token|jwt|password|credential|provider.?device.?id|serial|sn|ecoflow.?sn|email)/i.test(key);
}

function unique<T extends string>(values: readonly T[]): T[] {
  const seen = new Set<T>();
  const out: T[] = [];
  for (const value of values) {
    if (!value || seen.has(value)) {
      continue;
    }
    seen.add(value);
    out.push(value);
  }
  return out;
}

function normalizeText(value: string): string {
  return value.trim().toLowerCase();
}

function normalizeScalar(value: string | string[] | undefined): string | undefined {
  if (Array.isArray(value)) {
    return value[0]?.trim() || undefined;
  }
  return value?.trim() || undefined;
}

function isCanonicalDeviceId(value: string | undefined): value is string {
  return Boolean(value && UUID_PATTERN.test(value));
}

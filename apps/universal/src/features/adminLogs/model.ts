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

export type AdminLogFilterOption = {
  kind: 'device' | 'serial' | 'user';
  id: string;
  label: string;
  secondaryLabel: string;
  deviceIds: string[];
};

export type AdminLogSubscribeFilters = {
  deviceIds: string[];
  statuses: LogStatus[];
  sources: string[];
  typeCodes: string[];
};

export type AppendLogState = {
  entries: AdminLogEntry[];
  pending: AdminLogEntry[];
  pendingCount: number;
};

const DEFAULT_MAX_ENTRIES = 500;
const DEFAULT_MAX_PENDING = 200;
const searchableTextCache = new WeakMap<AdminLogEntry, string>();

export function createInitialLogState(): AppendLogState {
  return { entries: [], pending: [], pendingCount: 0 };
}

export function isGlobalAdmin(roles: readonly string[] | undefined): boolean {
  return roles?.some((role) => role.trim().toLowerCase() === 'admin') ?? false;
}

export function buildSubscribeFilters(input: {
  selectedOptions: AdminLogFilterOption[];
  statuses: LogStatus[];
  source?: string;
  typeCode?: string;
}): AdminLogSubscribeFilters {
  return {
    deviceIds: unique(input.selectedOptions.flatMap((option) => option.deviceIds)),
    statuses: unique(input.statuses),
    sources: input.source ? [input.source] : [],
    typeCodes: input.typeCode ? [input.typeCode] : []
  };
}

export function appendLogEntry(
  state: AppendLogState,
  entry: AdminLogEntry,
  options: { paused: boolean; maxEntries?: number; maxPending?: number }
): AppendLogState {
  const maxEntries = options.maxEntries ?? DEFAULT_MAX_ENTRIES;
  const maxPending = options.maxPending ?? DEFAULT_MAX_PENDING;
  if (options.paused) {
    return {
      ...state,
      pending: prependBounded(entry, state.pending, maxPending),
      pendingCount: Math.min(state.pendingCount + 1, maxPending)
    };
  }
  return {
    entries: prependBounded(entry, state.entries, maxEntries),
    pending: [],
    pendingCount: 0
  };
}

export function resumePending(state: AppendLogState, maxEntries = DEFAULT_MAX_ENTRIES): AppendLogState {
  if (state.pending.length === 0) {
    return { ...state, pendingCount: 0 };
  }
  return {
    entries: [...state.pending, ...state.entries].slice(0, maxEntries),
    pending: [],
    pendingCount: 0
  };
}

export function fuzzyFilterLogEntries(entries: AdminLogEntry[], query: string): AdminLogEntry[] {
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
  return {
    ...entry,
    labels: redactRecord(entry.labels),
    detail: redactUnknown(entry.detail) as Record<string, unknown>
  };
}

function searchableLogText(entry: AdminLogEntry): string {
  return [
    entry.id,
    entry.deviceId,
    entry.status,
    entry.source,
    entry.sourceKind,
    entry.typeCode,
    entry.summary,
    ...Object.entries(entry.labels).flat()
  ].join(' ');
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

function unique<T extends string>(values: T[]): T[] {
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

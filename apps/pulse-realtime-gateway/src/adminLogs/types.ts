export type AdminLogStatus = 'ok' | 'warning' | 'error';

export type AdminLogFilters = {
  deviceIds: string[];
  statuses: AdminLogStatus[];
  providers: string[];
  sources: string[];
  typeCodes: string[];
};

export type AdminLogEntry = {
  id: string;
  ts: number;
  receivedTs: number;
  deviceId: string;
  status: AdminLogStatus;
  source: string;
  sourceKind: string;
  typeCode: string;
  summary: string;
  labels: Record<string, string>;
  detail: Record<string, unknown>;
};

export type AdminLogSubscribeInput = {
  subscriptionId: string;
  filters: AdminLogFilters;
  replayLimit: number;
  replaySinceUnixMs: number;
  authHeader?: string;
  requestId: string;
  onEntry: (entry: AdminLogEntry) => void;
  onReplayDone: (info: { replayed: number }) => void;
  onStatus: (status: { state: 'replay' | 'live' | 'error'; message?: string }) => void;
};

export type AdminLogSubscription = {
  close: () => void;
};

export interface AdminLogSource {
  subscribe(input: AdminLogSubscribeInput): AdminLogSubscription;
  close(): void | Promise<void>;
}

export const DEFAULT_ADMIN_LOG_FILTERS: AdminLogFilters = {
  deviceIds: [],
  statuses: [],
  providers: [],
  sources: [],
  typeCodes: []
};

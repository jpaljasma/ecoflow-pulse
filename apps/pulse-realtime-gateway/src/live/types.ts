export type LiveCursor = {
  seq: number;
  tsUnixMs: number;
};

export type LiveSnapshot = {
  deviceId: string;
  cursor: LiveCursor;
  metrics: Record<string, number>;
};

export type LiveDelta = {
  deviceId: string;
  cursor: LiveCursor;
  changed: Record<string, number>;
  cleared: string[];
};

export type LiveHeartbeat = {
  deviceId: string;
  cursor: LiveCursor;
};

export type SubscribeInput = {
  deviceId: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
  onSnapshot: (snapshot: LiveSnapshot) => void;
  onDelta: (delta: LiveDelta) => void;
  onHeartbeat: (heartbeat: LiveHeartbeat) => void;
  onClose: (error?: Error & { code?: number }) => void;
};

export type LiveSubscription = {
  close: () => void;
};

export interface LiveTelemetryClient {
  subscribe(input: SubscribeInput): LiveSubscription;
  close(): void;
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { env } from '@/shared/config/env';
import {
  appendLogEntry,
  createInitialLogState,
  resumePending,
  type AdminLogEntry,
  type AdminLogSubscribeFilters,
  type AppendLogState
} from '@/features/adminLogs/model';

type LogsConnectionState = 'idle' | 'connecting' | 'replay' | 'live' | 'forbidden' | 'error' | 'closed';

type UseAdminLogStreamInput = {
  token?: string;
  enabled: boolean;
  filters: AdminLogSubscribeFilters;
};

const SUBSCRIPTION_ID = 'admin-logs';
const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;

export function useAdminLogStream({ token, enabled, filters }: UseAdminLogStreamInput) {
  const [state, setState] = useState<AppendLogState>(createInitialLogState);
  const [connectionState, setConnectionState] = useState<LogsConnectionState>('idle');
  const [replayedCount, setReplayedCount] = useState(0);
  const [paused, setPaused] = useState(false);
  const pausedRef = useRef(false);
  const socketRef = useRef<WebSocket | null>(null);
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);

  useEffect(() => {
    pausedRef.current = paused;
    if (!paused) {
      setState((current) => resumePending(current));
    }
  }, [paused]);

  useEffect(() => {
    let disposed = false;
    let terminal = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    const resetBuffer = () => {
      setState(createInitialLogState());
      setReplayedCount(0);
    };

    if (!enabled) {
      resetBuffer();
      setConnectionState('idle');
      return;
    }

    resetBuffer();
    const subscribeFilters = parseSubscribeFilters(filtersKey);

    const openSocket = (attempt: number) => {
      if (disposed || terminal) {
        return;
      }
      const ws = new WebSocket(buildWebSocketUrl(env.wsUrl, token));
      socketRef.current = ws;
      setConnectionState('connecting');

      ws.onopen = () => {
        ws.send(JSON.stringify(buildSubscribeMessage(subscribeFilters)));
      };

      ws.onmessage = (event) => {
        const message = parseJsonRecord(event.data);
        if (!message) {
          return;
        }
        const entry = message.entry;
        if (message.type === 'log_entry' && message.subscriptionId === SUBSCRIPTION_ID && isLogEntry(entry)) {
          setState((current) => appendLogEntry(current, entry, { paused: pausedRef.current }));
          return;
        }
        if (message.type === 'logs_replay_done' && typeof message.replayed === 'number') {
          setReplayedCount(message.replayed);
          return;
        }
        if (message.type === 'logs_status' && typeof message.state === 'string') {
          const nextState = normalizeConnectionState(message.state);
          terminal = nextState === 'forbidden';
          setConnectionState(nextState);
        }
      };

      ws.onerror = () => {
        setConnectionState('error');
      };
      ws.onclose = () => {
        if (socketRef.current === ws) {
          socketRef.current = null;
        }
        if (disposed) {
          return;
        }
        if (terminal) {
          setConnectionState('forbidden');
          return;
        }
        setConnectionState('closed');
        reconnectTimer = setTimeout(() => openSocket(attempt + 1), reconnectDelayMs(attempt));
      };
    };

    openSocket(0);

    return () => {
      disposed = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
      }
      const ws = socketRef.current;
      if (!ws) {
        return;
      }
      ws.onopen = null;
      ws.onmessage = null;
      ws.onerror = null;
      ws.onclose = null;
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'logs_unsubscribe', subscriptionId: SUBSCRIPTION_ID }));
      }
      ws.close();
      if (socketRef.current === ws) {
        socketRef.current = null;
      }
    };
  }, [enabled, filtersKey, token]);

  const clear = useCallback(() => {
    setState(createInitialLogState());
  }, []);

  return {
    entries: state.entries,
    pendingCount: state.pendingCount,
    connectionState,
    replayedCount,
    paused,
    setPaused,
    clear
  };
}

function normalizeConnectionState(value: string): LogsConnectionState {
  switch (value) {
    case 'replay':
    case 'live':
    case 'forbidden':
    case 'error':
    case 'closed':
      return value;
    default:
      return 'idle';
  }
}

function isLogEntry(value: unknown): value is AdminLogEntry {
  return Boolean(
    value &&
      typeof value === 'object' &&
      typeof (value as AdminLogEntry).id === 'string' &&
      typeof (value as AdminLogEntry).deviceId === 'string'
  );
}

function buildWebSocketUrl(baseUrl: string, token?: string): string {
  return token ? `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}` : baseUrl;
}

function buildSubscribeMessage(filters: AdminLogSubscribeFilters) {
  return {
    type: 'logs_subscribe',
    subscriptionId: SUBSCRIPTION_ID,
    filters,
    replayLimit: 200
  };
}

function parseSubscribeFilters(filtersKey: string): AdminLogSubscribeFilters {
  return JSON.parse(filtersKey) as AdminLogSubscribeFilters;
}

function parseJsonRecord(data: unknown): Record<string, unknown> | null {
  if (typeof data !== 'string') {
    return null;
  }
  try {
    const decoded: unknown = JSON.parse(data);
    return decoded && typeof decoded === 'object' ? (decoded as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function reconnectDelayMs(attempt: number): number {
  return Math.min(RECONNECT_MAX_MS, RECONNECT_BASE_MS * 2 ** Math.min(attempt, 5));
}

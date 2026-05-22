import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { env } from '@/shared/config/env';
import {
  appendLogEntry,
  createInitialLogState,
  DEFAULT_LOG_KEEP_LIMIT,
  resetLogState,
  resumePending,
  trimLogState,
  type AdminLogEntry,
  type AdminLogSubscribeFilters,
  type AppendLogState
} from '@/features/adminLogs/model';

type LogsConnectionState = 'idle' | 'connecting' | 'replay' | 'live' | 'forbidden' | 'error' | 'closed';

type UseAdminLogStreamInput = {
  token?: string;
  enabled: boolean;
  active?: boolean;
  filters: AdminLogSubscribeFilters;
  maxEntries?: number;
  holdVisible?: boolean;
};

const SUBSCRIPTION_ID = 'admin-logs';
const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;

export function useAdminLogStream({
  token,
  enabled,
  active = true,
  filters,
  maxEntries = DEFAULT_LOG_KEEP_LIMIT,
  holdVisible = false
}: UseAdminLogStreamInput) {
  const [state, setState] = useState<AppendLogState>(createInitialLogState);
  const [connectionState, setConnectionState] = useState<LogsConnectionState>('idle');
  const [replayedCount, setReplayedCount] = useState(0);
  const [paused, setPaused] = useState(false);
  const pausedRef = useRef(false);
  const holdVisibleRef = useRef(holdVisible);
  const maxEntriesRef = useRef(normalizeMaxEntries(maxEntries));
  const socketRef = useRef<WebSocket | null>(null);
  const subscriptionKeyRef = useRef('');
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);

  useEffect(() => {
    const nextMaxEntries = normalizeMaxEntries(maxEntries);
    maxEntriesRef.current = nextMaxEntries;
    setState((current) => trimLogState(current, nextMaxEntries));
  }, [maxEntries]);

  useEffect(() => {
    pausedRef.current = paused;
    if (!paused && !holdVisibleRef.current) {
      setState((current) => resumePending(current, maxEntriesRef.current));
    }
  }, [paused]);

  useEffect(() => {
    holdVisibleRef.current = holdVisible;
    if (!pausedRef.current && !holdVisible) {
      setState((current) => resumePending(current, maxEntriesRef.current));
    }
  }, [holdVisible]);

  useEffect(() => {
    let disposed = false;
    let terminal = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    const resetBuffer = () => {
      setState((current) => resetLogState(current));
      setReplayedCount(0);
    };

    if (!enabled) {
      resetBuffer();
      subscriptionKeyRef.current = '';
      setConnectionState('idle');
      return;
    }

    if (!active) {
      setConnectionState('idle');
      return;
    }

    const subscriptionKey = `${token ?? ''}:${filtersKey}`;
    if (subscriptionKeyRef.current !== subscriptionKey) {
      resetBuffer();
      subscriptionKeyRef.current = subscriptionKey;
    }
    const subscribeFilters = parseSubscribeFilters(filtersKey);

    const openSocket = (attempt: number) => {
      if (disposed || terminal) {
        return;
      }
      const ws = new WebSocket(buildWebSocketUrl(env.wsUrl, token));
      socketRef.current = ws;
      setConnectionState('connecting');

      ws.onopen = () => {
        ws.send(JSON.stringify(buildSubscribeMessage(subscribeFilters, maxEntriesRef.current)));
      };

      ws.onmessage = (event) => {
        if (disposed || socketRef.current !== ws) {
          return;
        }
        const message = parseJsonRecord(event.data);
        if (!message) {
          return;
        }
        const entry = message.entry;
        if (message.type === 'log_entry' && message.subscriptionId === SUBSCRIPTION_ID && isLogEntry(entry)) {
          setState((current) =>
            appendLogEntry(current, entry, {
              paused: pausedRef.current || holdVisibleRef.current,
              maxEntries: maxEntriesRef.current
            })
          );
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
  }, [active, enabled, filtersKey, token]);

  const clear = useCallback(() => {
    setState((current) => resetLogState(current));
    setReplayedCount(0);
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

function buildSubscribeMessage(filters: AdminLogSubscribeFilters, maxEntries: number) {
  return {
    type: 'logs_subscribe',
    subscriptionId: SUBSCRIPTION_ID,
    filters,
    replayLimit: Math.min(200, Math.max(1, maxEntries))
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

function normalizeMaxEntries(value: number): number {
  return Number.isFinite(value) ? Math.max(1, Math.min(200, Math.floor(value))) : DEFAULT_LOG_KEEP_LIMIT;
}

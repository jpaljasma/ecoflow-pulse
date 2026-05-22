import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { env } from '@/shared/config/env';
import {
  appendLogEntry,
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

export function useAdminLogStream({ token, enabled, filters }: UseAdminLogStreamInput) {
  const [state, setState] = useState<AppendLogState>({ entries: [], pending: [], pendingCount: 0 });
  const [connectionState, setConnectionState] = useState<LogsConnectionState>('idle');
  const [replayedCount, setReplayedCount] = useState(0);
  const [paused, setPaused] = useState(false);
  const pausedRef = useRef(false);
  const socketRef = useRef<WebSocket | null>(null);
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);
  const subscribeFilters = useMemo(
    () => JSON.parse(filtersKey) as AdminLogSubscribeFilters,
    [filtersKey]
  );

  useEffect(() => {
    pausedRef.current = paused;
    if (!paused) {
      setState((current) => resumePending(current));
    }
  }, [paused]);

  useEffect(() => {
    if (!enabled) {
      setConnectionState('idle');
      return;
    }
    const baseUrl = env.wsUrl;
    const url = token ? `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}` : baseUrl;
    const ws = new WebSocket(url);
    socketRef.current = ws;
    setConnectionState('connecting');
    setReplayedCount(0);

    ws.onopen = () => {
      ws.send(
        JSON.stringify({
          type: 'logs_subscribe',
          subscriptionId: SUBSCRIPTION_ID,
          filters: subscribeFilters,
          replayLimit: 200
        })
      );
    };

    ws.onmessage = (event) => {
      if (typeof event.data !== 'string') {
        return;
      }
      let decoded: unknown;
      try {
        decoded = JSON.parse(event.data);
      } catch {
        return;
      }
      if (!decoded || typeof decoded !== 'object') {
        return;
      }
      const message = decoded as Record<string, unknown>;
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
        setConnectionState(normalizeConnectionState(message.state));
      }
    };

    ws.onerror = () => {
      setConnectionState('error');
    };
    ws.onclose = () => {
      setConnectionState((current) => (current === 'forbidden' ? current : 'closed'));
    };

    return () => {
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
  }, [enabled, filtersKey, subscribeFilters, token]);

  const clear = useCallback(() => {
    setState({ entries: [], pending: [], pendingCount: 0 });
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

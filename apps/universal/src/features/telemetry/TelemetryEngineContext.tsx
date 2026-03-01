import { createContext, useContext, useEffect, useMemo } from 'react';
import { useAuthSession } from '@/features/auth/hooks';
import { TelemetryEngine } from '@/features/telemetry/engine/TelemetryEngine';

type TelemetryEngineContextValue = {
  engine: TelemetryEngine;
};

const TelemetryEngineContext = createContext<TelemetryEngineContextValue | null>(null);

export function TelemetryEngineProvider({ children }: { children: React.ReactNode }) {
  const value = useMemo(() => ({ engine: new TelemetryEngine() }), []);
  const { authConfigured, authReady, token } = useAuthSession();

  useEffect(() => {
    return () => {
      value.engine.disconnect();
    };
  }, [value.engine]);

  useEffect(() => {
    if (!authReady) {
      return;
    }
    value.engine.connect(token, { authRequired: authConfigured });
  }, [authConfigured, authReady, token, value.engine]);

  return <TelemetryEngineContext.Provider value={value}>{children}</TelemetryEngineContext.Provider>;
}

export function useTelemetryEngine(): TelemetryEngine {
  const ctx = useContext(TelemetryEngineContext);
  if (!ctx) {
    throw new Error('useTelemetryEngine must be used inside TelemetryEngineProvider');
  }
  return ctx.engine;
}

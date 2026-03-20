import AsyncStorage from '@react-native-async-storage/async-storage';
import type { TokenResponse } from 'expo-auth-session';

import { create, createJSONStorage, persist } from '@/shared/state/zustand';

export type StoredOidcSession = {
  issuerUrl: string;
  clientId: string;
  accessToken: string;
  refreshToken: string;
  idToken: string;
  tokenType: string;
  expiresAtUnixMs: number;
  updatedAtUnixMs: number;
};

export type ReauthReason = 'session_expired';

type AuthState = {
  hydrated: boolean;
  refreshing: boolean;
  session: StoredOidcSession | null;
  reauthRequest: {
    nonce: number;
    reason: ReauthReason | null;
  };
  setHydrated: (hydrated: boolean) => void;
  setRefreshing: (refreshing: boolean) => void;
  setSession: (input: {
    issuerUrl: string;
    clientId: string;
    token: Pick<TokenResponse, 'accessToken' | 'refreshToken' | 'idToken' | 'tokenType' | 'issuedAt' | 'expiresIn'>;
  }) => void;
  clearSession: () => void;
  requestReauthentication: (reason: ReauthReason) => void;
  clearReauthenticationRequest: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      hydrated: false,
      refreshing: false,
      session: null,
      reauthRequest: {
        nonce: 0,
        reason: null
      },
      setHydrated: (hydrated) => set({ hydrated }),
      setRefreshing: (refreshing) => set({ refreshing }),
      setSession: (input) =>
        set(() => ({
          refreshing: false,
          session: {
            issuerUrl: input.issuerUrl,
            clientId: input.clientId,
            accessToken: input.token.accessToken ?? '',
            refreshToken: input.token.refreshToken ?? '',
            idToken: input.token.idToken ?? '',
            tokenType: input.token.tokenType ?? 'Bearer',
            expiresAtUnixMs: deriveExpiresAtUnixMs(input.token.issuedAt, input.token.expiresIn),
            updatedAtUnixMs: Date.now()
          }
        })),
      clearSession: () => set({ refreshing: false, session: null }),
      requestReauthentication: (reason) =>
        set((state) => ({
          reauthRequest: {
            nonce: state.reauthRequest.nonce + 1,
            reason
          }
        })),
      clearReauthenticationRequest: () =>
        set((state) => ({
          reauthRequest: {
            nonce: state.reauthRequest.nonce,
            reason: null
          }
        }))
    }),
    {
      name: 'pulse-oidc-session-v1',
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ session: state.session }),
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      }
    }
  )
);

export function isSessionExpired(session: StoredOidcSession | null, nowUnixMs: number): boolean {
  if (!session) return true;
  if (session.expiresAtUnixMs <= 0) return false;
  return nowUnixMs >= session.expiresAtUnixMs;
}

function deriveExpiresAtUnixMs(issuedAtSeconds?: number, expiresInSeconds?: number): number {
  if (!expiresInSeconds || expiresInSeconds <= 0) {
    return 0;
  }
  const baseMs = issuedAtSeconds && issuedAtSeconds > 0 ? issuedAtSeconds * 1000 : Date.now();
  return baseMs + expiresInSeconds * 1000;
}

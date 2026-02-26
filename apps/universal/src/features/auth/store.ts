import AsyncStorage from '@react-native-async-storage/async-storage';
import type { TokenResponse } from 'expo-auth-session';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

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

type AuthState = {
  session: StoredOidcSession | null;
  setSession: (input: {
    issuerUrl: string;
    clientId: string;
    token: Pick<TokenResponse, 'accessToken' | 'refreshToken' | 'idToken' | 'tokenType' | 'issuedAt' | 'expiresIn'>;
  }) => void;
  clearSession: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      session: null,
      setSession: (input) =>
        set(() => ({
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
      clearSession: () => set({ session: null })
    }),
    {
      name: 'pulse-oidc-session-v1',
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ session: state.session })
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

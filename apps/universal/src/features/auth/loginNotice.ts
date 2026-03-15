import type { ReauthReason } from '@/features/auth/store';

export type LoginNotice = {
  iconText: string;
  headline: string;
  detail: string;
  statusLabel: string;
};

export function parseReauthReason(value: string | string[] | undefined): ReauthReason | null {
  const raw = Array.isArray(value) ? value[0] : value;
  if (raw === 'session_expired') {
    return raw;
  }
  return null;
}

export function buildLoginNotice(reason: ReauthReason | null): LoginNotice | null {
  if (reason !== 'session_expired') {
    return null;
  }
  return {
    iconText: '!',
    headline: 'Please sign in again',
    detail: 'Your session needs to be refreshed after a long period of inactivity.',
    statusLabel: 'Session expired'
  };
}

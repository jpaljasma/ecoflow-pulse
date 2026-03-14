import type { CurrentUser, CurrentUserBootstrap } from '@/features/profile/api';

export function resolveUserDisplayName(user: Pick<CurrentUser, 'displayName' | 'givenName' | 'familyName' | 'email'>): string {
  const explicit = user.displayName.trim();
  if (explicit) {
    return explicit;
  }
  const fullName = [user.givenName.trim(), user.familyName.trim()].filter(Boolean).join(' ').trim();
  if (fullName) {
    return fullName;
  }
  return user.email.trim();
}

export function formatAuthMethodLabel(value: string): string {
  const normalized = value.trim().toLowerCase();
  switch (normalized) {
    case 'google':
      return 'Google';
    case 'facebook':
      return 'Facebook';
    case 'password':
      return 'Password';
    case 'keycloak':
    case 'local':
      return 'Pulse account';
    case '':
      return 'Pulse account';
    default:
      return normalized
        .split(/[-_\s]+/)
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(' ');
  }
}

export function mergeCurrentUserBootstrap(
  previous: CurrentUserBootstrap | undefined,
  user: CurrentUser
): CurrentUserBootstrap {
  if (!previous) {
    return {
      user,
      authorization: {
        roles: [],
        deviceCount: 0
      }
    };
  }
  return {
    ...previous,
    user
  };
}

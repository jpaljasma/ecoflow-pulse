import { buildReturnTo } from '@/features/auth/useReturnTo';

export type RequireAuthDecision = {
  allowed: boolean;
  waiting: boolean;
  redirectTo: null | {
    pathname: '/login';
    params: {
      returnTo: string;
    };
  };
};

export function getRequireAuthDecision(input: {
  authConfigured: boolean;
  authReady: boolean;
  sessionValid: boolean;
  pathname: string;
  params: Record<string, string | string[] | undefined>;
}): RequireAuthDecision {
  const { authConfigured, authReady, sessionValid, pathname, params } = input;
  const allowed = !authConfigured || (authReady && sessionValid);
  const waiting = authConfigured && !authReady;

  if (!authConfigured || !authReady || sessionValid) {
    return {
      allowed,
      waiting,
      redirectTo: null
    };
  }

  return {
    allowed,
    waiting,
    redirectTo: {
      pathname: '/login',
      params: {
        returnTo: buildReturnTo(pathname, params)
      }
    }
  };
}

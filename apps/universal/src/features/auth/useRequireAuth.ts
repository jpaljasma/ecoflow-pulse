import { useEffect } from 'react';
import { useLocalSearchParams, usePathname, useRouter } from 'expo-router';
import { getRequireAuthDecision } from '@/features/auth/guard';
import { useAuthSession } from '@/features/auth/hooks';

export function useRequireAuth() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useLocalSearchParams();
  const { authConfigured, authReady, sessionValid } = useAuthSession();
  const decision = getRequireAuthDecision({
    authConfigured,
    authReady,
    sessionValid,
    pathname,
    params
  });

  useEffect(() => {
    if (!decision.redirectTo) {
      return;
    }
    router.replace(decision.redirectTo);
  }, [decision.redirectTo, router]);

  return {
    authConfigured,
    authReady,
    sessionValid,
    allowed: decision.allowed,
    waiting: decision.waiting
  };
}

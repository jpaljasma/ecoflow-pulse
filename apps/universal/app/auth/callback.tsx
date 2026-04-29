import * as WebBrowser from 'expo-web-browser';
import { useEffect, useState } from 'react';
import { Platform } from 'react-native';
import { useRouter } from 'expo-router';
import { useAuthStore } from '@/features/auth/store';
import { completeFullPageWebAuthRedirect } from '@/features/auth/webPkceRedirect';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';

WebBrowser.maybeCompleteAuthSession();

export default function AuthCallbackScreen() {
  const router = useRouter();
  const [errorText, setErrorText] = useState('');

  useEffect(() => {
    let cancelled = false;
    let fallbackTimer: number | undefined;

    if (Platform.OS !== 'web') {
      router.replace('/login');
      return undefined;
    }

    if (typeof window !== 'undefined' && window.opener) {
      return undefined;
    }

    const completeRedirect = async () => {
      const result = await completeFullPageWebAuthRedirect();
      if (cancelled) return;
      if (result.type === 'success') {
        useAuthStore.getState().setSession({
          issuerUrl: result.issuerUrl,
          clientId: result.clientId,
          token: result.token
        });
        router.replace('/login');
        return;
      }
      if (result.type === 'error') {
        setErrorText(result.message);
        window.setTimeout(() => {
          router.replace('/login');
        }, 1800);
        return;
      }
      fallbackTimer = window.setTimeout(() => {
        router.replace('/login');
      }, 900);
    };

    void completeRedirect();

    return () => {
      cancelled = true;
      if (fallbackTimer) {
        window.clearTimeout(fallbackTimer);
      }
    };
  }, [router]);

  return (
    <BrandedLoadingState
      minHeight={240}
      message={errorText ? `Sign-in failed: ${errorText}` : 'Completing sign-in…'}
    />
  );
}

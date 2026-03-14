import * as WebBrowser from 'expo-web-browser';
import { useEffect } from 'react';
import { Platform } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';

WebBrowser.maybeCompleteAuthSession();

export default function AuthCallbackScreen() {
  const router = useRouter();

  useEffect(() => {
    if (Platform.OS !== 'web') {
      router.replace('/login');
      return;
    }

    if (typeof window !== 'undefined' && window.opener) {
      return;
    }

    const timer = window.setTimeout(() => {
      router.replace('/login');
    }, 900);

    return () => {
      window.clearTimeout(timer);
    };
  }, [router]);

  return <BrandedLoadingState minHeight={240} message="Completing sign-in…" />;
}

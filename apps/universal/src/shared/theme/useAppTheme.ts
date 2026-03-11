import { useEffect, useMemo, useState } from 'react';
import { Platform, useColorScheme } from 'react-native';

import { getThemeSpec, resolveThemeMode, resolveThemeVariant, type ThemeMode } from './catalog';
import { useThemeStore } from './store';

export function useAppTheme() {
  const family = useThemeStore((state) => state.family);
  const hydrated = useThemeStore((state) => state.hydrated);
  const systemMode = useSystemThemeMode();

  return useMemo(() => {
    const variant = resolveThemeVariant(family, systemMode);
    const spec = getThemeSpec(variant);
    return {
      hydrated,
      family,
      mode: systemMode,
      variant,
      spec,
      isDark: systemMode === 'dark'
    };
  }, [family, hydrated, systemMode]);
}

function useSystemThemeMode(): ThemeMode {
  const nativeColorScheme = useColorScheme();
  const [webMode, setWebMode] = useState<ThemeMode>(() => getBrowserThemeMode());

  useEffect(() => {
    if (Platform.OS !== 'web' || typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const updateMode = () => {
      setWebMode(mediaQuery.matches ? 'dark' : 'light');
    };

    updateMode();
    if (typeof mediaQuery.addEventListener === 'function') {
      mediaQuery.addEventListener('change', updateMode);
      return () => mediaQuery.removeEventListener('change', updateMode);
    }

    mediaQuery.addListener(updateMode);
    return () => mediaQuery.removeListener(updateMode);
  }, []);

  if (Platform.OS === 'web') {
    return webMode;
  }

  return resolveThemeMode(nativeColorScheme);
}

function getBrowserThemeMode(): ThemeMode {
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return 'dark';
}

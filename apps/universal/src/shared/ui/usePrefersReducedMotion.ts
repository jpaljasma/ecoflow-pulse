import { useEffect, useState } from 'react';
import { AccessibilityInfo, Platform } from 'react-native';

export function usePrefersReducedMotion(): boolean {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

  useEffect(() => {
    if (Platform.OS === 'web' && typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      const query = window.matchMedia('(prefers-reduced-motion: reduce)');
      setPrefersReducedMotion(query.matches);
      const onChange = (event: MediaQueryListEvent) => setPrefersReducedMotion(event.matches);
      query.addEventListener?.('change', onChange);
      return () => query.removeEventListener?.('change', onChange);
    }

    let mounted = true;
    AccessibilityInfo.isReduceMotionEnabled()
      .then((enabled) => {
        if (mounted) setPrefersReducedMotion(enabled);
      })
      .catch(() => {
        if (mounted) setPrefersReducedMotion(false);
      });
    const subscription = AccessibilityInfo.addEventListener?.('reduceMotionChanged', setPrefersReducedMotion);
    return () => {
      mounted = false;
      subscription?.remove?.();
    };
  }, []);

  return prefersReducedMotion;
}

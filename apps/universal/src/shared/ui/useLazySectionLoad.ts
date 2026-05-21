import { useCallback, useEffect, useState, type RefCallback } from 'react';
import { Platform, type View } from 'react-native';

export function useLazySectionLoad({
  rootMargin = '0px',
  threshold = 0.01
}: {
  rootMargin?: string;
  threshold?: number;
} = {}) {
  const [target, setTarget] = useState<Element | null>(null);
  const [shouldLoad, setShouldLoad] = useState(
    () => Platform.OS !== 'web' || typeof IntersectionObserver === 'undefined'
  );
  const ref = useCallback<RefCallback<View>>((node) => {
    setTarget(node as unknown as Element | null);
  }, []);

  useEffect(() => {
    if (shouldLoad || Platform.OS !== 'web' || typeof IntersectionObserver === 'undefined') {
      return;
    }

    if (!target) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setShouldLoad(true);
          observer.disconnect();
        }
      },
      { rootMargin, threshold }
    );
    observer.observe(target);

    return () => {
      observer.disconnect();
    };
  }, [rootMargin, shouldLoad, target, threshold]);

  return { ref, shouldLoad };
}

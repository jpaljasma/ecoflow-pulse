import { memo } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { Image as ExpoImage } from 'expo-image';
import { Image as RNImage, Platform } from 'react-native';
import type { StyleProp, ImageStyle } from 'react-native';

export type CachedImageProps = {
  uri: string;
  style?: StyleProp<ImageStyle>;
  contentFit?: 'contain' | 'cover' | 'fill' | 'none' | 'scale-down';
  onError?: () => void;
};

const webObjectUrlCache = new Map<string, string>();
const webPendingCache = new Map<string, Promise<string>>();
const debugStats = {
  hits: 0,
  misses: 0,
  pendingHits: 0
};

function reportDebugStats() {
  if (Platform.OS !== 'web') return;
  const w = globalThis as any;
  w.__pulseImageCacheStats = {
    ...debugStats,
    entries: webObjectUrlCache.size,
    pending: webPendingCache.size
  };
}

async function getWebCachedUri(uri: string): Promise<string> {
  const cached = webObjectUrlCache.get(uri);
  if (cached) {
    debugStats.hits += 1;
    reportDebugStats();
    return cached;
  }

  const pending = webPendingCache.get(uri);
  if (pending) {
    debugStats.pendingHits += 1;
    reportDebugStats();
    return pending;
  }

  debugStats.misses += 1;
  reportDebugStats();

  const fetchPromise = fetch(uri, { cache: 'force-cache' })
    .then(async (res) => {
      if (!res.ok) {
        throw new Error(`failed to fetch image: ${res.status}`);
      }
      const blob = await res.blob();
      const objectUrl = URL.createObjectURL(blob);
      webObjectUrlCache.set(uri, objectUrl);
      reportDebugStats();
      return objectUrl;
    })
    .finally(() => {
      webPendingCache.delete(uri);
    });

  webPendingCache.set(uri, fetchPromise);
  return fetchPromise;
}

export const CachedImage = memo(function CachedImage({
  uri,
  style,
  contentFit = 'cover',
  onError
}: CachedImageProps) {
  const expoStyle = style as any;
  const isRemoteUri = useMemo(() => /^https?:\/\//i.test(uri), [uri]);
  const [resolvedUri, setResolvedUri] = useState(() => {
    if (Platform.OS !== 'web' || !isRemoteUri) return uri;
    return webObjectUrlCache.get(uri) ?? uri;
  });

  useEffect(() => {
    let cancelled = false;
    if (Platform.OS !== 'web' || !isRemoteUri) {
      setResolvedUri(uri);
      return () => {
        cancelled = true;
      };
    }

    const cached = webObjectUrlCache.get(uri);
    if (cached) {
      setResolvedUri(cached);
      return () => {
        cancelled = true;
      };
    }

    void getWebCachedUri(uri)
      .then((next) => {
        if (!cancelled) {
          setResolvedUri(next);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setResolvedUri(uri);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [uri, isRemoteUri]);

  return (
    Platform.OS === 'web' ? (
      <RNImage
        source={{ uri: resolvedUri }}
        style={style}
        resizeMode={contentFit === 'contain' ? 'contain' : 'cover'}
        onError={onError}
      />
    ) : (
      <ExpoImage
        source={resolvedUri}
        style={expoStyle}
        contentFit={contentFit}
        cachePolicy="memory-disk"
        transition={120}
        onError={onError}
      />
    )
  );
});

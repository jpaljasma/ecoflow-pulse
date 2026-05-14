export type BffCacheResult = 'bypass' | 'coalesced' | 'error' | 'hit' | 'miss';

type CacheEntry<T> = {
  expiresAtMs: number;
  value: T;
};

type BffCacheOptions = {
  enabled: boolean;
  maxEntries: number;
  now?: () => number;
  observe?: (input: { namespace: string; result: BffCacheResult }) => void;
};

export type BffResponseCache = {
  getOrLoad<T>(
    namespace: string,
    key: string,
    ttlMs: number,
    loader: () => Promise<T>
  ): Promise<T>;
  clear(): void;
};

export function createBffCache(options: BffCacheOptions): BffResponseCache {
  const entries = new Map<string, CacheEntry<unknown>>();
  const inflight = new Map<string, Promise<unknown>>();
  const maxEntries = Math.max(1, Math.floor(options.maxEntries));
  const now = options.now ?? Date.now;

  const observe = (namespace: string, result: BffCacheResult) => {
    options.observe?.({ namespace, result });
  };

  const cache: BffResponseCache = {
    async getOrLoad<T>(
      namespace: string,
      key: string,
      ttlMs: number,
      loader: () => Promise<T>
    ): Promise<T> {
      if (!options.enabled || ttlMs <= 0) {
        observe(namespace, 'bypass');
        return await loader();
      }

      const entryKey = `${namespace}:${key}`;
      const currentTime = now();
      const existing = entries.get(entryKey);
      if (existing && existing.expiresAtMs > currentTime) {
        observe(namespace, 'hit');
        return cloneValue(existing.value as T);
      }
      if (existing) {
        entries.delete(entryKey);
      }

      const pending = inflight.get(entryKey);
      if (pending) {
        observe(namespace, 'coalesced');
        return cloneValue((await pending) as T);
      }

      observe(namespace, 'miss');
      const load = loader()
        .then((value) => {
          const stored = cloneValue(value);
          pruneExpired(entries, now());
          entries.set(entryKey, {
            expiresAtMs: now() + ttlMs,
            value: stored
          });
          evictOldest(entries, maxEntries);
          return stored;
        })
        .catch((error) => {
          observe(namespace, 'error');
          throw error;
        })
        .finally(() => {
          inflight.delete(entryKey);
        });
      inflight.set(entryKey, load);
      return cloneValue((await load) as T);
    },
    clear(): void {
      entries.clear();
      inflight.clear();
    }
  };

  return cache;
}

function pruneExpired(entries: Map<string, CacheEntry<unknown>>, nowMs: number): void {
  for (const [key, entry] of entries) {
    if (entry.expiresAtMs <= nowMs) {
      entries.delete(key);
    }
  }
}

function evictOldest(entries: Map<string, CacheEntry<unknown>>, maxEntries: number): void {
  while (entries.size > maxEntries) {
    const oldestKey = entries.keys().next().value as string | undefined;
    if (!oldestKey) {
      return;
    }
    entries.delete(oldestKey);
  }
}

function cloneValue<T>(value: T): T {
  return structuredClone(value);
}

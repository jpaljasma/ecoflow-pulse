# Valkey Cache Layer Backend Go Memory

- Use `internal/hashutil.XXH3Hex128` for cache digests.
- Canonical key shape: `prefix:namespace:{partition}:xxh3-128:<digest>`.
- Prefer versioned tag invalidation over reverse-index scanning.
- Compress above 4 KiB with S2 only when the stored payload shrinks.
- Encrypt sensitive provider-session payloads with AES-GCM and bypass the cache
  when encryption is not configured.
- Preserve in-memory fallbacks for services that do not have Valkey configured.

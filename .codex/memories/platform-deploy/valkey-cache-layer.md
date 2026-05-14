# Valkey Cache Layer Platform Deploy Memory

- Preserve the existing Valkey replication + Sentinel deployment in phase 1.
- Add configuration only for cache-client local TTL/client-side-cache behavior
  and cache encryption keys.
- Do not switch to Valkey Cluster or managed cache without a later ADR and
  scale trigger.
- Document that legacy lease/script clients keep client-side caching disabled.

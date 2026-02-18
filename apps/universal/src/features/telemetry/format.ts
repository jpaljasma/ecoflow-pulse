export function formatW(value: number | undefined | null): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  const abs = Math.abs(value);
  if (abs >= 1000) {
    return `${(value / 1000).toFixed(2)} kW`;
  }
  return `${value.toFixed(0)} W`;
}

export function formatSoc(value: number | undefined | null): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return `${value.toFixed(1)}%`;
}

export function formatAgo(ts: number | null): string {
  if (!ts) return 'never';
  const diff = Math.max(0, Date.now() - ts);
  if (diff < 1000) return '< 1s ago';
  if (diff < 60_000) return `${Math.floor(diff / 1000)}s ago`;
  return `${Math.floor(diff / 60_000)}m ago`;
}

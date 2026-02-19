function formatScaled(
  value: number,
  baseUnit: string,
  kiloUnit: string
): string {
  if (Math.abs(value) < 1000) {
    return `${Math.round(value)} ${baseUnit}`;
  }
  return `${(value / 1000).toFixed(2)} ${kiloUnit}`;
}

export function formatW(value: number | undefined | null): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return formatScaled(value, 'W', 'kW');
}

export function formatKWh(value: number | undefined | null): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  const wh = value * 1000;
  if (Math.abs(wh) < 1000) {
    return `${Math.round(wh)} Wh`;
  }
  return `${value.toFixed(2)} kWh`;
}

export function formatWhAndKWh(valueWh: number | undefined | null): string {
  if (valueWh === null || valueWh === undefined || Number.isNaN(valueWh)) {
    return '—';
  }
  return formatScaled(Math.max(0, valueWh), 'Wh', 'kWh');
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

export function formatEtaMinutes(minutes: number | undefined | null): string {
  if (minutes === null || minutes === undefined || Number.isNaN(minutes)) {
    return '—';
  }
  const total = Math.max(0, Math.round(minutes));
  const d = Math.floor(total / (24 * 60));
  const h = Math.floor((total % (24 * 60)) / 60);
  const m = total % 60;
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

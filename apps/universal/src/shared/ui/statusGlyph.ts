export type UiStatus =
  | 'waiting'
  | 'idle'
  | 'processing'
  | 'loading'
  | 'online'
  | 'stale'
  | 'charging'
  | 'discharging';

export function getStatusGlyph(status: UiStatus): string {
  switch (status) {
    case 'waiting':
      return '⟳';
    case 'idle':
      return '◌';
    case 'processing':
      return '⚙';
    case 'loading':
      return '⟳';
    case 'online':
      return '●';
    case 'stale':
      return '◒';
    case 'charging':
      return '↗';
    case 'discharging':
      return '↘';
    default:
      return '•';
  }
}

export function getPowerFlowGlyph(params: {
  stale?: boolean;
  status?: 'charging' | 'discharging' | 'idle' | 'stale';
  pvW?: number;
  loadW?: number;
}): string {
  if (params.stale || params.status === 'stale') return getStatusGlyph('stale');

  if (params.status === 'charging') {
    const pvW = params.pvW ?? 0;
    const loadW = params.loadW ?? 0;

    if (pvW > 20 && loadW > 20) return '☀⚡'; // hybrid (solar + AC path active)
    if (pvW > 20) return '☀'; // solar charging
    return '⚡'; // AC charging fallback
  }

  if (params.status === 'discharging') {
    return '🔌⎓⌁'; // discharging to AC / DC / USB loads
  }

  if (params.status === 'idle') return getStatusGlyph('idle');
  return getStatusGlyph('waiting');
}

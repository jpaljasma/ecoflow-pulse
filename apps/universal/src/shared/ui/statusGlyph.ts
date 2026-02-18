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
      return '◯';
    case 'processing':
      return '⚙';
    case 'loading':
      return '⟳';
    case 'online':
      return '●';
    case 'stale':
      return '◔';
    case 'charging':
      return '⚡';
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
    if (pvW > 5) return '☀⚡'; // charging with solar present
    return '⚡'; // charging, likely AC or non-PV source
  }

  if (params.status === 'discharging') {
    return '↘'; // discharging
  }

  if (params.status === 'idle') return getStatusGlyph('idle');
  return getStatusGlyph('waiting');
}

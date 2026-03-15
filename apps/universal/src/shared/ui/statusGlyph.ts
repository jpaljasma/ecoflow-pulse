import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';

export type UiStatus =
  | 'waiting'
  | 'idle'
  | 'processing'
  | 'loading'
  | 'online'
  | 'stale'
  | 'charging'
  | 'discharging';

export function getStatusIconName(status: UiStatus): ComponentProps<typeof MaterialCommunityIcons>['name'] {
  switch (status) {
    case 'waiting':
    case 'loading':
      return 'sync';
    case 'idle':
      return 'radiobox-blank';
    case 'processing':
      return 'cog-outline';
    case 'online':
      return 'checkbox-blank-circle';
    case 'stale':
      return 'progress-clock';
    case 'charging':
      return 'lightning-bolt';
    case 'discharging':
      return 'arrow-bottom-right';
    default:
      return 'circle-medium';
  }
}

export function getPowerFlowIconNames(params: {
  stale?: boolean;
  status?: 'charging' | 'discharging' | 'idle' | 'stale';
  pvW?: number;
  loadW?: number;
}): Array<ComponentProps<typeof MaterialCommunityIcons>['name']> {
  if (params.stale || params.status === 'stale') return [getStatusIconName('stale')];

  if (params.status === 'charging') {
    const pvW = params.pvW ?? 0;
    if (pvW > 5) return ['white-balance-sunny', 'lightning-bolt'];
    return ['lightning-bolt'];
  }

  if (params.status === 'discharging') {
    return ['arrow-bottom-right'];
  }

  if (params.status === 'idle') return [getStatusIconName('idle')];
  return [getStatusIconName('waiting')];
}

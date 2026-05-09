import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';

export type PulsePrimaryNavKey = 'devices' | 'energy' | 'energy-calendar' | 'settings' | 'search' | 'about';

export type PulsePrimaryNavItem = {
  key: PulsePrimaryNavKey;
  label: string;
  href: string;
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
};

export const pulsePrimaryNavItems: PulsePrimaryNavItem[] = [
  {
    key: 'devices',
    label: 'Devices',
    href: '/(tabs)/devices',
    icon: 'view-dashboard-outline'
  },
  {
    key: 'energy',
    label: 'Energy',
    href: '/(tabs)/energy',
    icon: 'lightning-bolt-outline'
  },
  {
    key: 'energy-calendar',
    label: 'Calendar',
    href: '/(tabs)/energy-calendar',
    icon: 'calendar-month-outline'
  },
  {
    key: 'settings',
    label: 'Settings',
    href: '/(tabs)/settings',
    icon: 'tune-variant'
  },
  {
    key: 'search',
    label: 'Search',
    href: '/(tabs)/search',
    icon: 'magnify'
  },
  {
    key: 'about',
    label: 'About',
    href: '/(tabs)/about',
    icon: 'information-outline'
  }
];

export function resolvePulsePrimaryNavKey(routeName: string): PulsePrimaryNavKey {
  switch (routeName) {
    case 'energy':
    case 'energy-calendar':
    case 'settings':
    case 'search':
    case 'about':
      return routeName;
    case 'devices':
    default:
      return 'devices';
  }
}

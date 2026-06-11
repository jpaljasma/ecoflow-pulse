import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import {
  canAccessPulseLogs,
  isPulseGlobalAdmin,
  type PulseLogAccessInput
} from '@/shared/authz/pulseRoles';

export { canAccessPulseLogs, isPulseGlobalAdmin };

export type PulsePrimaryNavKey = 'devices' | 'energy' | 'energy-calendar' | 'logs' | 'settings' | 'search' | 'about';

export type PulsePrimaryNavItem = {
  key: PulsePrimaryNavKey;
  label: string;
  href: string;
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  adminOnly?: boolean;
  hiddenFromNav?: boolean;
  webDocumentHref?: string;
};

export type PulsePrimaryNavPressTarget = {
  mode: 'router' | 'document';
  href: string;
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
    key: 'logs',
    label: 'Logs',
    href: '/(tabs)/logs',
    webDocumentHref: '/logs',
    icon: 'text-box-search-outline',
    adminOnly: true
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
    icon: 'magnify',
    hiddenFromNav: true
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
    case 'logs':
    case 'settings':
    case 'search':
    case 'about':
      return routeName;
    case 'devices':
    default:
      return 'devices';
  }
}

export function filterPulsePrimaryNavItems(access: PulseLogAccessInput): PulsePrimaryNavItem[] {
  const logsAccess = canAccessPulseLogs(access);
  return pulsePrimaryNavItems.filter((item) => !item.hiddenFromNav && (!item.adminOnly || logsAccess));
}

export function resolvePulsePrimaryNavPressTarget(item: PulsePrimaryNavItem, platformOS: string): PulsePrimaryNavPressTarget {
  if (platformOS === 'web' && item.webDocumentHref) {
    return { mode: 'document', href: item.webDocumentHref };
  }
  return { mode: 'router', href: item.href };
}

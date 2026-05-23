import type { ComponentProps } from 'react';
import { Tabs } from 'expo-router';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { PulseSidebarNav } from '@/shared/ui/PulseSidebarNav';
import { resolvePulsePrimaryNavKey } from '@/shared/ui/pulsePrimaryNav';

type PulseTabBarProps = Parameters<NonNullable<ComponentProps<typeof Tabs>['tabBar']>>[0];

export function PulseTabBar(props: PulseTabBarProps) {
  const { isSidebarMode } = useNavigationShellMetrics();

  if (!isSidebarMode) {
    return null;
  }

  const currentRoute = props.state.routes[props.state.index];
  return <PulseSidebarNav activeKey={resolvePulsePrimaryNavKey(currentRoute?.name ?? 'devices')} />;
}

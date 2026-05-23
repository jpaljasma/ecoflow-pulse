import { BottomTabBar, type BottomTabBarProps } from 'expo-router/build/react-navigation/bottom-tabs';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { PulseSidebarNav } from '@/shared/ui/PulseSidebarNav';
import { resolvePulsePrimaryNavKey } from '@/shared/ui/pulsePrimaryNav';

export function PulseTabBar(props: BottomTabBarProps) {
  const { isSidebarMode } = useNavigationShellMetrics();

  if (!isSidebarMode) {
    return <BottomTabBar {...props} />;
  }

  const currentRoute = props.state.routes[props.state.index];
  return <PulseSidebarNav activeKey={resolvePulsePrimaryNavKey(currentRoute?.name ?? 'devices')} />;
}

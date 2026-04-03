import { BottomTabBar, type BottomTabBarProps } from '@react-navigation/bottom-tabs';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { PulseSidebarNav, resolvePulsePrimaryNavKey } from '@/shared/ui/PulseSidebarNav';

export function PulseTabBar(props: BottomTabBarProps) {
  const { isSidebarMode } = useNavigationShellMetrics();

  if (!isSidebarMode) {
    return <BottomTabBar {...props} />;
  }

  const currentRoute = props.state.routes[props.state.index];
  return <PulseSidebarNav activeKey={resolvePulsePrimaryNavKey(currentRoute?.name ?? 'devices')} />;
}

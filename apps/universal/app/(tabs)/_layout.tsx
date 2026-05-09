import { Tabs } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { pulsePrimaryNavItems } from '@/shared/ui/pulsePrimaryNav';
import { PulseTabBar } from '@/shared/ui/PulseTabBar';

export default function TabsLayout() {
  const { spec, isDark } = useAppTheme();
  const semantics = useThemeSemantics();
  const { isSidebarMode, sidebarWidth } = useNavigationShellMetrics();
  const activeTint = spec.semantic.solar;
  const inactiveTint = isDark ? spec.colors.colorMuted : spec.colors.borderColor;

  return (
    <Tabs
      tabBar={(props) => <PulseTabBar {...props} />}
      screenOptions={{
        headerShown: false,
        sceneStyle: {
          backgroundColor: spec.colors.background
        },
        tabBarActiveTintColor: activeTint,
        tabBarInactiveTintColor: inactiveTint,
        tabBarPosition: isSidebarMode ? 'left' : 'bottom',
        tabBarVariant: isSidebarMode ? 'material' : 'uikit',
        tabBarLabelPosition: isSidebarMode ? 'beside-icon' : 'below-icon',
        tabBarLabelStyle: {
          fontSize: 11,
          fontWeight: '700'
        },
        tabBarStyle: isSidebarMode
          ? {
              width: sidebarWidth,
              backgroundColor: semantics.railBackground,
              borderRightColor: semantics.railBorder
            }
          : {
              height: 64,
              backgroundColor: spec.colors.background,
              borderTopColor: spec.colors.borderColor,
              paddingTop: 8,
              paddingBottom: 10
            }
      }}
    >
      {pulsePrimaryNavItems.map((item) => (
        <Tabs.Screen
          key={item.key}
          name={item.key}
          options={{
            title: item.label,
            tabBarButtonTestID: `tab-${item.key}`,
            tabBarIcon: ({ color, size }) => (
              <MaterialCommunityIcons name={item.icon} size={size} color={color} />
            )
          }}
        />
      ))}
    </Tabs>
  );
}

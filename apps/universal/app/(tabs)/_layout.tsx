import { Tabs } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
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
      <Tabs.Screen
        name="devices"
        options={{
          title: 'Devices',
          tabBarButtonTestID: 'tab-devices',
          tabBarIcon: ({ color, size }) => (
            <MaterialCommunityIcons name="view-dashboard-outline" size={size} color={color} />
          )
        }}
      />
      <Tabs.Screen
        name="energy"
        options={{
          title: 'Energy',
          tabBarButtonTestID: 'tab-energy',
          tabBarIcon: ({ color, size }) => (
            <MaterialCommunityIcons name="lightning-bolt-outline" size={size} color={color} />
          )
        }}
      />
      <Tabs.Screen
        name="settings"
        options={{
          title: 'Settings',
          tabBarButtonTestID: 'tab-settings',
          tabBarIcon: ({ color, size }) => (
            <MaterialCommunityIcons name="tune-variant" size={size} color={color} />
          )
        }}
      />
      <Tabs.Screen
        name="search"
        options={{
          title: 'Search',
          tabBarButtonTestID: 'tab-search',
          tabBarIcon: ({ color, size }) => (
            <MaterialCommunityIcons name="magnify" size={size} color={color} />
          )
        }}
      />
      <Tabs.Screen
        name="about"
        options={{
          title: 'About',
          tabBarButtonTestID: 'tab-about',
          tabBarIcon: ({ color, size }) => (
            <MaterialCommunityIcons name="information-outline" size={size} color={color} />
          )
        }}
      />
    </Tabs>
  );
}

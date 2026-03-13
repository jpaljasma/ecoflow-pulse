import { Image } from 'react-native';
import { Tabs } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import appIcon from '../../assets/icon.png';

export default function TabsLayout() {
  const { spec, isDark } = useAppTheme();
  const activeTint = spec.semantic.solar;
  const inactiveTint = isDark ? spec.colors.colorMuted : spec.colors.borderColor;
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: activeTint,
        tabBarInactiveTintColor: inactiveTint,
        tabBarLabelStyle: {
          fontSize: 11,
          fontWeight: '700'
        },
        tabBarStyle: {
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
          tabBarIcon: ({ size, focused }) => (
            <Image
              source={appIcon}
              style={{
                width: size,
                height: size,
                opacity: focused ? 1 : 0.82
              }}
              resizeMode="contain"
            />
          )
        }}
      />
    </Tabs>
  );
}

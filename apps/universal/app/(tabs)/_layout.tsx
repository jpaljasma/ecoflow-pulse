import { Tabs } from 'expo-router';

export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarStyle: {
          height: 64,
          paddingTop: 8,
          paddingBottom: 10
        }
      }}
    >
      <Tabs.Screen
        name="devices"
        options={{ title: 'Devices', tabBarButtonTestID: 'tab-devices' }}
      />
      <Tabs.Screen
        name="settings"
        options={{ title: 'Settings', tabBarButtonTestID: 'tab-settings' }}
      />
      <Tabs.Screen
        name="search"
        options={{ title: 'Search', tabBarButtonTestID: 'tab-search' }}
      />
      <Tabs.Screen
        name="about"
        options={{ title: 'About', tabBarButtonTestID: 'tab-about' }}
      />
    </Tabs>
  );
}

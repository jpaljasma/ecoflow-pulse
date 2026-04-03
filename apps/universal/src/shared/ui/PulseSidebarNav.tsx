import { useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Image, Pressable, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import type { ComponentProps } from 'react';
import appIcon from '../../../assets/icon.png';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { useNavigationShellStore } from '@/shared/ui/navigationShellStore';

type PulsePrimaryNavItem = {
  key: PulsePrimaryNavKey;
  label: string;
  href: string;
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
};

export type PulsePrimaryNavKey = 'devices' | 'energy' | 'settings' | 'search' | 'about';

const pulsePrimaryNavItems: PulsePrimaryNavItem[] = [
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

export function PulseSidebarNav({
  activeKey
}: {
  activeKey: PulsePrimaryNavKey;
}) {
  const router = useRouter();
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const { isSidebarMode, sidebarExpanded, sidebarWidth } = useNavigationShellMetrics();
  const toggleSidebarExpanded = useNavigationShellStore((state) => state.toggleSidebarExpanded);

  if (!isSidebarMode) {
    return null;
  }

  return (
    <View
      style={{
        width: sidebarWidth,
        paddingTop: 18,
        paddingBottom: 16,
        paddingHorizontal: sidebarExpanded ? 16 : 10,
        backgroundColor: semantics.railBackground,
        borderRightWidth: 1,
        borderRightColor: semantics.railBorder
      }}
    >
      <YStack flex={1} gap="$4">
        {sidebarExpanded ? (
          <XStack alignItems="center" justifyContent="space-between" gap="$3">
            <Pressable
              onPress={() => router.replace('/(tabs)/devices')}
              accessibilityRole="button"
              accessibilityLabel="Go to Pulse home"
            >
              <XStack
                alignItems="center"
                gap="$3"
                paddingVertical="$2"
                paddingHorizontal="$2"
              >
                <YStack
                  width={42}
                  height={42}
                  borderRadius={16}
                  alignItems="center"
                  justifyContent="center"
                  borderWidth={1}
                  style={{
                    backgroundColor: semantics.navBrandBackground,
                    borderColor: semantics.navBrandBorder
                  }}
                >
                  <Image
                    source={appIcon}
                    style={{ width: 30, height: 30, borderRadius: 9 }}
                    resizeMode="contain"
                  />
                </YStack>
                <YStack gap={2}>
                  <Text fontSize="$6" fontWeight="800" letterSpacing={-0.3}>
                    Pulse
                  </Text>
                  <Text
                    fontSize="$2"
                    textTransform="uppercase"
                    letterSpacing={0.7}
                    style={{ color: semantics.navSectionLabel }}
                  >
                    Workspace
                  </Text>
                </YStack>
              </XStack>
            </Pressable>

            <Pressable
              onPress={toggleSidebarExpanded}
              accessibilityRole="button"
              accessibilityLabel="Collapse sidebar"
              style={({ pressed }) => ({
                width: 40,
                height: 40,
                borderRadius: 14,
                alignItems: 'center',
                justifyContent: 'center',
                borderWidth: 1,
                backgroundColor: semantics.navToggleBackground,
                borderColor: semantics.navToggleBorder,
                opacity: pressed ? 0.84 : 1
              })}
            >
              <MaterialCommunityIcons name="dock-left" size={18} color={spec.colors.color} />
            </Pressable>
          </XStack>
        ) : (
          <YStack alignItems="center" gap="$3">
            <Pressable
              onPress={() => router.replace('/(tabs)/devices')}
              accessibilityRole="button"
              accessibilityLabel="Go to Pulse home"
            >
              <YStack
                width={52}
                height={52}
                borderRadius={18}
                alignItems="center"
                justifyContent="center"
                borderWidth={1}
                style={{
                  backgroundColor: semantics.navBrandBackground,
                  borderColor: semantics.navBrandBorder
                }}
              >
                <Image
                  source={appIcon}
                  style={{ width: 34, height: 34, borderRadius: 10 }}
                  resizeMode="contain"
                />
              </YStack>
            </Pressable>

            <Pressable
              onPress={toggleSidebarExpanded}
              accessibilityRole="button"
              accessibilityLabel="Expand sidebar"
              style={({ pressed }) => ({
                width: 44,
                height: 44,
                borderRadius: 16,
                alignItems: 'center',
                justifyContent: 'center',
                borderWidth: 1,
                backgroundColor: semantics.navToggleBackground,
                borderColor: semantics.navToggleBorder,
                opacity: pressed ? 0.84 : 1
              })}
            >
              <MaterialCommunityIcons name="dock-right" size={18} color={spec.colors.color} />
            </Pressable>
          </YStack>
        )}

        <YStack flex={1} gap="$2">
          {sidebarExpanded ? (
            <Text
              fontSize="$2"
              fontWeight="700"
              textTransform="uppercase"
              letterSpacing={0.8}
              paddingHorizontal="$2"
              paddingTop="$1"
              style={{ color: semantics.navSectionLabel }}
            >
              Navigate
            </Text>
          ) : null}

          {pulsePrimaryNavItems.map((item) => {
            const focused = item.key === activeKey;

            return (
              <Pressable
                key={item.key}
                accessibilityRole="button"
                accessibilityState={focused ? { selected: true } : {}}
                accessibilityLabel={item.label}
                testID={`sidebar-${item.key}`}
                onPress={() => {
                  if (!focused) {
                    router.replace(item.href);
                  }
                }}
                style={({ pressed }) => ({
                  minHeight: sidebarExpanded ? 58 : 54,
                  borderRadius: 18,
                  borderWidth: 1,
                  borderColor: focused ? semantics.navItemActiveBorder : 'transparent',
                  backgroundColor: focused ? semantics.navItemActiveBackground : 'transparent',
                  opacity: pressed ? 0.9 : 1,
                  paddingHorizontal: sidebarExpanded ? 14 : 0,
                  paddingVertical: sidebarExpanded ? 8 : 0,
                  justifyContent: 'center',
                  alignItems: sidebarExpanded ? 'stretch' : 'center'
                })}
              >
                <XStack
                  alignItems="center"
                  justifyContent={sidebarExpanded ? 'space-between' : 'center'}
                  gap="$3"
                >
                  <XStack alignItems="center" gap="$3" flex={1}>
                    <YStack
                      width={sidebarExpanded ? 38 : 40}
                      height={sidebarExpanded ? 38 : 40}
                      borderRadius={sidebarExpanded ? 14 : 16}
                      alignItems="center"
                      justifyContent="center"
                      style={{
                        backgroundColor: focused
                          ? semantics.navItemActiveIconBackground
                          : semantics.navItemIdleIconBackground
                      }}
                    >
                      <MaterialCommunityIcons
                        name={item.icon}
                        size={22}
                        color={focused ? semantics.navItemActiveText : semantics.navItemIdleText}
                      />
                    </YStack>

                    {sidebarExpanded ? (
                      <YStack gap={2} flex={1}>
                        <Text
                          fontSize="$4"
                          fontWeight={focused ? '800' : '700'}
                          numberOfLines={1}
                          style={{ color: focused ? semantics.navItemActiveText : semantics.navItemIdleText }}
                        >
                          {item.label}
                        </Text>
                        <Text
                          fontSize="$1"
                          numberOfLines={1}
                          textTransform="uppercase"
                          letterSpacing={0.55}
                          style={{ color: focused ? semantics.navItemActiveSubtleText : semantics.navSectionLabel }}
                        >
                          {focused ? 'Current view' : 'Open view'}
                        </Text>
                      </YStack>
                    ) : null}
                  </XStack>

                  {sidebarExpanded && focused ? (
                    <YStack
                      width={8}
                      height={8}
                      borderRadius={999}
                      style={{ backgroundColor: semantics.navItemIndicator }}
                    />
                  ) : null}
                </XStack>
              </Pressable>
            );
          })}
        </YStack>
      </YStack>
    </View>
  );
}

export function resolvePulsePrimaryNavKey(routeName: string): PulsePrimaryNavKey {
  switch (routeName) {
    case 'energy':
    case 'settings':
    case 'search':
    case 'about':
      return routeName;
    case 'devices':
    default:
      return 'devices';
  }
}

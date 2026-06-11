import { useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Platform, Pressable, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { useNavigationShellStore } from '@/shared/ui/navigationShellStore';
import { PulseMark } from '@/shared/ui/PulseMark';
import {
  filterPulsePrimaryNavItems,
  resolvePulsePrimaryNavPressTarget,
  type PulsePrimaryNavItem,
  type PulsePrimaryNavKey
} from '@/shared/ui/pulsePrimaryNav';
import { useAuthSession } from '@/features/auth/hooks';
import { useCurrentUser } from '@/features/profile/hooks';

export function PulseSidebarNav({ activeKey }: { activeKey: PulsePrimaryNavKey }) {
  const router = useRouter();
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const { isSidebarMode, sidebarExpanded, sidebarWidth } = useNavigationShellMetrics();
  const toggleSidebarExpanded = useNavigationShellStore((state) => state.toggleSidebarExpanded);
  const { authReady, authKey, token } = useAuthSession();
  const currentUserQuery = useCurrentUser({ token, authKey, enabled: authReady });
  const navItems = filterPulsePrimaryNavItems({
    roles: currentUserQuery.data?.authorization.roles,
    deviceCount: currentUserQuery.data?.authorization.deviceCount
  });

  if (!isSidebarMode) {
    return null;
  }

  const openNavItem = (item: PulsePrimaryNavItem) => {
    const target = resolvePulsePrimaryNavPressTarget(item, Platform.OS);
    if (target.mode === 'document' && typeof window !== 'undefined') {
      window.location.assign(target.href);
      return;
    }
    router.replace(target.href);
  };

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
            <Pressable onPress={() => router.replace('/(tabs)/devices')} accessibilityRole="button" accessibilityLabel="Go to Pulse home">
              <XStack alignItems="center" gap="$3" paddingVertical="$2" paddingHorizontal="$2">
                <YStack width={42} height={42} alignItems="center" justifyContent="center" borderRadius={16}>
                  <PulseMark size={38} />
                </YStack>
                <YStack gap={2}>
                  <Text fontSize="$6" fontWeight="800" letterSpacing={0}>
                    Pulse
                  </Text>
                  <Text fontSize="$2" textTransform="uppercase" letterSpacing={0.7} style={{ color: semantics.navSectionLabel }}>
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
            <Pressable onPress={() => router.replace('/(tabs)/devices')} accessibilityRole="button" accessibilityLabel="Go to Pulse home">
              <YStack width={52} height={52} alignItems="center" justifyContent="center" borderRadius={18}>
                <PulseMark size={46} />
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

          {navItems.map((item) => {
            const focused = item.key === activeKey;

            return (
              <Pressable
                key={item.key}
                accessibilityRole="button"
                accessibilityState={focused ? { selected: true } : {}}
                accessibilityLabel={item.label}
                testID={`sidebar-${item.key}`}
                onPress={() => openNavItem(item)}
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
                <XStack alignItems="center" justifyContent={sidebarExpanded ? 'space-between' : 'center'} gap="$3">
                  <XStack alignItems="center" gap="$3" flex={1}>
                    <YStack
                      width={sidebarExpanded ? 38 : 40}
                      height={sidebarExpanded ? 38 : 40}
                      borderRadius={sidebarExpanded ? 14 : 16}
                      alignItems="center"
                      justifyContent="center"
                      style={{
                        backgroundColor: focused ? semantics.navItemActiveIconBackground : semantics.navItemIdleIconBackground
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
                          style={{
                            color: focused ? semantics.navItemActiveText : semantics.navItemIdleText
                          }}
                        >
                          {item.label}
                        </Text>
                        <Text
                          fontSize="$1"
                          numberOfLines={1}
                          textTransform="uppercase"
                          letterSpacing={0.55}
                          style={{
                            color: focused ? semantics.navItemActiveSubtleText : semantics.navSectionLabel
                          }}
                        >
                          {focused ? 'Current view' : 'Open view'}
                        </Text>
                      </YStack>
                    ) : null}
                  </XStack>

                  {sidebarExpanded && focused ? (
                    <YStack width={8} height={8} borderRadius={999} style={{ backgroundColor: semantics.navItemIndicator }} />
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

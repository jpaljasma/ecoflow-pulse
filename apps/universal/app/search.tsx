import { useState } from 'react';
import { Animated, ScrollView } from 'react-native';
import { useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { AppMenu } from '@/shared/ui/AppMenu';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { Card } from '@/shared/ui/Card';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { TopBar } from '@/shared/ui/TopBar';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { XStack, YStack } from 'tamagui';

export default function SearchScreen() {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const semantics = useThemeSemantics();
  const { compactHeader, horizontalPadding, isSidebarMode, layoutMaxWidth } = usePageLayoutMetrics();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  return (
    <Animated.View style={containerStyle} testID="screen-search">
      <YStack flex={1} backgroundColor="$background" paddingHorizontal={horizontalPadding} paddingVertical="$4" gap="$4">
        <TopBar
          left={isSidebarMode ? undefined : <CloseToHomeButton onClose={closeToHome} />}
          eyebrow={(
            <BreadcrumbTrail
              items={[
                {
                  label: 'Home',
                  href: '/(tabs)/devices',
                  icon: 'home-variant-outline',
                  hideLabel: true
                },
                {
                  label: 'Search',
                  current: true
                }
              ]}
            />
          )}
          title="Search"
          subtitle="Search devices and telemetry"
          right={
            <YStack alignItems="flex-end">
              <AppMenu />
            </YStack>
          }
          titleFlex={compactHeader ? 1 : 3}
          rightFlex={compactHeader ? 0 : 1}
        />
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ paddingBottom: 16, alignItems: 'center' }}
          showsVerticalScrollIndicator
        >
          <YStack gap="$4" width="100%" maxWidth={layoutMaxWidth}>
            <Card gap="$3">
              <XStack alignItems="center" gap="$2">
                <AppTextInput
                  flex={1}
                  value={query}
                  onChangeText={setQuery}
                  placeholder="Search"
                  placeholderTextColor={semantics.subtleText}
                />
                <XStack
                  width={52}
                  minHeight={52}
                  alignItems="center"
                  justifyContent="center"
                  borderWidth={1}
                  borderRadius={20}
                  style={{
                    backgroundColor: semantics.sectionBackgroundStrong,
                    borderColor: semantics.sectionBorder
                  }}
                >
                  <MaterialCommunityIcons name="magnify" size={22} color={semantics.subtleStrongText} />
                </XStack>
              </XStack>
            </Card>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}

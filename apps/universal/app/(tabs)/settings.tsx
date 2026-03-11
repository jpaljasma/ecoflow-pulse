import { useRouter } from 'expo-router';
import { Animated, ScrollView, useWindowDimensions } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { KeycloakPkceCard } from '@/features/auth/KeycloakPkceCard';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { themeFamilyOptions } from '@/shared/theme/catalog';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useThemeStore } from '@/shared/theme/store';

export default function SettingsScreen() {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const compactHeader = width < 430;
  const isTablet = width >= 768;
  const isDesktop = width >= 1120;
  const horizontalPadding = isDesktop ? '$7' : isTablet ? '$5' : '$4';
  const layoutMaxWidth = isDesktop ? 1180 : 980;
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const themeFamily = useThemeStore((state) => state.family);
  const setThemeFamily = useThemeStore((state) => state.setFamily);
  const { mode, spec, isDark } = useAppTheme();
  const systemModeLabel = mode === 'dark' ? 'Dark' : 'Light';

  return (
    <Animated.View style={containerStyle} testID="screen-settings">
      <YStack
        flex={1}
        backgroundColor="$background"
        paddingHorizontal={horizontalPadding}
        paddingVertical="$4"
        gap="$4"
      >
        <TopBar
          left={<CloseToHomeButton onClose={closeToHome} />}
          title={<BrandLogo compact={compactHeader} dense onPress={() => router.push('/devices')} />}
          subtitle="Configuration and diagnostics"
          titleFlex={compactHeader ? 1 : 3}
          rightFlex={compactHeader ? 0 : 1}
          right={
            <YStack alignItems="flex-end">
              <AppMenu />
            </YStack>
          }
        />
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ paddingBottom: 16, alignItems: 'center' }}
          showsVerticalScrollIndicator
        >
          <YStack gap="$4" width="100%" maxWidth={layoutMaxWidth}>
            <KeycloakPkceCard />
            <Card
              gap="$4"
              padding={isDesktop ? '$6' : isTablet ? '$5' : '$4'}
              backgroundColor="$backgroundElevated"
            >
              <XStack
                justifyContent="space-between"
                alignItems={isTablet ? 'center' : 'flex-start'}
                gap="$3"
                flexWrap="wrap"
              >
                <YStack gap="$2" maxWidth={760} flex={1}>
                  <Text fontSize={isTablet ? '$6' : '$5'} fontWeight="800" letterSpacing={-0.2}>
                    Appearance
                  </Text>
                  <Text color="$colorMuted" opacity={0.94} fontSize="$3" lineHeight={24}>
                    Choose the palette family. Light and dark mode continue to follow the device appearance.
                  </Text>
                </YStack>
                <XStack
                  alignItems="center"
                  gap="$2"
                  paddingHorizontal="$2"
                  paddingVertical="$1"
                  borderRadius={999}
                  backgroundColor="$backgroundHover"
                  borderWidth={1}
                  borderColor="$borderColor"
                >
                  <YStack width={6} height={6} borderRadius={999} backgroundColor="$accentColor" />
                  <Text fontSize="$2" fontWeight="700" opacity={0.94}>
                    System controlled
                  </Text>
                </XStack>
              </XStack>
              <XStack gap="$3" flexWrap="wrap" alignItems="stretch">
                {themeFamilyOptions.map((option) => {
                  const selected = option.value === themeFamily;
                  const accentColor = option.darkPreview.colors.accentColor;
                  const cardBackground = selected
                    ? hexToRgba(accentColor, isDark ? 0.12 : 0.08)
                    : hexToRgba(spec.colors.backgroundHover, isDark ? 0.58 : 0.72);
                  const cardBorderColor = selected
                    ? hexToRgba(accentColor, isDark ? 0.92 : 0.82)
                    : hexToRgba(spec.colors.borderColor, isDark ? 0.72 : 0.82);
                  const eyebrowColor = selected
                    ? hexToRgba(accentColor, isDark ? 0.88 : 0.82)
                    : hexToRgba(spec.colors.colorMuted, isDark ? 0.82 : 0.9);
                  const bodyColor = selected
                    ? spec.colors.color
                    : hexToRgba(spec.colors.colorMuted, isDark ? 0.92 : 0.96);
                  const statusColor = selected
                    ? accentColor
                    : hexToRgba(spec.colors.colorMuted, isDark ? 0.9 : 0.96);
                  return (
                    <Button
                      key={option.value}
                      unstyled
                      onPress={() => setThemeFamily(option.value)}
                      flexGrow={1}
                      flexBasis={isTablet ? 0 : '100%'}
                      minWidth={isTablet ? 320 : undefined}
                      borderRadius="$3"
                      borderWidth={selected ? 1.5 : 1}
                      padding={isDesktop ? '$4' : '$3'}
                      minHeight={isDesktop ? 164 : 156}
                      justifyContent="flex-start"
                      style={{
                        borderColor: cardBorderColor,
                        backgroundColor: cardBackground,
                        opacity: selected ? 1 : 0.8
                      }}
                      hoverStyle={{
                        y: -2,
                        opacity: 1
                      }}
                      pressStyle={{
                        scale: 0.995
                      }}
                      focusStyle={{
                        outlineWidth: 0
                      }}
                      shadowColor="$shadowColor"
                      shadowOpacity={selected ? 0.12 : 0.03}
                      shadowRadius={selected ? 14 : 6}
                      shadowOffset={{ width: 0, height: selected ? 5 : 1 }}
                    >
                      <YStack flex={1} gap="$2">
                        <XStack justifyContent="space-between" alignItems="center" gap="$3">
                          <YStack gap="$1" flex={1}>
                            <Text fontSize={isDesktop ? '$5' : '$4'} fontWeight="800" letterSpacing={-0.1}>
                              {option.label}
                            </Text>
                            <Text
                              fontSize="$2"
                              textTransform="uppercase"
                              letterSpacing={0.5}
                              style={{ color: eyebrowColor }}
                            >
                              {option.value === 'new' ? 'Recommended' : 'Alternative'}
                            </Text>
                          </YStack>
                          <Text
                            fontSize="$2"
                            fontWeight="700"
                            style={{ color: statusColor }}
                          >
                            {selected ? 'Active' : 'Select'}
                          </Text>
                        </XStack>
                        <Text fontSize="$3" lineHeight={21} maxWidth={440} style={{ color: bodyColor }}>
                          {option.description}
                        </Text>

                        <XStack
                          gap="$2"
                          padding="$1"
                          borderRadius="$3"
                          style={{
                            backgroundColor: hexToRgba(spec.colors.backgroundHover, selected ? 0.92 : 0.58),
                            borderColor: selected
                              ? hexToRgba(accentColor, isDark ? 0.4 : 0.28)
                              : hexToRgba(spec.colors.borderColor, isDark ? 0.4 : 0.52),
                            opacity: selected ? 1 : 0.74
                          }}
                          borderWidth={1}
                        >
                          <ThemeModePreview
                            label="Light"
                            backgroundColor={option.lightPreview.colors.backgroundElevated}
                            borderColor={option.lightPreview.colors.borderColor}
                            textColor={option.lightPreview.colors.color}
                            accentColor={option.lightPreview.colors.accentColor}
                          />
                          <ThemeModePreview
                            label="Dark"
                            backgroundColor={option.darkPreview.colors.backgroundElevated}
                            borderColor={option.darkPreview.colors.borderColor}
                            textColor={option.darkPreview.colors.color}
                            accentColor={option.darkPreview.colors.accentColor}
                          />
                        </XStack>

                        <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
                          <Text fontSize="$3" style={{ color: eyebrowColor }}>
                            Palette family
                          </Text>
                          <Text fontSize="$3" fontWeight="700" style={{ color: selected ? spec.colors.color : bodyColor }}>
                            {option.value === 'new' ? 'Mint energy palette' : 'Current Pulse palette'}
                          </Text>
                        </XStack>
                      </YStack>
                    </Button>
                  );
                })}
              </XStack>
              <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
                <XStack
                  alignItems="center"
                  gap="$2"
                  paddingHorizontal="$2"
                  paddingVertical="$1"
                  borderRadius={999}
                  backgroundColor="$backgroundHover"
                  borderWidth={1}
                  borderColor="$borderColor"
                >
                  <Text fontSize="$2" color="$colorMuted" fontWeight="700">
                    System appearance
                  </Text>
                  <Text fontSize="$2" fontWeight="800">
                    {systemModeLabel}
                  </Text>
                </XStack>
                <Text color="$colorMuted" opacity={0.94} fontSize="$2">
                  Default family is New.
                </Text>
              </XStack>
            </Card>
            <Card gap="$2" padding={isTablet ? '$5' : '$4'}>
              <Text fontSize="$5" fontWeight="700" testID="settings-api-endpoints">
                API Endpoints
              </Text>
              <Text opacity={0.75}>
                Set EXPO_PUBLIC_API_URL for non-default routing. EXPO_PUBLIC_WS_URL is optional and
                defaults to API /ws.
              </Text>
            </Card>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}

function ThemeModePreview({
  label,
  backgroundColor,
  borderColor,
  textColor,
  accentColor
}: {
  label: string;
  backgroundColor: string;
  borderColor: string;
  textColor: string;
  accentColor: string;
}) {
  return (
    <YStack
      flex={1}
      minHeight={74}
      justifyContent="space-between"
      paddingHorizontal="$3"
      paddingVertical="$3"
      borderRadius="$3"
      borderWidth={1}
      style={{ backgroundColor, borderColor }}
    >
      <Text fontSize="$3" fontWeight="700" style={{ color: textColor }}>
        {label}
      </Text>
      <YStack
        width={44}
        height={4}
        borderRadius={999}
        style={{ backgroundColor: accentColor }}
      />
    </YStack>
  );
}

function hexToRgba(hex: string, alpha: number): string {
  const normalized = hex.replace('#', '');
  const value = normalized.length === 3
    ? normalized
        .split('')
        .map((part) => part + part)
        .join('')
    : normalized;
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

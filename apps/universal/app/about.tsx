import { useRouter } from 'expo-router';
import { Animated, ScrollView } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { PulseMark } from '@/shared/ui/PulseMark';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { appMetadata } from '@/shared/theme/catalog';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';

const capabilityItems = [
  {
    title: 'Realtime control room',
    body: 'See solar input, battery state, load flow, and device health in a single operator-grade view.'
  },
  {
    title: 'Energy dashboard',
    body: 'Compare solar, load, battery movement, PV envelope, and estimated value across local-calendar windows for one device or the whole fleet.'
  },
  {
    title: 'Cross-platform by default',
    body: 'One experience across web, iPhone, iPad, and Android without giving up fast telemetry feedback.'
  },
  {
    title: 'Built for clear action',
    body: 'From fleet snapshots to per-device detail, the UI is tuned to surface what matters without noise.'
  }
] as const;

const trustItems = [
  'Live telemetry with system-aware light and dark presentation',
  'Professional device summaries, trends, Energy dashboard views, and solar performance context',
  'Fast local and cloud-ready workflows for modern energy operations'
] as const;

export default function AboutScreen() {
  const router = useRouter();
  const { compactHeader, horizontalPadding, isDesktop, isSidebarMode, isTablet, layoutMaxWidth } = usePageLayoutMetrics();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();

  return (
    <Animated.View style={containerStyle} testID="screen-about">
      <YStack
        flex={1}
        backgroundColor="$background"
        paddingHorizontal={horizontalPadding}
        paddingVertical="$4"
        gap="$4"
      >
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
                  label: 'About',
                  current: true
                }
              ]}
            />
          )}
          title="About Pulse"
          subtitle="Product overview and platform capabilities"
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
            <Card
              gap={isTablet ? '$5' : '$4'}
              padding={isDesktop ? '$6' : isTablet ? '$5' : '$4'}
              style={{
                backgroundColor: semantics.energyCardBackground,
                borderColor: semantics.energyCardBorder
              }}
            >
              <XStack
                gap="$4"
                alignItems={isTablet ? 'center' : 'flex-start'}
                justifyContent="space-between"
                flexWrap="wrap"
              >
                <XStack gap="$4" alignItems={isTablet ? 'center' : 'flex-start'} flexWrap="wrap" flex={1}>
                  <PulseMark size={isTablet ? 112 : 96} />

                  <YStack gap="$3" flex={1} minWidth={280} maxWidth={700}>
                    <YStack gap="$2">
                      <Text
                        fontSize={isDesktop ? '$8' : isTablet ? '$7' : '$6'}
                        fontWeight="800"
                        letterSpacing={-0.4}
                      >
                        {appMetadata.title}
                      </Text>
                      <Text
                        fontSize={isTablet ? '$4' : '$3'}
                        lineHeight={isTablet ? 28 : 24}
                        style={{ color: semantics.subtleStrongText }}
                      >
                        {appMetadata.tagline}
                      </Text>
                    </YStack>

                    <Text
                      fontSize={isTablet ? '$5' : '$4'}
                      lineHeight={isTablet ? 30 : 26}
                      style={{ color: spec.colors.color }}
                    >
                      Pulse turns live device telemetry into a clean energy operations interface.
                      It brings solar, storage, backup power, fleet awareness, and a dedicated Energy
                      dashboard into one refined control room that feels immediate on web and native.
                    </Text>

                    <XStack gap="$2" flexWrap="wrap">
                      <YStack
                        paddingHorizontal="$3"
                        paddingVertical="$2"
                        borderRadius={999}
                        borderWidth={1}
                        style={{
                          backgroundColor: semantics.actionBackground,
                          borderColor: semantics.actionBorder
                        }}
                      >
                        <Text fontSize="$2" fontWeight="700" style={{ color: semantics.actionText }}>
                          Universal app
                        </Text>
                      </YStack>
                      <YStack
                        paddingHorizontal="$3"
                        paddingVertical="$2"
                        borderRadius={999}
                        borderWidth={1}
                        style={{
                          backgroundColor: semantics.solarBadgeBackground,
                          borderColor: semantics.solarBadgeBorder
                        }}
                      >
                        <Text fontSize="$2" fontWeight="700" style={{ color: semantics.solarBadgeTitle }}>
                          Realtime telemetry
                        </Text>
                      </YStack>
                      <YStack
                        paddingHorizontal="$3"
                        paddingVertical="$2"
                        borderRadius={999}
                        borderWidth={1}
                        style={{
                          backgroundColor: semantics.periodActiveBackground,
                          borderColor: semantics.periodActiveBorder
                        }}
                      >
                        <Text fontSize="$2" fontWeight="700" style={{ color: semantics.periodActiveText }}>
                          Energy dashboard
                        </Text>
                      </YStack>
                    </XStack>
                  </YStack>
                </XStack>
              </XStack>
            </Card>

            <XStack gap="$3" flexWrap="wrap" alignItems="stretch">
              {capabilityItems.map((item) => (
                <Card
                  key={item.title}
                  flexGrow={1}
                  flexBasis={isTablet ? 0 : '100%'}
                  minWidth={isTablet ? 250 : undefined}
                  gap="$3"
                  padding={isTablet ? '$4' : '$3'}
                  backgroundColor="$backgroundElevated"
                  style={{
                    borderColor: semantics.sectionBorder
                  }}
                >
                  <YStack
                    width={44}
                    height={44}
                    borderRadius={14}
                    alignItems="center"
                    justifyContent="center"
                    borderWidth={1}
                    style={{
                      backgroundColor: semantics.mutedPanelBackground,
                      borderColor: semantics.mutedPanelBorder
                    }}
                  >
                    <YStack
                      width={18}
                      height={18}
                      borderRadius={999}
                      style={{ backgroundColor: spec.colors.accentColor }}
                    />
                  </YStack>
                  <YStack gap="$2">
                    <Text fontSize="$4" fontWeight="800">
                      {item.title}
                    </Text>
                    <Text fontSize="$3" lineHeight={23} style={{ color: semantics.subtleStrongText }}>
                      {item.body}
                    </Text>
                  </YStack>
                </Card>
              ))}
            </XStack>

            <Card gap="$4" padding={isDesktop ? '$5' : '$4'}>
              <YStack gap="$2">
                <Text fontSize="$6" fontWeight="800" letterSpacing={-0.2}>
                  Why it exists
                </Text>
                <Text fontSize="$3" lineHeight={24} style={{ color: semantics.subtleStrongText }}>
                  Energy telemetry is only useful when it is fast to read, easy to trust, and consistent
                  across the surfaces people already use. Pulse is designed to make live power data
                  operationally clear instead of visually noisy.
                </Text>
              </YStack>

              <YStack gap="$3">
                {trustItems.map((item) => (
                  <XStack
                    key={item}
                    gap="$3"
                    alignItems="flex-start"
                    padding="$3"
                    borderRadius="$3"
                    borderWidth={1}
                    style={{
                      backgroundColor: semantics.mutedPanelBackground,
                      borderColor: semantics.mutedPanelBorder
                    }}
                  >
                    <YStack
                      width={10}
                      height={10}
                      marginTop={6}
                      borderRadius={999}
                      flexShrink={0}
                      style={{ backgroundColor: spec.colors.accentColor }}
                    />
                    <Text fontSize="$3" lineHeight={23} style={{ color: semantics.subtleStrongText }}>
                      {item}
                    </Text>
                  </XStack>
                ))}
              </YStack>
            </Card>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}

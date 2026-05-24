import { router } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Platform } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { buildEnergyRouteParams } from '@/features/energy/model';
import type { SolarOutlook } from '@/features/weather/model';
import {
  formatSolarCapacitySummary,
  formatSolarModelSummary,
  formatSolarOutlookSummary,
  formatSolarProvenanceSummary
} from '@/features/weather/model';
import { Card } from '@/shared/ui/Card';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { buildPulseActionButtonStyles } from '@/shared/ui/buttonInteractions';

export function DeviceSolarForecastCard({
  deviceName,
  deviceId,
  solarOutlook,
  isLoading,
  errorText,
  fill = false
}: {
  deviceName?: string;
  deviceId?: string;
  solarOutlook?: SolarOutlook;
  isLoading?: boolean;
  errorText?: string;
  fill?: boolean;
}) {
  const semantics = useThemeSemantics();
  const actionButtonStyles = buildPulseActionButtonStyles(semantics, { web: Platform.OS === 'web' });

  if (!isLoading && !solarOutlook && !errorText) {
    return null;
  }

  const todaySummary = formatSolarOutlookSummary(solarOutlook?.today);
  const capacitySummary = formatSolarCapacitySummary(solarOutlook?.capacity);
  const modelSummary = formatSolarModelSummary(solarOutlook);
  const provenanceSummary = formatSolarProvenanceSummary(solarOutlook);

  return (
    <Card flex={fill ? 1 : undefined} height={fill ? '100%' : undefined} justifyContent={fill ? 'space-between' : undefined} minHeight={280} gap="$3">
      <YStack gap="$1">
        <XStack alignItems="center" justifyContent="space-between" gap="$3" flexWrap="wrap">
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons name="weather-sunny-alert" size={18} color="rgba(13, 148, 136, 0.92)" />
            <Text fontSize="$6" fontWeight="800">Device Solar Forecast</Text>
          </XStack>
          {deviceId ? (
            <Button
              size="$3"
              borderRadius="$5"
              borderWidth={1}
              style={actionButtonStyles.style}
              hoverStyle={actionButtonStyles.hoverStyle}
              pressStyle={actionButtonStyles.pressStyle}
              onPress={() =>
                router.push({
                  pathname: '/(tabs)/energy',
                  params: buildEnergyRouteParams({
                    scope: 'device',
                    deviceId,
                    preset: 'today',
                    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
                    includeComparison: true,
                    panel: 'solar'
                  })
                })
              }
            >
              <XStack alignItems="center" gap="$2">
                <MaterialCommunityIcons name="weather-sunny" size={16} color={semantics.actionText} />
                <Text fontWeight="700" style={{ color: semantics.actionText }}>Open Solar</Text>
              </XStack>
            </Button>
          ) : null}
        </XStack>
        <Text color="$colorMuted">
          {deviceName ? `${deviceName} forecast only.` : 'Current device forecast only.'}
        </Text>
      </YStack>

      {errorText ? (
        <Text color="$colorMuted">{errorText}</Text>
      ) : isLoading && !solarOutlook ? (
        <Text color="$colorMuted">Loading device solar forecast…</Text>
      ) : (
        <YStack gap="$2">
          {todaySummary ? <Text fontWeight="700">{todaySummary}</Text> : null}
          <Text color="$colorMuted">{capacitySummary}</Text>
          <Text color="$colorMuted">{modelSummary}</Text>
          {provenanceSummary ? <Text color="$colorMuted">{provenanceSummary}</Text> : null}
        </YStack>
      )}
    </Card>
  );
}

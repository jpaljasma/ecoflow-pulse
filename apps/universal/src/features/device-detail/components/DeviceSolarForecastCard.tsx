import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack, YStack } from 'tamagui';
import type { SolarOutlook } from '@/features/weather/model';
import {
  formatSolarCapacitySummary,
  formatSolarModelSummary,
  formatSolarOutlookSummary,
  formatSolarProvenanceSummary
} from '@/features/weather/model';
import { Card } from '@/shared/ui/Card';

export function DeviceSolarForecastCard({
  deviceName,
  solarOutlook,
  isLoading,
  errorText
}: {
  deviceName?: string;
  solarOutlook?: SolarOutlook;
  isLoading?: boolean;
  errorText?: string;
}) {
  if (!isLoading && !solarOutlook && !errorText) {
    return null;
  }

  const todaySummary = formatSolarOutlookSummary(solarOutlook?.today);
  const capacitySummary = formatSolarCapacitySummary(solarOutlook?.capacity);
  const modelSummary = formatSolarModelSummary(solarOutlook);
  const provenanceSummary = formatSolarProvenanceSummary(solarOutlook);

  return (
    <Card gap="$3">
      <YStack gap="$1">
        <XStack alignItems="center" gap="$2">
          <MaterialCommunityIcons name="weather-sunny-alert" size={18} color="rgba(13, 148, 136, 0.92)" />
          <Text fontSize="$6" fontWeight="800">Device Solar Forecast</Text>
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

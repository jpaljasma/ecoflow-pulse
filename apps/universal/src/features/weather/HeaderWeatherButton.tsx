import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import {
  formatMiniSolarOutlookSummary,
  formatWindSummary,
  formatWeatherValue,
  getWeatherCodeLabel
} from '@/features/weather/model';
import { WindDirectionIcon } from '@/features/weather/WindDirectionIcon';
import type { SolarOutlook, WeatherForecast } from '@/features/weather/model';
import { PulseStatusDot } from '@/shared/ui/PulseStatusDot';

type Props = {
  forecast?: WeatherForecast;
  solarOutlook?: SolarOutlook;
  showConfigure?: boolean;
  isLoading?: boolean;
  errorText?: string;
  onPress: () => void;
};

export function HeaderWeatherButton({
  forecast,
  solarOutlook,
  showConfigure = false,
  isLoading = false,
  errorText,
  onPress
}: Props) {
  const current = forecast?.current;
  const weatherLabel = current?.weatherLabel ?? getWeatherCodeLabel(current?.weatherCode);
  const weatherIcon = current?.weatherIcon ?? 'weather-partly-cloudy';
  const temperature = current
    ? formatWeatherValue(current.temperature2m, forecast?.unitSystem ?? 'metric', 'temperature2m')
    : '';
  const windSummary = current
    ? formatWindSummary(current.windSpeed10m, forecast?.unitSystem ?? 'metric')
    : '';
  const solarSummary = formatMiniSolarOutlookSummary(solarOutlook?.today);

  return (
    <XStack alignItems="center" gap="$2">
      <PulseStatusDot />
      <Button
        size="$3"
        onPress={onPress}
        paddingHorizontal="$3"
        paddingVertical="$2"
        minHeight={58}
        maxWidth={230}
        alignItems="flex-start"
        justifyContent="center"
        borderWidth={1}
        borderRadius={20}
        backgroundColor="rgba(14, 116, 144, 0.08)"
        borderColor="rgba(14, 116, 144, 0.18)"
        pressStyle={{ opacity: 0.85 }}
      >
        {showConfigure ? (
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons name="weather-sunny" size={18} color="rgba(14, 116, 144, 0.95)" />
            <MaterialCommunityIcons name="cog-outline" size={16} color="rgba(28, 43, 45, 0.8)" />
            <Text fontWeight="700" numberOfLines={1}>
              Configure weather
            </Text>
          </XStack>
        ) : (
          <YStack gap={2} alignItems="flex-start" width="100%">
            <XStack alignItems="center" justifyContent="space-between" gap="$2" width="100%">
              <XStack alignItems="center" gap="$2" flex={1} minWidth={0}>
                <MaterialCommunityIcons name={weatherIcon} size={18} color="rgba(14, 116, 144, 0.95)" />
                <Text flexShrink={1} fontWeight="700" numberOfLines={1}>
                  {current ? weatherLabel : 'Weather'}
                </Text>
                {temperature ? (
                  <Text fontWeight="800" numberOfLines={1}>
                    {temperature}
                  </Text>
                ) : null}
              </XStack>
              {windSummary ? (
                <XStack alignItems="center" gap={6} justifyContent="flex-end">
                  <WindDirectionIcon directionDegrees={current?.windDirection10mDegrees} size={11} />
                  <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>
                    {windSummary}
                  </Text>
                </XStack>
              ) : null}
            </XStack>
            <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>
              {solarSummary || errorText || (isLoading ? 'Loading weather…' : 'Open weather')}
            </Text>
          </YStack>
        )}
      </Button>
    </XStack>
  );
}

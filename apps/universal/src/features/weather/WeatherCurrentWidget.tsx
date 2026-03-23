import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import {
  formatVisibilityKilometers,
  formatSolarProvenanceSummary,
  formatSolarOutlookSummary,
  formatWeatherValue,
  formatWindSummary,
  getForecastDayparts,
  getWeatherCodeLabel
} from '@/features/weather/model';
import { WindDirectionIcon } from '@/features/weather/WindDirectionIcon';
import type { SolarOutlook, WeatherForecast } from '@/features/weather/model';

type Props = {
  forecast?: WeatherForecast;
  solarOutlook?: SolarOutlook;
  isLoading?: boolean;
  enabled?: boolean;
  errorText?: string;
};

export function WeatherCurrentWidget({
  forecast,
  solarOutlook,
  isLoading = false,
  enabled = true,
  errorText
}: Props) {
  const current = forecast?.current;
  const hasData = Boolean(current);
  const temperature = current ? formatWeatherValue(current.temperature2m, forecast?.unitSystem ?? 'metric', 'temperature2m') : '—';
  const label = current?.weatherLabel ?? getWeatherCodeLabel(current?.weatherCode);
  const icon = current?.weatherIcon ?? 'weather-cloudy';
  const windSummary = current
    ? formatWindSummary(current.windSpeed10m, forecast?.unitSystem ?? 'metric')
    : 'Wind unavailable';
  const visibilityLabel = current ? formatVisibilityKilometers(current.visibility) : '—';
  const dayparts = getForecastDayparts(forecast?.hourly ?? [], forecast?.timezone);
  const solarTodaySummary = formatSolarOutlookSummary(solarOutlook?.today);
  const solarProvenanceSummary = formatSolarProvenanceSummary(solarOutlook);
  const statusMessage = errorText ?? (isLoading ? 'Loading weather…' : enabled ? ' ' : 'Enable weather location consent to load forecasts.');

  return (
    <Card gap="$3" minHeight={220} opacity={isLoading && hasData ? 0.88 : 1}>
      <XStack alignItems="center" gap="$3">
        <YStack
          width={52}
          height={52}
          borderRadius={26}
          alignItems="center"
          justifyContent="center"
          style={{ backgroundColor: 'rgba(14, 116, 144, 0.12)' }}
        >
          <MaterialCommunityIcons name={icon} size={30} color="rgba(14, 116, 144, 0.95)" />
        </YStack>
        <YStack flex={1} minWidth={0} gap="$1">
          <Text fontSize="$5" fontWeight="800">
            Current weather
          </Text>
          <Text color="$colorMuted" numberOfLines={1}>
            {label}
          </Text>
          <XStack alignItems="center" gap={6}>
            <WindDirectionIcon directionDegrees={current?.windDirection10mDegrees} size={14} />
            <Text color="$colorMuted" numberOfLines={1}>
              {visibilityLabel !== '—' ? `${windSummary} · Vis ${visibilityLabel}` : windSummary}
            </Text>
          </XStack>
        </YStack>
        <YStack alignItems="flex-end" minWidth={100}>
          <Text fontSize="$8" fontWeight="900">
            {temperature}
          </Text>
          <Text color="$colorMuted">Now</Text>
        </YStack>
      </XStack>

      <YStack gap="$1">
        {solarTodaySummary ? (
          <Text fontSize="$2" color="$colorMuted">
            {solarTodaySummary}
          </Text>
        ) : null}
        <XStack gap="$2" flexWrap="wrap">
          {dayparts.map((daypart) => (
            <YStack
              key={daypart.key}
              flex={1}
              minWidth={68}
              padding="$2"
              borderRadius="$3"
              borderWidth={1}
              style={{ borderColor: 'rgba(28, 43, 45, 0.12)', backgroundColor: 'rgba(14, 116, 144, 0.04)' }}
            >
              <XStack alignItems="center" gap="$2">
                <MaterialCommunityIcons
                  name={daypart.point?.weatherIcon ?? 'weather-cloudy'}
                  size={16}
                  color="rgba(14, 116, 144, 0.95)"
                />
                <Text fontSize="$1" color="$colorMuted">
                  {daypart.label}
                </Text>
              </XStack>
              <Text fontWeight="700">
                {daypart.point
                  ? formatWeatherValue(daypart.point.temperature2m, forecast?.unitSystem ?? 'metric', 'temperature2m')
                  : '—'}
              </Text>
              <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>
                {daypart.point?.weatherLabel ?? getWeatherCodeLabel(daypart.point?.weatherCode)}
              </Text>
            </YStack>
          ))}
        </XStack>
        {solarProvenanceSummary ? (
          <Text fontSize="$2" color="$colorMuted">
            {solarProvenanceSummary}
          </Text>
        ) : null}
        <Text
          fontSize="$1"
          style={{ color: 'rgba(28, 43, 45, 0.72)', opacity: statusMessage.trim() ? 1 : 0 }}
          minHeight={16}
        >
          {statusMessage}
        </Text>
      </YStack>
    </Card>
  );
}

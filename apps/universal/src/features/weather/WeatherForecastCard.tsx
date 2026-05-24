import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import {
  formatDailyDuration,
  formatRelativeWeatherDayLabel,
  formatSolarOutlookSummary,
  formatTemperatureRange,
  formatVisibilityKilometers,
  formatWeatherValue,
  formatWindRange,
  getTodayIsoInTimezone,
  getWeatherCodeLabel
} from '@/features/weather/model';
import { summarizeDayFromHourly } from '@/features/weather/model';
import { WindDirectionIcon } from '@/features/weather/WindDirectionIcon';
import type { SolarOutlook, WeatherForecast } from '@/features/weather/model';

type Props = {
  forecast?: WeatherForecast;
  solarOutlook?: SolarOutlook;
  isLoading?: boolean;
  enabled?: boolean;
  errorText?: string;
};

export function WeatherForecastCard({
  forecast,
  solarOutlook,
  isLoading = false,
  enabled = true,
  errorText
}: Props) {
  const unitSystem = forecast?.unitSystem ?? 'metric';
  const todayIso = getTodayIsoInTimezone(forecast?.timezone);
  const visibleDays = (forecast?.daily ?? [])
    .filter((day) => day.dateIso >= todayIso)
    .slice(0, 7);
  const statusMessage = errorText ?? (isLoading ? 'Loading 7-day forecast…' : enabled ? ' ' : 'Enable weather location consent to load forecasts.');

  return (
    <Card gap="$3" padding="$3" minHeight={320} opacity={isLoading && forecast ? 0.9 : 1}>
      <YStack gap="$1">
        <Text fontSize="$7" fontWeight="800">
          7-day forecast
        </Text>
      </YStack>

      <Text
        fontSize="$1"
        style={{ color: 'rgba(28, 43, 45, 0.72)', opacity: statusMessage.trim() ? 1 : 0 }}
        minHeight={16}
      >
        {statusMessage}
      </Text>

      <YStack gap="$2">
        {visibleDays.map((day) => {
          const summary = summarizeDayFromHourly(day.dateIso, forecast?.hourly ?? [], forecast?.timezone);
          const solarDay =
            solarOutlook?.daily.find((item) => item.dateIso === day.dateIso) ??
            (day.dateIso === todayIso ? solarOutlook?.today : undefined);
          const solarSummary = formatSolarOutlookSummary(solarDay);
          const visibilityLabel = formatVisibilityKilometers(summary.representativeVisibility);
          return (
            <XStack
              key={day.dateIso}
              alignItems="center"
              gap="$2"
              padding="$2"
              borderRadius="$3"
              borderWidth={1}
              style={{ borderColor: 'rgba(28, 43, 45, 0.12)' }}
            >
              <YStack
                width={42}
                height={42}
                borderRadius={21}
                alignItems="center"
                justifyContent="center"
                style={{ backgroundColor: 'rgba(14, 116, 144, 0.1)' }}
              >
                <MaterialCommunityIcons
                  name={day.weatherIcon ?? 'weather-cloudy'}
                  size={22}
                  color="rgba(14, 116, 144, 0.95)"
                />
              </YStack>

              <YStack flex={1} minWidth={0}>
                <Text fontWeight="700">{formatRelativeWeatherDayLabel(day.dateIso, forecast?.timezone)}</Text>
                <Text color="$colorMuted" numberOfLines={1}>
                  {day.weatherLabel ?? getWeatherCodeLabel(day.weatherCode)}
                </Text>
                <XStack alignItems="center" gap={6}>
                  <Text color="$colorMuted">Wind</Text>
                  <WindDirectionIcon
                    directionDegrees={summary.representativeWindDirectionDegrees}
                    size={14}
                  />
                  <Text color="$colorMuted" numberOfLines={1}>
                    {formatWindRange(summary.lowWindSpeed, summary.highWindSpeed, unitSystem)}
                    {visibilityLabel !== '—' ? ` · Vis ${visibilityLabel}` : ''}
                  </Text>
                </XStack>
              </YStack>

              <YStack alignItems="flex-end" minWidth={132}>
                <Text fontWeight="700">
                  {formatTemperatureRange(summary.lowTemperature, summary.highTemperature, unitSystem)}
                </Text>
                {solarSummary ? (
                  <Text color="$colorMuted">
                    {solarSummary}
                  </Text>
                ) : null}
                <Text color="$colorMuted">
                  Sun {formatDailyDuration(day.sunshineDurationSeconds)} · UV {formatWeatherValue(day.uvIndexMax, unitSystem, 'uvIndex')} · daylight {formatDailyDuration(day.daylightDurationSeconds)}
                </Text>
              </YStack>
            </XStack>
          );
        })}
      </YStack>

      <Text fontSize="$1" color="$colorMuted">
        Open-Meteo data, CC BY 4.0. The profile widget uses the saved weather location and timezone from your account.
      </Text>
    </Card>
  );
}

import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import {
  formatDailyDuration,
  formatRelativeWeatherDayLabel,
  formatSolarOutlookSummary,
  formatTemperatureRange,
  formatWeatherValue,
  formatWindRange,
  getTodayIsoInTimezone,
  getWeatherCodeLabel
} from '@/features/weather/model';
import { summarizeDayFromHourly } from '@/features/weather/model';
import { WindDirectionIcon } from '@/features/weather/WindDirectionIcon';
import type { SolarOutlook, WeatherForecast, WeatherYesterdayVerification } from '@/features/weather/model';

type Props = {
  forecast?: WeatherForecast;
  solarOutlook?: SolarOutlook;
  verification?: WeatherYesterdayVerification;
  isLoading?: boolean;
  enabled?: boolean;
  errorText?: string;
  verificationErrorText?: string;
};

export function WeatherForecastCard({
  forecast,
  solarOutlook,
  verification,
  isLoading = false,
  enabled = true,
  errorText,
  verificationErrorText
}: Props) {
  const unitSystem = forecast?.unitSystem ?? 'metric';
  const visibleDays = (forecast?.daily ?? [])
    .filter((day) => day.dateIso >= getTodayIsoInTimezone(forecast?.timezone))
    .slice(0, 7);
  const statusMessage = errorText ?? (isLoading ? 'Loading 7-day forecast…' : enabled ? ' ' : 'Enable weather location consent to load forecasts.');
  const verificationStatus =
    verificationErrorText ?? (verification ? ' ' : isLoading ? 'Loading yesterday verification…' : enabled ? ' ' : 'Verification appears after a saved location is available.');

  return (
    <Card gap="$4" minHeight={320} opacity={isLoading && forecast ? 0.9 : 1}>
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
        <YStack gap="$1" flex={1} minWidth={220}>
          <Text fontSize="$7" fontWeight="800">
            7-day forecast
          </Text>
        </YStack>
        <YStack gap="$1" alignItems="flex-end" minWidth={180}>
          <Text fontWeight="700">
            {forecast?.timezone || 'Timezone auto'}
          </Text>
        </YStack>
      </XStack>

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
          const solarDay = solarOutlook?.daily.find((item) => item.dateIso === day.dateIso);
          return (
            <XStack
              key={day.dateIso}
              alignItems="center"
              gap="$3"
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
                  </Text>
                </XStack>
              </YStack>

              <YStack alignItems="flex-end" minWidth={132}>
                <Text fontWeight="700">
                  {formatTemperatureRange(summary.lowTemperature, summary.highTemperature, unitSystem)}
                </Text>
                <Text color="$colorMuted">
                  {formatSolarOutlookSummary(solarDay)}
                </Text>
                <Text color="$colorMuted">
                  Sun {formatDailyDuration(day.sunshineDurationSeconds)} · UV {formatWeatherValue(day.uvIndexMax, unitSystem, 'uvIndex')} · daylight {formatDailyDuration(day.daylightDurationSeconds)}
                </Text>
              </YStack>
            </XStack>
          );
        })}
      </YStack>

      <YStack gap="$2">
        <Text fontSize="$5" fontWeight="800">
          Yesterday verification
        </Text>
        <Text
          fontSize="$1"
          style={{ color: 'rgba(28, 43, 45, 0.72)', opacity: verificationStatus.trim() ? 1 : 0 }}
          minHeight={16}
        >
          {verificationStatus}
        </Text>

        {verification ? (
          <>
            <XStack gap="$3" flexWrap="wrap">
              <InfoPill label="Matched hours" value={`${verification.summary.matchedHours}/${verification.summary.comparedHours}`} />
              <InfoPill label="Source" value={verification.verificationSource} />
              <InfoPill
                label="Temp MAE"
                value={verification.summary.meanAbsoluteTemperatureError?.toFixed(1) ?? '—'}
              />
              <InfoPill
                label="Wind MAE"
                value={verification.summary.meanAbsoluteWindSpeedError?.toFixed(1) ?? '—'}
              />
            </XStack>

            <YStack gap="$2">
              {verification.hours.slice(0, 3).map((hour) => (
                <XStack key={hour.timestampIso} alignItems="center" gap="$3" padding="$2" borderRadius="$3" borderWidth={1} style={{ borderColor: 'rgba(28, 43, 45, 0.1)' }}>
                  <Text width={84} fontWeight="700">
                    {new Date(hour.timestampIso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}
                  </Text>
                  <Text flex={1} numberOfLines={1}>
                    {hour.forecast.weatherLabel ?? getWeatherCodeLabel(hour.forecast.weatherCode)}
                  </Text>
                  <Text color="$colorMuted">
                    ΔT {hour.error.temperature2m?.toFixed(1) ?? '—'} · ΔWind {hour.error.windSpeed10m?.toFixed(1) ?? '—'} · ΔDir {hour.error.windDirection?.toFixed(0) ?? '—'}
                  </Text>
                </XStack>
              ))}
            </YStack>
          </>
        ) : null}
      </YStack>

      <Text fontSize="$1" color="$colorMuted">
        Open-Meteo data, CC BY 4.0. The profile widget uses the saved weather location and timezone from your account.
      </Text>
    </Card>
  );
}

function InfoPill({ label, value }: { label: string; value: string }) {
  return (
    <YStack paddingHorizontal="$3" paddingVertical="$2" borderRadius="$5" borderWidth={1} style={{ borderColor: 'rgba(28, 43, 45, 0.12)', backgroundColor: 'rgba(14, 116, 144, 0.06)' }}>
      <Text fontSize="$1" color="$colorMuted">
        {label}
      </Text>
      <Text fontWeight="800">{value}</Text>
    </YStack>
  );
}

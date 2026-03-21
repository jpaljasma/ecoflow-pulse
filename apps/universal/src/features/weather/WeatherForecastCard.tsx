import { useState } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import {
  formatDailyDuration,
  formatLocalTimeLabel,
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
  verificationIsLoading?: boolean;
  enabled?: boolean;
  errorText?: string;
  verificationErrorText?: string;
  onVerificationExpand?: () => void;
};

export function WeatherForecastCard({
  forecast,
  solarOutlook,
  verification,
  isLoading = false,
  verificationIsLoading = false,
  enabled = true,
  errorText,
  verificationErrorText,
  onVerificationExpand
}: Props) {
  const [verificationExpanded, setVerificationExpanded] = useState(false);
  const unitSystem = forecast?.unitSystem ?? 'metric';
  const todayIso = getTodayIsoInTimezone(forecast?.timezone);
  const visibleDays = (forecast?.daily ?? [])
    .filter((day) => day.dateIso >= todayIso)
    .slice(0, 7);
  const statusMessage = errorText ?? (isLoading ? 'Loading 7-day forecast…' : enabled ? ' ' : 'Enable weather location consent to load forecasts.');
  const verificationStatus =
    verificationErrorText ??
    (verification
      ? ' '
      : verificationIsLoading
        ? 'Loading yesterday verification…'
        : enabled
          ? ' '
          : 'Verification appears after a saved location is available.');

  function handleVerificationToggle() {
    const nextExpanded = !verificationExpanded;
    setVerificationExpanded(nextExpanded);
    if (nextExpanded) {
      onVerificationExpand?.();
    }
  }

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
        {visibleDays.map((day, index) => {
          const summary = summarizeDayFromHourly(day.dateIso, forecast?.hourly ?? [], forecast?.timezone);
          const solarDay =
            solarOutlook?.daily.find((item) => item.dateIso === day.dateIso) ??
            (day.dateIso === todayIso ? solarOutlook?.today : undefined) ??
            solarOutlook?.daily[index];
          const solarSummary = formatSolarOutlookSummary(solarDay);
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

      <YStack gap="$2">
        <Button
          size="$4"
          justifyContent="space-between"
          alignItems="center"
          paddingHorizontal="$2"
          paddingVertical="$2"
          borderRadius="$3"
          borderWidth={1}
          onPress={handleVerificationToggle}
          style={{ borderColor: 'rgba(28, 43, 45, 0.12)', backgroundColor: 'rgba(14, 116, 144, 0.04)' }}
        >
          <XStack alignItems="center" justifyContent="space-between" width="100%">
            <YStack gap={2} alignItems="flex-start">
              <Text fontSize="$5" fontWeight="800">
                Yesterday verification
              </Text>
              <Text fontSize="$1" color="$colorMuted">
                {verificationExpanded ? 'Hide 24-hour verification details' : 'Show 24-hour verification details'}
              </Text>
            </YStack>
            <MaterialCommunityIcons
              name={verificationExpanded ? 'chevron-up' : 'chevron-down'}
              size={22}
              color="rgba(28, 43, 45, 0.92)"
            />
          </XStack>
        </Button>

        {verificationExpanded ? (
          <>
            <Text
              fontSize="$1"
              style={{ color: 'rgba(28, 43, 45, 0.72)', opacity: verificationStatus.trim() ? 1 : 0 }}
              minHeight={16}
            >
              {verificationStatus}
            </Text>

            {verification ? (
              <YStack
                gap="$2"
                padding="$2"
                borderRadius="$3"
                borderWidth={1}
                style={{ borderColor: 'rgba(28, 43, 45, 0.12)', backgroundColor: 'rgba(14, 116, 144, 0.03)' }}
              >
                <XStack gap="$2" flexWrap="wrap">
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
                  {verification.hours.slice(0, 24).map((hour) => (
                    <XStack key={hour.timestampIso} alignItems="center" gap="$3" padding="$2" borderRadius="$3" borderWidth={1} style={{ borderColor: 'rgba(28, 43, 45, 0.1)' }}>
                      <Text width={84} fontWeight="700">
                        {formatLocalTimeLabel(hour.timestampIso, verification.timezone)}
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
              </YStack>
            ) : null}
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
    <YStack paddingHorizontal="$2" paddingVertical="$2" borderRadius="$5" borderWidth={1} style={{ borderColor: 'rgba(28, 43, 45, 0.12)', backgroundColor: 'rgba(14, 116, 144, 0.06)' }}>
      <Text fontSize="$1" color="$colorMuted">
        {label}
      </Text>
      <Text fontWeight="800">{value}</Text>
    </YStack>
  );
}

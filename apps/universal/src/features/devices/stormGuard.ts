import type { DeviceSummary } from '@/features/devices/api';
import type { StormGuardBannerProps } from '@/shared/ui/StormGuardBanner';

function formatRemaining(ms: number): string {
  const totalMinutes = Math.max(1, Math.round(ms / 60_000));
  if (totalMinutes < 60) {
    return `~${totalMinutes}m more`;
  }
  const hours = totalMinutes / 60;
  if (hours < 10) {
    return `~${hours.toFixed(hours < 2 ? 1 : 0)}h more`;
  }
  return `~${Math.round(hours)}h more`;
}

export function buildStormGuardLabel(
  details: DeviceSummary['details'] | undefined,
  now = Date.now()
): string | null {
  if (details?.stormGuardActive !== true) {
    return null;
  }

  const endsAt = details.stormGuardEndsAtUnixMs;
  if (typeof endsAt === 'number' && Number.isFinite(endsAt) && endsAt > now) {
    return `Storm Guard ${formatRemaining(endsAt - now)}`;
  }

  return 'Storm Guard active';
}

export function buildStormGuardBanner(
  devices: DeviceSummary[] | undefined,
  now = new Date()
): StormGuardBannerProps | null {
  const activeDevices = (devices ?? []).filter(
    (device) => device.details?.stormGuardActive === true
  );
  if (activeDevices.length === 0) {
    return null;
  }

  const soonestEnd = activeDevices
    .map((device) => device.details?.stormGuardEndsAtUnixMs)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0)
    .sort((left, right) => left - right)[0];

  const headline =
    soonestEnd && soonestEnd > now.getTime()
      ? `Storm Guard active for ${formatRemaining(soonestEnd - now.getTime())}`
      : 'Storm Guard is active';

  const detail =
    activeDevices.length === 1
      ? 'Device-reported protection mode is active and solar intake may stay limited until the alert window ends.'
      : 'At least one visible EcoFlow device reports protective Storm Guard mode and solar intake may stay limited until the alert window ends.';

  const affectedLabel =
    activeDevices.length === 1
      ? `Active on ${activeDevices[0]?.name ?? '1 device'}`
      : `Active on ${activeDevices.length} visible devices`;

  return {
    headline,
    detail,
    affectedLabel
  };
}

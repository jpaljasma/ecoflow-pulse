import { StatusBanner } from '@/shared/ui/StatusBanner';

export type StormGuardBannerProps = {
  headline: string;
  detail?: string;
  affectedLabel?: string;
  compact?: boolean;
};

export function StormGuardBanner({
  headline,
  detail,
  affectedLabel,
  compact = false
}: StormGuardBannerProps) {
  return (
    <StatusBanner
      iconText="!"
      headline={headline}
      detail={detail}
      footnote={affectedLabel}
      statusLabel="Active"
      compact={compact}
    />
  );
}

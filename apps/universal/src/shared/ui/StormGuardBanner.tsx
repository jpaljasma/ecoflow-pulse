import { StatusBanner } from '@/shared/ui/StatusBanner';

export type StormGuardBannerProps = {
  headline: string;
  detail?: string;
  affectedLabel?: string;
};

export function StormGuardBanner({
  headline,
  detail,
  affectedLabel
}: StormGuardBannerProps) {
  return (
    <StatusBanner
      iconText="!"
      headline={headline}
      detail={detail}
      footnote={affectedLabel}
      statusLabel="Active"
    />
  );
}

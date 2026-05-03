import { XStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailSignalPillVM } from '@/features/device-detail/view-model';
import { IconLabel } from '@/shared/ui/IconLabel';

function signalPillLabel(pill: DetailSignalPillVM): string {
  if (pill.standalone === true) {
    return pill.label;
  }
  if (pill.value !== undefined) {
    return `${pill.label} · ${pill.value}`;
  }
  return `${pill.on ? 'On' : 'Off'} · ${pill.label}`;
}

export function SystemSignalsSection({
  pills,
  minWidth
}: {
  pills: DetailSignalPillVM[];
  minWidth?: number;
}) {
  return (
    <SectionCard title={<IconLabel icon="check-decagram-outline" label="System Signals" />} minWidth={minWidth}>
      <XStack gap="$2" flexWrap="wrap">
        {pills.map((pill) => (
          <Pill
            key={pill.key}
            label={signalPillLabel(pill)}
            tone={pill.tone}
          />
        ))}
      </XStack>
    </SectionCard>
  );
}

import { XStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailSignalPillVM } from '@/features/device-detail/view-model';
import { IconLabel } from '@/shared/ui/IconLabel';

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
            label={
              pill.value !== undefined
                ? `${pill.label} · ${pill.value}`
                : `${pill.on ? 'On' : 'Off'} · ${pill.label}`
            }
            tone={pill.tone}
          />
        ))}
      </XStack>
    </SectionCard>
  );
}

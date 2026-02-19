import { XStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailEstimatePillVM } from '@/features/device-detail/view-model';

export function EstimateQueueSection({
  pills,
  minWidth
}: {
  pills: DetailEstimatePillVM[];
  minWidth?: number;
}) {
  return (
    <SectionCard title="🧭 Estimate & Queue" minWidth={minWidth}>
      <XStack gap="$2" flexWrap="wrap">
        {pills.map((pill) => (
          <Pill key={pill.key} label={pill.label} tone={pill.tone} />
        ))}
      </XStack>
    </SectionCard>
  );
}


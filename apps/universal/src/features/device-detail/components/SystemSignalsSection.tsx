import { XStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailSignalPillVM } from '@/features/device-detail/view-model';

export function SystemSignalsSection({
  pills,
  minWidth
}: {
  pills: DetailSignalPillVM[];
  minWidth?: number;
}) {
  return (
    <SectionCard title="✅ System Signals" minWidth={minWidth}>
      <XStack gap="$2" flexWrap="wrap">
        {pills.map((pill) => (
          <Pill key={pill.key} label={`${pill.on ? '●' : '○'} ${pill.label}`} tone={pill.tone} />
        ))}
      </XStack>
    </SectionCard>
  );
}


import { YStack } from 'tamagui';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailSolarPortVM } from '@/features/device-detail/view-model';
import { SolarPortCard } from '@/features/device-detail/components/SolarPortCard';

export function SolarInputsSection({
  ports,
  minWidth
}: {
  ports: DetailSolarPortVM[];
  minWidth?: number;
}) {
  return (
    <SectionCard title="☀ Solar Inputs" minWidth={minWidth}>
      <YStack gap="$2">
        {ports.map((port) => (
          <SolarPortCard key={port.id} port={port} />
        ))}
      </YStack>
    </SectionCard>
  );
}


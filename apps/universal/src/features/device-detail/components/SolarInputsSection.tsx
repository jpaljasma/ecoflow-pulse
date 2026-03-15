import { Text, YStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailSolarPortVM } from '@/features/device-detail/view-model';
import { SolarPortCard } from '@/features/device-detail/components/SolarPortCard';
import { formatW } from '@/features/telemetry/format';
import { IconLabel } from '@/shared/ui/IconLabel';

export function SolarInputsSection({
  ports,
  minWidth
}: {
  ports: DetailSolarPortVM[];
  minWidth?: number;
}) {
  const totalCapacityW = ports.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0);
  const activeCount = ports.filter((port) => !port.inactive).length;
  const chargingCount = ports.filter((port) => port.stateLabel === 'charging').length;

  return (
    <SectionCard
      title={<IconLabel icon="white-balance-sunny" label="Solar Inputs" />}
      right={<Pill label={`${ports.length} ports · ${formatW(totalCapacityW)}`} tone="warning" />}
      minWidth={minWidth}
    >
      <Text opacity={0.7}>
        {chargingCount > 0
          ? `${chargingCount} port${chargingCount === 1 ? '' : 's'} charging · ${activeCount} active`
          : activeCount > 0
            ? `${activeCount} port${activeCount === 1 ? '' : 's'} currently reporting input`
            : 'No PV input currently flowing'}
      </Text>
      <YStack gap="$2">
        {ports.map((port) => (
          <SolarPortCard key={port.id} port={port} />
        ))}
      </YStack>
    </SectionCard>
  );
}

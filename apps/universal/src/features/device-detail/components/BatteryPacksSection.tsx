import { Text, XStack, YStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SocBar } from '@/shared/ui/SocBar';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailBatteryPackVM } from '@/features/device-detail/view-model';

export function BatteryPacksSection({
  packs,
  bpCount,
  minWidth
}: {
  packs: DetailBatteryPackVM[];
  bpCount?: number;
  minWidth?: number;
}) {
  return (
    <SectionCard
      title="🔋 Battery Packs"
      right={<Pill label={`${bpCount ?? packs.length ?? 0} packs`} tone="info" />}
      minWidth={minWidth}
    >
      {packs.length ? (
        <YStack gap="$2">
          {packs.map((pack) => (
            <YStack
              key={pack.id}
              gap="$1"
              padding="$2"
              borderRadius="$3"
              borderWidth={1}
              borderColor={pack.heatingOn ? 'rgba(255,159,10,0.55)' : 'rgba(120,120,128,0.24)'}
              backgroundColor={pack.heatingOn ? 'rgba(255,159,10,0.10)' : undefined}
            >
              <XStack alignItems="center" justifyContent="space-between">
                <Text fontWeight="700">{pack.id.toUpperCase()}</Text>
                <XStack gap="$2" alignItems="center">
                  {pack.heatingOn ? (
                    <Text color="rgba(255,159,10,0.95)" fontWeight="700">
                      ♨ Preconditioning
                    </Text>
                  ) : null}
                  <Text opacity={0.8}>
                    {pack.powerText} {' · '} {pack.tempText}
                  </Text>
                </XStack>
              </XStack>
              <SocBar value={pack.socPct} fullWidth />
            </YStack>
          ))}
        </YStack>
      ) : (
        <Text opacity={0.7}>No per-pack telemetry yet.</Text>
      )}
    </SectionCard>
  );
}


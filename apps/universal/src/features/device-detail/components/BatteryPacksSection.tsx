import { useMemo, useRef } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { SocBar } from '@/shared/ui/SocBar';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { DetailBatteryPackVM } from '@/features/device-detail/view-model';
import { BatteryUpsellComponent } from '@/features/device-detail/components/BatteryUpsellComponent';
import { getBatteryUpsellUrl, getMaxBatteryCount } from '@/shared/config/merchandising';

export function BatteryPacksSection({
  packs,
  bpCount,
  summaryText,
  model,
  serialNumber,
  minWidth
}: {
  packs: DetailBatteryPackVM[];
  bpCount?: number;
  summaryText?: string;
  model?: string;
  serialNumber?: string;
  minWidth?: number;
}) {
  const stableBatteryCountRef = useRef<number>(0);
  const observedCount = Math.max(bpCount ?? 0, packs.length ?? 0);
  if (observedCount > stableBatteryCountRef.current) {
    stableBatteryCountRef.current = observedCount;
  }
  const batteryCount = stableBatteryCountRef.current;
  const inferredOnlyCount = Math.max(0, batteryCount - packs.length);
  const upsellHref = useMemo(
    () =>
      getBatteryUpsellUrl({
        model,
        serialNumber,
        batteryCount
      }),
    [model, serialNumber, batteryCount]
  );
  const maxBatteries = useMemo(() => getMaxBatteryCount(model), [model]);
  return (
    <SectionCard
      title="🔋 Battery Packs"
      right={<Pill label={`${batteryCount} packs`} tone="info" />}
      minWidth={minWidth}
    >
      {summaryText ? <Text opacity={0.7}>{summaryText}</Text> : null}
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
              {pack.summaryText || pack.reserveText ? (
                <Text opacity={0.65}>
                  {[pack.summaryText, pack.reserveText ? `limits ${pack.reserveText}` : undefined]
                    .filter(Boolean)
                    .join(' · ')}
                </Text>
              ) : null}
            </YStack>
          ))}
        </YStack>
      ) : batteryCount > 0 ? (
        <YStack gap="$2">
          <Text opacity={0.85}>{batteryCount} pack{batteryCount === 1 ? '' : 's'} detected.</Text>
          <Text opacity={0.65}>Per-pack telemetry has not been published yet.</Text>
        </YStack>
      ) : (
        <Text opacity={0.7}>No per-pack telemetry yet.</Text>
      )}
      {packs.length && inferredOnlyCount > 0 ? (
        <Text opacity={0.65}>
          {inferredOnlyCount} additional pack{inferredOnlyCount === 1 ? '' : 's'} detected from device capabilities.
        </Text>
      ) : null}
      <BatteryUpsellComponent
        href={upsellHref}
        modelName={model}
        batteryCount={batteryCount}
        maxBatteries={maxBatteries}
      />
    </SectionCard>
  );
}

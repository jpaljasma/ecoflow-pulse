import { Text, XStack, YStack } from 'tamagui';
import { formatKWh, formatSoc } from '@/features/telemetry/format';
import { useThemeSemantics } from '@/shared/theme/semantic';

function clampPct(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, value));
}

export function BatteryWindowSummary({
  chargeKwh,
  dischargeKwh,
  netKwh,
  socStartPct,
  socEndPct,
  socMinPct,
  socMaxPct
}: {
  chargeKwh: number;
  dischargeKwh: number;
  netKwh: number;
  socStartPct: number;
  socEndPct: number;
  socMinPct: number;
  socMaxPct: number;
}) {
  const semantics = useThemeSemantics();
  const totalFlow = Math.max(chargeKwh + dischargeKwh, 0.001);
  const chargePct = Math.max(8, Math.round((chargeKwh / totalFlow) * 100));
  const dischargePct = Math.max(8, Math.round((dischargeKwh / totalFlow) * 100));
  const bandStart = clampPct(socMinPct);
  const bandWidth = Math.max(4, clampPct(socMaxPct) - bandStart);
  const startMarker = clampPct(socStartPct);
  const endMarker = clampPct(socEndPct);

  return (
    <YStack gap="$3">
      <YStack gap="$2">
        <XStack justifyContent="space-between" alignItems="center">
          <Text fontSize="$2" color="$colorMuted">
            Flow strip
          </Text>
          <Text fontSize="$2" color="$colorMuted">
            Net {formatKWh(netKwh)}
          </Text>
        </XStack>
        <XStack
          height={14}
          borderRadius="$5"
          overflow="hidden"
          style={{
            backgroundColor: semantics.mutedPanelBackground,
            borderColor: semantics.mutedPanelBorder,
            borderWidth: 1
          }}
        >
          <XStack
            height="100%"
            width={`${chargePct}%` as `${number}%`}
            style={{ backgroundColor: semantics.batteryFlowCharge }}
          />
          <XStack
            height="100%"
            width={`${dischargePct}%` as `${number}%`}
            style={{ backgroundColor: semantics.batteryFlowDischarge }}
          />
        </XStack>
        <XStack justifyContent="space-between" gap="$3" flexWrap="wrap">
          <Text fontSize="$2" style={{ color: semantics.batteryFlowCharge }}>
            Charge {formatKWh(chargeKwh)}
          </Text>
          <Text fontSize="$2" style={{ color: semantics.batteryFlowDischarge }}>
            Discharge {formatKWh(dischargeKwh)}
          </Text>
        </XStack>
      </YStack>

      <YStack gap="$2">
        <XStack justifyContent="space-between" alignItems="center">
          <Text fontSize="$2" color="$colorMuted">
            SOC band
          </Text>
          <Text fontSize="$2" color="$colorMuted">
            {formatSoc(socMinPct)} - {formatSoc(socMaxPct)}
          </Text>
        </XStack>
        <YStack
          height={16}
          borderRadius="$5"
          justifyContent="center"
          style={{
            backgroundColor: semantics.mutedPanelBackground,
            borderColor: semantics.mutedPanelBorder,
            borderWidth: 1
          }}
        >
          <XStack
            height={8}
            borderRadius="$4"
            position="absolute"
            left={`${bandStart}%` as `${number}%`}
            width={`${bandWidth}%` as `${number}%`}
            style={{ backgroundColor: semantics.chartSolarMuted }}
          />
          <XStack
            width={2}
            height={16}
            position="absolute"
            left={`${startMarker}%` as `${number}%`}
            style={{ backgroundColor: semantics.chartAc }}
          />
          <XStack
            width={2}
            height={16}
            position="absolute"
            left={`${endMarker}%` as `${number}%`}
            style={{ backgroundColor: semantics.chartLoad }}
          />
        </YStack>
        <XStack justifyContent="space-between" gap="$3" flexWrap="wrap">
          <Text fontSize="$2">Start {formatSoc(socStartPct)}</Text>
          <Text fontSize="$2">End {formatSoc(socEndPct)}</Text>
        </XStack>
      </YStack>
    </YStack>
  );
}

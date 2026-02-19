import { Text, XStack, YStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { isMutedMetric, pvLoadColor } from '@/shared/ui/uiMappings';
import type { DetailSolarPortVM } from '@/features/device-detail/view-model';

export function SolarPortCard({ port }: { port: DetailSolarPortVM }) {
  return (
    <YStack
      gap="$2"
      padding="$2"
      borderRadius="$3"
      borderWidth={1}
      borderColor="rgba(120,120,128,0.24)"
      opacity={port.inactive ? 0.72 : 1}
    >
      <XStack justifyContent="space-between" alignItems="center">
        <Text fontWeight="700">{port.name}</Text>
        <Pill label={port.stateLabel} tone={port.stateTone} />
      </XStack>
      <XStack gap="$3" flexWrap="wrap">
        <Stat
          label="⚡ W"
          value={port.wattsText}
          tone={port.inactive || isMutedMetric(port.watts) ? 'muted' : 'default'}
        />
        <Stat label="V" value={port.voltsText} tone={port.inactive ? 'muted' : 'default'} />
        <Stat label="A" value={port.ampsText} tone={port.inactive ? 'muted' : 'default'} />
        <Stat label="Cap" value={port.capText} tone={port.inactive ? 'muted' : 'default'} />
      </XStack>
      <YStack gap="$1">
        <XStack alignItems="center" justifyContent="space-between">
          <Text opacity={port.inactive ? 0.6 : 0.85} fontWeight="600">
            PV Load
          </Text>
          <Text opacity={port.inactive ? 0.6 : 0.9} fontWeight="700">
            {port.pvLoadPct === null ? '—' : `${port.pvLoadClamped.toFixed(1)}%`}
          </Text>
        </XStack>
        <XStack
          height={10}
          borderRadius="$5"
          overflow="hidden"
          backgroundColor="rgba(255,159,10,0.14)"
        >
          <XStack
            height="100%"
            width={`${port.pvLoadClamped}%` as `${number}%`}
            opacity={port.inactive ? 0.5 : 1}
            style={{ backgroundColor: pvLoadColor(port.pvLoadClamped) }}
          />
        </XStack>
      </YStack>
    </YStack>
  );
}


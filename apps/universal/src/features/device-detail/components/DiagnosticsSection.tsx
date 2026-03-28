import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useState } from 'react';
import { Button, Text, XStack, YStack } from 'tamagui';
import { Pill } from '@/shared/ui/Pill';
import { Card } from '@/shared/ui/Card';
import type { DetailDiagnosticPillVM } from '@/features/device-detail/view-model';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { IconLabel } from '@/shared/ui/IconLabel';

export function DiagnosticsSection({
  pills,
  minWidth
}: {
  pills: DetailDiagnosticPillVM[];
  minWidth?: number;
}) {
  const [expanded, setExpanded] = useState(false);
  const semantics = useThemeSemantics();

  return (
    <Card gap="$3" flex={1} minWidth={minWidth}>
      <Button
        unstyled
        width="100%"
        onPress={() => setExpanded((value) => !value)}
        accessibilityRole="button"
        accessibilityState={{ expanded }}
      >
        <XStack alignItems="center" justifyContent="space-between" gap="$3">
          <YStack gap="$1">
            <IconLabel icon="bug-outline" label="Diagnostics" />
            <Text fontSize="$2" opacity={0.7}>
              {expanded ? 'Hide diagnostics' : 'Show diagnostics'}
            </Text>
          </YStack>
          <MaterialCommunityIcons
            name={expanded ? 'chevron-up' : 'chevron-down'}
            size={22}
            color={semantics.subtleStrongText}
          />
        </XStack>
      </Button>

      {expanded ? (
        <XStack gap="$2" flexWrap="wrap">
          {pills.map((pill) => (
            <Pill key={pill.key} label={`${pill.label} · ${pill.value}`} tone={pill.tone} />
          ))}
        </XStack>
      ) : (
        <Text fontSize="$2" opacity={0.65}>
          Hidden by default for a cleaner device summary.
        </Text>
      )}
    </Card>
  );
}

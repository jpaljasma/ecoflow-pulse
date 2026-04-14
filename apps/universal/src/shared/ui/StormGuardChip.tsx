import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack } from 'tamagui';

import { useThemeSemantics } from '@/shared/theme/semantic';

export function StormGuardChip({
  label
}: {
  label: string;
}) {
  const semantics = useThemeSemantics();

  return (
    <XStack
      alignItems="center"
      gap="$2"
      paddingHorizontal="$3"
      paddingVertical="$2"
      borderRadius={999}
      borderWidth={1}
      minHeight={40}
      style={{
        backgroundColor: `${semantics.statusWarning}1f`,
        borderColor: semantics.statusWarning
      }}
    >
      <MaterialCommunityIcons name="shield-alert-outline" size={16} color={semantics.statusWarning} />
      <Text fontSize="$2" fontWeight="700" style={{ color: semantics.statusWarning }}>
        {label}
      </Text>
    </XStack>
  );
}

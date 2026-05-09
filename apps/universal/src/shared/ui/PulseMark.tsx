import { MaterialCommunityIcons } from '@expo/vector-icons';
import { YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

export function PulseMark({ size = 42 }: { size?: number }) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      width={size}
      height={size}
      alignItems="center"
      justifyContent="center"
      borderRadius={Math.round(size * 0.28)}
      borderWidth={1}
      style={{
        backgroundColor: semantics.navBrandBackground,
        borderColor: semantics.navBrandBorder
      }}
    >
      <MaterialCommunityIcons name="heart-pulse" size={Math.round(size * 0.58)} color={semantics.chartSolar} />
    </YStack>
  );
}

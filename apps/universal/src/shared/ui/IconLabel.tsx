import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack } from 'tamagui';

export function IconLabel({
  icon,
  label,
  color,
  size = 14,
  gap = '$1'
}: {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
  color?: string;
  size?: number;
  gap?: ComponentProps<typeof XStack>['gap'];
}) {
  return (
    <XStack alignItems="center" gap={gap}>
      <MaterialCommunityIcons name={icon} size={size} color={color} />
      <Text style={color ? { color } : undefined}>{label}</Text>
    </XStack>
  );
}

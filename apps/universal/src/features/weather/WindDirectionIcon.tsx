import { MaterialCommunityIcons } from '@expo/vector-icons';

type Props = {
  directionDegrees?: number | null;
  size?: number;
  color?: string;
};

export function WindDirectionIcon({
  directionDegrees,
  size = 16,
  color = 'rgba(14, 116, 144, 0.95)'
}: Props) {
  if (directionDegrees === null || directionDegrees === undefined || Number.isNaN(directionDegrees)) {
    return null;
  }

  return (
    <MaterialCommunityIcons
      name="navigation"
      size={size}
      color={color}
      style={{ transform: [{ rotate: `${normalizeDegrees(directionDegrees)}deg` }] }}
    />
  );
}

function normalizeDegrees(value: number): number {
  return ((value % 360) + 360) % 360;
}

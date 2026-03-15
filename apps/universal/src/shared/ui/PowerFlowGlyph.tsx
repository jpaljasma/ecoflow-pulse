import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Platform } from 'react-native';
import { XStack } from 'tamagui';
import { getPowerFlowIconNames } from '@/shared/ui/statusGlyph';

export type PowerFlowGlyphProps = {
  stale?: boolean;
  status?: 'charging' | 'discharging' | 'idle' | 'stale';
  pvW?: number;
  loadW?: number;
  fontSize?: number | string;
  lineHeight?: number;
  opacity?: number;
};

export function PowerFlowGlyph({
  stale,
  status,
  pvW,
  loadW,
  fontSize = '$7',
  lineHeight = 30,
  opacity = 1
}: PowerFlowGlyphProps) {
  const glyphParts = getPowerFlowIconNames({ stale, status, pvW, loadW });
  const adjustedLineHeight = Math.max(lineHeight, 34);
  const iconSize =
    typeof fontSize === 'number'
      ? fontSize
      : fontSize === '$8'
        ? 28
        : fontSize === '$7'
          ? 24
          : fontSize === '$6'
            ? 20
            : 22;

  return (
    <XStack alignItems="center" gap={2} opacity={opacity}>
      {glyphParts.map((glyph, index) => (
        <MaterialCommunityIcons
          key={`${glyph}-${index}`}
          name={glyph}
          size={iconSize}
          color="rgba(28, 43, 45, 0.92)"
          style={Platform.OS === 'ios' ? ({ paddingTop: 2, lineHeight: adjustedLineHeight } as any) : undefined}
        />
      ))}
    </XStack>
  );
}

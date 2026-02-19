import type { ComponentProps } from 'react';
import { Platform } from 'react-native';
import { Text, XStack } from 'tamagui';
import { getPowerFlowGlyphParts } from '@/shared/ui/statusGlyph';

export type PowerFlowGlyphProps = {
  stale?: boolean;
  status?: 'charging' | 'discharging' | 'idle' | 'stale';
  pvW?: number;
  loadW?: number;
  fontSize?: ComponentProps<typeof Text>['fontSize'];
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
  const glyphParts = getPowerFlowGlyphParts({ stale, status, pvW, loadW });
  const adjustedLineHeight = Math.max(lineHeight, 34);

  return (
    <XStack alignItems="center" gap={2} opacity={opacity}>
      {glyphParts.map((glyph, index) => (
        <Text
          key={`${glyph}-${index}`}
          fontSize={fontSize}
          lineHeight={adjustedLineHeight}
          style={Platform.OS === 'ios' ? ({ paddingTop: 2 } as any) : undefined}
        >
          {glyph}
        </Text>
      ))}
    </XStack>
  );
}

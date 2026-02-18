import type { ComponentProps } from 'react';
import { Text } from 'tamagui';
import { getPowerFlowGlyph } from '@/shared/ui/statusGlyph';

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
  return (
    <Text fontSize={fontSize} lineHeight={lineHeight} opacity={opacity}>
      {getPowerFlowGlyph({ stale, status, pvW, loadW })}
    </Text>
  );
}

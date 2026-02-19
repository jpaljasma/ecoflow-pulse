import type { ReactNode } from 'react';
import { XStack, YStack } from 'tamagui';

export type MetricsGridItem = {
  key: string;
  content: ReactNode;
  span?: number;
};

export function MetricsGrid({
  items,
  columns = 3,
  padX = 4,
  padY = 2
}: {
  items: MetricsGridItem[];
  columns?: number;
  padX?: number;
  padY?: number;
}) {
  return (
    <XStack flexWrap="wrap" marginHorizontal={-padX}>
      {items.map((item) => {
        const span = Math.max(1, Math.min(columns, item.span ?? 1));
        const widthPct = `${(span / columns) * 100}%` as `${number}%`;
        return (
          <YStack key={item.key} width={widthPct} paddingHorizontal={padX} paddingVertical={padY}>
            {item.content}
          </YStack>
        );
      })}
    </XStack>
  );
}


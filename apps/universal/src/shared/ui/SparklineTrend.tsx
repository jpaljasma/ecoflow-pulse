import { Platform } from 'react-native';
import { Text, XStack } from 'tamagui';

export function normalizeTrend(values: number[], targetPoints: number): number[] {
  const clipped = values.slice(-targetPoints);
  if (clipped.length >= targetPoints) return clipped;
  return [...Array.from({ length: targetPoints - clipped.length }, () => 0), ...clipped];
}

export function SparklineTrend({
  values,
  points = 60
}: {
  values: number[];
  points?: number;
}) {
  const normalizedInput = normalizeTrend(values, points);
  const min = Math.min(...normalizedInput);
  const max = Math.max(...normalizedInput);
  const range = max - min || 1;
  const normalized = normalizedInput.map((v) => ((v - min) / range) * 36);
  const sparkline = normalized.map((v) => (v > 28 ? '█' : v > 18 ? '▓' : v > 10 ? '▒' : '░'));
  const intensity = normalized.map((v) => Math.max(0, Math.min(1, v / 36)));

  return (
    <XStack width="100%" justifyContent="flex-end" overflow="hidden">
      <Text
        fontSize="$3"
        opacity={0.85}
        textAlign="right"
        {...(Platform.OS === 'web' ? ({ style: { whiteSpace: 'nowrap' } } as const) : {})}
      >
        {sparkline.map((glyph, idx) => {
          const level = intensity[idx] ?? 0;
          const alpha = 0.25 + level * 0.75;
          return (
            <Text key={idx} color={`rgba(255,159,10,${alpha.toFixed(3)})`}>
              {glyph}
            </Text>
          );
        })}
      </Text>
    </XStack>
  );
}

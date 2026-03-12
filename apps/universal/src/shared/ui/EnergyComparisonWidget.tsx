import { useEffect, useMemo, useRef, useState } from 'react';
import { ScrollView, useWindowDimensions } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import type { EnergyComparisonInsightResponse } from '@/features/energy/api';
import { Card } from '@/shared/ui/Card';
import { useThemeSemantics } from '@/shared/theme/semantic';

function verdictColor(verdictClass: string, semantics: ReturnType<typeof useThemeSemantics>): string {
  switch (verdictClass) {
    case 'solar_freedom_up':
      return semantics.statusSuccess;
    case 'solar_freedom_down':
      return semantics.statusDanger;
    case 'steady_state':
      return semantics.subtleStrongText;
    default:
      return semantics.statusWarning;
  }
}

type Props = {
  data?: EnergyComparisonInsightResponse;
  loading?: boolean;
};

export function EnergyComparisonWidget({ data, loading = false }: Props) {
  const semantics = useThemeSemantics();
  const { width } = useWindowDimensions();
  const scrollRef = useRef<ScrollView>(null);
  const pauseUntilRef = useRef<number>(0);
  const [cardIndex, setCardIndex] = useState(0);
  const insight = data?.insight;
  const cards = insight?.cards ?? [];
  const cardWidth = useMemo(() => Math.max(280, Math.min(width - 72, 420)), [width]);

  useEffect(() => {
    if (cards.length < 2) {
      return undefined;
    }
    const timer = setInterval(() => {
      if (Date.now() < pauseUntilRef.current) {
        return;
      }
      setCardIndex((current) => {
        const next = (current + 1) % cards.length;
        scrollRef.current?.scrollTo({ x: next * (cardWidth + 12), animated: true });
        return next;
      });
    }, 4500);
    return () => clearInterval(timer);
  }, [cardWidth, cards.length]);

  const beginManualInteraction = () => {
    pauseUntilRef.current = Date.now() + 15000;
  };

  if (!insight && !loading) {
    return null;
  }

  return (
    <Card
      gap="$3"
      style={{
        backgroundColor: semantics.chartFrameBackground,
        borderColor: semantics.chartFrameBorder
      }}
    >
      <YStack gap="$2">
        <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
          <YStack gap="$1" flex={1}>
            <Text fontSize="$5" fontWeight="800">
              Comparison status
            </Text>
            <Text color="$colorMuted">
              Cached hourly inference over the current and previous energy windows. This view always compares, regardless of the chart toggle.
            </Text>
          </YStack>
          {insight ? (
            <YStack alignItems="flex-end" gap="$1">
              <Text
                fontSize="$2"
                fontWeight="700"
                style={{
                  color: verdictColor(insight.verdictClass, semantics)
                }}
              >
                {insight.verdictClass.replaceAll('_', ' ')}
              </Text>
              <Text color="$colorMuted">{`${Math.round(insight.confidence * 100)}% confidence`}</Text>
            </YStack>
          ) : null}
        </XStack>

        {loading && !insight ? (
          <Text color="$colorMuted">Loading comparison analysis…</Text>
        ) : insight ? (
          <>
            <YStack gap="$1">
              <Text fontSize="$7" fontWeight="800">
                {insight.headline}
              </Text>
              <Text color="$colorMuted">{insight.summary}</Text>
            </YStack>

            {cards.length ? (
              <YStack gap="$2">
                <ScrollView
                  ref={scrollRef}
                  horizontal
                  showsHorizontalScrollIndicator={false}
                  pagingEnabled={false}
                  snapToInterval={cardWidth + 12}
                  decelerationRate="fast"
                  onTouchStart={beginManualInteraction}
                  onScrollBeginDrag={beginManualInteraction}
                  onMomentumScrollEnd={(event) => {
                    const next = Math.round(event.nativeEvent.contentOffset.x / (cardWidth + 12));
                    setCardIndex(Math.max(0, Math.min(cards.length - 1, next)));
                  }}
                >
                  <XStack gap="$3">
                    {cards.map((card) => (
                      <YStack
                        key={`${card.category}:${card.title}`}
                        width={cardWidth}
                        gap="$2"
                        padding="$3"
                        borderRadius="$4"
                        borderWidth={1}
                        style={{
                          borderColor: semantics.mutedPanelBorder,
                          backgroundColor: semantics.mutedPanelBackground
                        }}
                      >
                        <XStack justifyContent="space-between" alignItems="flex-start" gap="$2">
                          <YStack gap="$1" flex={1}>
                            <Text fontWeight="700">{card.title}</Text>
                            <Text color="$colorMuted">{card.category.replaceAll('_', ' ')}</Text>
                          </YStack>
                          <Text color="$colorMuted">{`${Math.round(card.confidence * 100)}%`}</Text>
                        </XStack>
                        <Text>{card.summary}</Text>
                        <Text color="$colorMuted">{card.recommendation}</Text>
                      </YStack>
                    ))}
                  </XStack>
                </ScrollView>
                <XStack gap="$2" alignItems="center">
                  {cards.map((card, index) => (
                    <YStack
                      key={`${card.category}-dot`}
                      width={index === cardIndex ? 18 : 8}
                      height={8}
                      borderRadius={999}
                      style={{
                        backgroundColor: index === cardIndex ? semantics.chartSolar : semantics.mutedPanelBorder
                      }}
                    />
                  ))}
                </XStack>
              </YStack>
            ) : null}
          </>
        ) : null}
      </YStack>
    </Card>
  );
}

import { useMemo, useRef, useState } from 'react';
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

function cardTrend(cardCategory: string, score: number, semantics: ReturnType<typeof useThemeSemantics>) {
  const directionUp = score >= 0;
  const productionLike =
    cardCategory === 'self_sufficiency' ||
    cardCategory === 'solar' ||
    cardCategory === 'value';

  const favorable = productionLike ? directionUp : !directionUp;
  return {
    glyph: directionUp ? '▲' : '▼',
    color: favorable ? semantics.statusSuccess : semantics.statusDanger
  };
}

type Props = {
  data?: EnergyComparisonInsightResponse;
  loading?: boolean;
};

export function EnergyComparisonWidget({ data, loading = false }: Props) {
  const semantics = useThemeSemantics();
  const { width } = useWindowDimensions();
  const scrollRef = useRef<ScrollView>(null);
  const [cardIndex, setCardIndex] = useState(0);
  const insight = data?.insight;
  const cards = insight?.cards ?? [];
  const wideLayout = width >= 1440 && cards.length <= 4;
  const sidePadding = 4;
  const cardGap = 12;
  const cardWidth = useMemo(() => {
    if (wideLayout) {
      return Math.max(240, Math.floor((width - 80 - cardGap*3) / 4));
    }
    return Math.max(260, Math.min(width - 56, 560));
  }, [cardGap, wideLayout, width]);
  const snapInterval = cardWidth + cardGap;

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
              State of Energy
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
                {wideLayout ? (
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
                          </YStack>
                          <Text
                            fontSize="$4"
                            fontWeight="800"
                            style={{
                              color: cardTrend(card.category, card.score, semantics).color
                            }}
                          >
                            {cardTrend(card.category, card.score, semantics).glyph}
                          </Text>
                        </XStack>
                        <Text>{card.summary}</Text>
                        <Text color="$colorMuted">{card.recommendation}</Text>
                      </YStack>
                    ))}
                  </XStack>
                ) : (
                  <>
                    <ScrollView
                      ref={scrollRef}
                      horizontal
                      showsHorizontalScrollIndicator={false}
                      pagingEnabled={false}
                      snapToInterval={snapInterval}
                      decelerationRate="fast"
                      contentContainerStyle={{ paddingLeft: sidePadding, paddingRight: sidePadding }}
                      onMomentumScrollEnd={(event) => {
                        const next = Math.round(event.nativeEvent.contentOffset.x / snapInterval);
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
                              </YStack>
                              <Text
                                fontSize="$4"
                                fontWeight="800"
                                style={{
                                  color: cardTrend(card.category, card.score, semantics).color
                                }}
                              >
                                {cardTrend(card.category, card.score, semantics).glyph}
                              </Text>
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
                  </>
                )}
              </YStack>
            ) : null}
          </>
        ) : null}
      </YStack>
    </Card>
  );
}

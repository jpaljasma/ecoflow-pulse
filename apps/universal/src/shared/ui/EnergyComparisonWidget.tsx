import { useMemo, useRef, useState } from 'react';
import { ScrollView, useWindowDimensions } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import type { ComponentProps } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import type { EnergyComparisonInsightResponse } from '@/features/energy/api';
import { Card } from '@/shared/ui/Card';
import { useThemeSemantics } from '@/shared/theme/semantic';

const COMPARISON_CARD_GAP = 12;
const COMPARISON_SIDE_PADDING = 4;
const COMPARISON_WIDE_BREAKPOINT = 1000;
const COMPARISON_WIDE_CARD_MIN_WIDTH = 220;
const COMPARISON_SCROLL_CARD_MIN_WIDTH = 260;
const COMPARISON_SCROLL_CARD_MAX_WIDTH = 560;

type SemanticColors = ReturnType<typeof useThemeSemantics>;
type ComparisonInsight = NonNullable<EnergyComparisonInsightResponse['insight']>;
type ComparisonCard = ComparisonInsight['cards'][number];

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

function cardTrend(cardCategory: string, score: number, semantics: SemanticColors) {
  const directionUp = score >= 0;
  const productionLike =
    cardCategory === 'self_sufficiency' ||
    cardCategory === 'solar' ||
    cardCategory === 'value';

  const favorable = productionLike ? directionUp : !directionUp;
  return {
    icon: (directionUp ? 'trending-up' : 'trending-down') as ComponentProps<typeof MaterialCommunityIcons>['name'],
    label: directionUp ? 'Increasing' : 'Decreasing',
    color: favorable ? semantics.statusSuccess : semantics.statusDanger
  };
}

function getComparisonCardWidth({
  availableWidth,
  cardCount,
  wideLayout
}: {
  availableWidth: number;
  cardCount: number;
  wideLayout: boolean;
}) {
  const safeWidth = Math.max(0, availableWidth);
  if (wideLayout) {
    const totalGap = COMPARISON_CARD_GAP * Math.max(cardCount - 1, 0);
    const widthPerCard = Math.floor((safeWidth - totalGap) / Math.max(cardCount, 1));
    return Math.max(COMPARISON_WIDE_CARD_MIN_WIDTH, widthPerCard);
  }
  const scrollWidth = safeWidth - COMPARISON_SIDE_PADDING * 2;
  return Math.max(COMPARISON_SCROLL_CARD_MIN_WIDTH, Math.min(scrollWidth, COMPARISON_SCROLL_CARD_MAX_WIDTH));
}

function EnergyComparisonInsightCard({
  card,
  width,
  semantics
}: {
  card: ComparisonCard;
  width: number;
  semantics: SemanticColors;
}) {
  const trend = cardTrend(card.category, card.score, semantics);

  return (
    <YStack
      testID="energy-comparison-card"
      width={width}
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
        <MaterialCommunityIcons
          name={trend.icon}
          size={20}
          color={trend.color}
          accessibilityLabel={trend.label}
        />
      </XStack>
      <Text>{card.summary}</Text>
      <Text color="$colorMuted">{card.recommendation}</Text>
    </YStack>
  );
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
  const [containerWidth, setContainerWidth] = useState(0);
  const insight = data?.insight;
  const cards = insight?.cards ?? [];
  const availableWidth = containerWidth || width;
  const wideLayout = containerWidth >= COMPARISON_WIDE_BREAKPOINT && cards.length <= 4;
  const cardWidth = useMemo(() => {
    return getComparisonCardWidth({
      availableWidth,
      cardCount: cards.length,
      wideLayout
    });
  }, [availableWidth, cards.length, wideLayout]);
  const snapInterval = cardWidth + COMPARISON_CARD_GAP;

  if (!insight && !loading) {
    return null;
  }

  return (
    <Card
      testID="energy-comparison-widget"
      gap="$3"
      style={{
        backgroundColor: semantics.chartFrameBackground,
        borderColor: semantics.chartFrameBorder
      }}
    >
      <YStack
        gap="$2"
        onLayout={(event) => {
          const nextWidth = Math.round(event.nativeEvent.layout.width);
          setContainerWidth((currentWidth) => (Math.abs(currentWidth - nextWidth) > 1 ? nextWidth : currentWidth));
        }}
      >
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
                  <XStack gap="$3" flexWrap="nowrap" width="100%">
                    {cards.map((card) => (
                      <EnergyComparisonInsightCard
                        key={`${card.category}:${card.title}`}
                        card={card}
                        width={cardWidth}
                        semantics={semantics}
                      />
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
                      contentContainerStyle={{
                        paddingLeft: COMPARISON_SIDE_PADDING,
                        paddingRight: COMPARISON_SIDE_PADDING
                      }}
                      onMomentumScrollEnd={(event) => {
                        const next = Math.round(event.nativeEvent.contentOffset.x / snapInterval);
                        setCardIndex(Math.max(0, Math.min(cards.length - 1, next)));
                      }}
                    >
                      <XStack gap="$3">
                        {cards.map((card) => (
                          <EnergyComparisonInsightCard
                            key={`${card.category}:${card.title}`}
                            card={card}
                            width={cardWidth}
                            semantics={semantics}
                          />
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

import { useEffect, useMemo, useRef, type ComponentProps } from 'react';
import { Animated, Easing, Platform } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import {
  getRollingMetricDirection,
  getRollingMetricDigitTiming,
  tokenizeRollingMetricValue,
  type RollingMetricDirection
} from '@/shared/ui/RollingMetricValueModel';
import { usePrefersReducedMotion } from '@/shared/ui/usePrefersReducedMotion';

export {
  getRollingMetricDirection,
  parseRollingMetricNumber,
  tokenizeRollingMetricValue,
  type RollingMetricDirection,
  type RollingMetricToken
} from '@/shared/ui/RollingMetricValueModel';

type RollingMetricFontWeight = '400' | '500' | '600' | '700' | '800' | '900' | 400 | 500 | 600 | 700 | 800 | 900;

function metricTextStyle(color?: string): ComponentProps<typeof Text>['style'] {
  return {
    ...(Platform.OS === 'web' ? { fontVariantNumeric: 'tabular-nums' } : undefined),
    ...(color ? { color } : undefined)
  } as ComponentProps<typeof Text>['style'];
}

function RollingDigit({
  value,
  previousValue,
  direction,
  disabled,
  fontSize,
  fontWeight,
  lineHeight,
  digitIndex,
  color
}: {
  value: string;
  previousValue?: string;
  direction: RollingMetricDirection;
  disabled: boolean;
  fontSize: number;
  fontWeight: RollingMetricFontWeight;
  lineHeight: number;
  digitIndex: number;
  color?: string;
}) {
  const translateY = useRef(new Animated.Value(0)).current;
  const shouldAnimate = !disabled && direction !== 'none' && Boolean(previousValue) && previousValue !== value;
  const width = Math.max(10, fontSize * 0.58);
  const textStyle = metricTextStyle(color);
  const timing = getRollingMetricDigitTiming(digitIndex);

  useEffect(() => {
    if (!shouldAnimate) {
      translateY.setValue(0);
      return;
    }

    if (direction === 'up') {
      translateY.setValue(0);
      Animated.timing(translateY, {
        toValue: -lineHeight,
        delay: timing.delayMs,
        duration: timing.durationMs,
        easing: Easing.out(Easing.quad),
        useNativeDriver: Platform.OS !== 'web'
      }).start();
      return;
    }

    translateY.setValue(-lineHeight);
    Animated.timing(translateY, {
      toValue: 0,
      delay: timing.delayMs,
      duration: timing.durationMs,
      easing: Easing.out(Easing.quad),
      useNativeDriver: Platform.OS !== 'web'
    }).start();
  }, [direction, lineHeight, shouldAnimate, timing.delayMs, timing.durationMs, translateY, value]);

  if (!shouldAnimate) {
    return (
      <YStack width={width} height={lineHeight} overflow="hidden" alignItems="center" justifyContent="center">
        <Text fontFamily="$body" fontSize={fontSize} lineHeight={lineHeight} fontWeight={fontWeight} style={textStyle} numberOfLines={1}>
          {value}
        </Text>
      </YStack>
    );
  }

  const previous = previousValue ?? value;
  const digits = direction === 'up' ? [previous, value] : [value, previous];

  return (
    <YStack width={width} height={lineHeight} overflow="hidden" alignItems="center">
      <Animated.View style={{ transform: [{ translateY }] }}>
        {digits.map((digit, index) => (
          <Text
            key={`${digit}-${index}`}
            fontFamily="$body"
            fontSize={fontSize}
            lineHeight={lineHeight}
            fontWeight={fontWeight}
            style={textStyle}
            numberOfLines={1}
            accessibilityElementsHidden
            importantForAccessibility="no-hide-descendants"
          >
            {digit}
          </Text>
        ))}
      </Animated.View>
    </YStack>
  );
}

export function RollingMetricValue({
  value,
  animate = true,
  color,
  fontSize = 28,
  fontWeight = '800',
  lineHeight = Math.round(fontSize * 1.1),
  letterSpacing = 0,
  testID
}: {
  value: string;
  animate?: boolean;
  color?: string;
  fontSize?: number;
  fontWeight?: RollingMetricFontWeight;
  lineHeight?: number;
  letterSpacing?: number;
  testID?: string;
}) {
  const previousValueRef = useRef<string | undefined>(undefined);
  const previousValue = previousValueRef.current;
  const prefersReducedMotion = usePrefersReducedMotion();
  const direction = getRollingMetricDirection(previousValue, value);
  const tokens = useMemo(() => tokenizeRollingMetricValue(value), [value]);
  const previousDigitValues = useMemo(
    () => tokenizeRollingMetricValue(previousValue ?? value).filter((token) => token.kind === 'digit').map((token) => token.value),
    [previousValue, value]
  );

  useEffect(() => {
    previousValueRef.current = value;
  }, [value]);

  const disabled = !animate || prefersReducedMotion;
  const staticTextStyle = metricTextStyle(color);

  return (
    <XStack
      testID={testID}
      alignItems="center"
      flexWrap="nowrap"
      accessibilityRole="text"
      accessibilityLabel={value}
    >
      {tokens.map((token, index) => {
        if (token.kind === 'static') {
          return (
            <Text
              key={`static-${index}-${token.value}`}
              fontFamily="$body"
              fontSize={fontSize}
              lineHeight={lineHeight}
              fontWeight={fontWeight}
              letterSpacing={letterSpacing}
              style={staticTextStyle}
              numberOfLines={1}
              accessibilityElementsHidden
              importantForAccessibility="no-hide-descendants"
            >
              {token.value}
            </Text>
          );
        }

        const digitIndex = tokens.slice(0, index + 1).filter((candidate) => candidate.kind === 'digit').length - 1;
        const previousDigit = previousDigitValues[digitIndex];
        return (
          <RollingDigit
            key={`digit-${index}-${token.value}`}
            value={token.value}
            previousValue={previousDigit}
            direction={direction}
            disabled={disabled}
            fontSize={fontSize}
            fontWeight={fontWeight}
            lineHeight={lineHeight}
            digitIndex={digitIndex}
            color={color}
          />
        );
      })}
    </XStack>
  );
}

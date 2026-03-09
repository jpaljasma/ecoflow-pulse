import { memo } from 'react';
import { Platform } from 'react-native';
import * as Linking from 'expo-linking';
import { Button, Text, YStack } from 'tamagui';

export const BatteryUpsellComponent = memo(function BatteryUpsellComponent({
  title,
  summary,
  href,
  ctaLabel,
  loading
}: {
  title?: string;
  summary?: string;
  href?: string;
  ctaLabel?: string;
  loading?: boolean;
}) {
  if (!loading && !href && !summary && !title) return null;

  return (
    <YStack alignItems="center" justifyContent="center" paddingTop="$4" minHeight={loading ? 88 : undefined}>
      {title ? <Text textAlign="center">{title}</Text> : null}
      {summary ? (
        <Text opacity={0.72} textAlign="center" marginTop={title ? '$1' : undefined}>
          {summary}
        </Text>
      ) : loading ? (
        <Text opacity={0.52}>Loading battery recommendation…</Text>
      ) : null}
      {href && ctaLabel ? (
        <Button
          backgroundColor="#22c55e"
          color="white"
          borderColor="#16a34a"
          borderWidth={1}
          borderRadius="$5"
          size="$5"
          minWidth={220}
          minHeight={48}
          marginTop={16}
          paddingHorizontal="$5"
          paddingVertical="$3"
          onPress={() => {
            if (Platform.OS === 'web' && typeof window !== 'undefined') {
              window.open(href, '_blank', 'noopener,noreferrer');
              return;
            }
            void Linking.openURL(href);
          }}
          pressStyle={{ opacity: 0.88 }}
        >
          <Text color="white" fontWeight="700" fontSize={16}>
            🛒 {ctaLabel}
          </Text>
        </Button>
      ) : null}
    </YStack>
  );
});

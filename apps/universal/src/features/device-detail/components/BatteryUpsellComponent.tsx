import { memo } from 'react';
import { Platform } from 'react-native';
import * as Linking from 'expo-linking';
import { Button, Text, YStack } from 'tamagui';

export const BatteryUpsellComponent = memo(function BatteryUpsellComponent({
  href, modelName, batteryCount, maxBatteries
}: {
  href?: string,
  modelName?: string,
  batteryCount: number,
  maxBatteries: number;
}) {
  if (!href) return null;
  let moreBatteries = maxBatteries - batteryCount;
  if (!moreBatteries || moreBatteries < 1) return null;

  return (
    <YStack alignItems="center" justifyContent="center" paddingTop="$4">
      <Text>Your {modelName} supports more batteries!</Text>
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
          <span>🛒</span> Get More Batteries ({moreBatteries})
        </Text>
      </Button>
    </YStack>
  );
});

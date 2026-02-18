import { useState } from 'react';
import { router } from 'expo-router';
import { Button, Text, XStack, YStack } from 'tamagui';
import { Sheet } from '@/shared/ui/Sheet';

export function AppMenu() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        size="$3"
        circular
        onPress={() => setOpen(true)}
        backgroundColor="rgba(120,120,128,0.16)"
        borderColor="rgba(120,120,128,0.3)"
        borderWidth={1}
        pressStyle={{ opacity: 0.85 }}
        aria-label="Open menu"
      >
        <Text fontSize="$4" fontWeight="700">
          ☰
        </Text>
      </Button>

      <Sheet open={open} onOpenChange={setOpen} title="Menu">
        <YStack gap="$3">
          <Button
            size="$4"
            justifyContent="flex-start"
            onPress={() => {
              setOpen(false);
              router.push('/devices');
            }}
          >
            Devices
          </Button>
          <Button
            size="$4"
            justifyContent="flex-start"
            onPress={() => {
              setOpen(false);
              router.push('/settings');
            }}
          >
            Settings
          </Button>
        </YStack>

        <XStack marginTop="$3">
          <Text opacity={0.65} fontSize="$2">
            Universal dashboard navigation
          </Text>
        </XStack>
      </Sheet>
    </>
  );
}

import { useState } from 'react';
import { router } from 'expo-router';
import { Image, Platform, ScrollView, useColorScheme } from 'react-native';
import { Button, Input, Text, XStack, YStack } from 'tamagui';
import { Sheet } from '@/shared/ui/Sheet';
import { getBundledBrandMark } from '@/shared/assets/brandBundled';

export function AppMenu() {
  const [open, setOpen] = useState(false);
  const [searchText, setSearchText] = useState('');
  const scheme = useColorScheme();
  const menuMark = getBundledBrandMark(scheme === 'dark' ? 'dark' : 'light');

  return (
    <>
      <Button
        size="$4"
        onPress={() => setOpen(true)}
        height={36}
        minWidth={56}
        paddingHorizontal="$3"
        paddingVertical="$1"
        paddingBottom="$2"
        alignSelf="flex-start"
        backgroundColor="rgba(120,120,128,0.16)"
        borderColor="rgba(120,120,128,0.45)"
        borderWidth={1}
        borderRadius="$4"
        alignItems="center"
        justifyContent="center"
        pressStyle={{ opacity: 0.85 }}
        style={
          Platform.OS === 'web'
            ? ({
                paddingTop: 10,
                paddingBottom: 6
              } as any)
            : undefined
        }
        aria-label="Open menu"
      >
        <XStack width={24} height={24} alignItems="center" justifyContent="center">
          <Image source={menuMark} style={{ width: 20, height: 20, marginTop: 1 }} resizeMode="contain" />
        </XStack>
      </Button>

      <Sheet open={open} onOpenChange={setOpen} title="Menu">
        <YStack minHeight={360} maxHeight={520} justifyContent="space-between">
          <ScrollView showsVerticalScrollIndicator>
            <YStack gap="$3" paddingRight="$1">
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/devices');
                }}
              >
                Devices
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/settings');
                }}
              >
                Settings
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/(tabs)/search');
                }}
              >
                Search
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/(tabs)/about');
                }}
              >
                About
              </Button>
            </YStack>
          </ScrollView>

          <YStack gap="$3" paddingTop="$3">
            <XStack height={1} backgroundColor="rgba(120,120,128,0.28)" />
            <XStack alignItems="center" gap="$2" width="100%" maxWidth={360}>
              <Input
                flex={1}
                value={searchText}
                onChangeText={setSearchText}
                placeholder="Search"
                size="$5"
                minHeight={56}
                paddingHorizontal={16}
                placeholderTextColor="#a8adb8"
                style={Platform.OS === 'web' ? ({ height: '2em' } as any) : undefined}
              />
              <XStack
                width={56}
                minHeight={56}
                alignItems="center"
                justifyContent="center"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.3)"
                borderRadius={24}
              >
                <Text fontSize="$9" style={Platform.OS === 'web' ? ({ fontSize: '2.4em' } as any) : undefined}>
                  ⌕
                </Text>
              </XStack>
            </XStack>
          </YStack>
        </YStack>
      </Sheet>
    </>
  );
}

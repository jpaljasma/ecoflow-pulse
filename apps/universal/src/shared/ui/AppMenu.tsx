import { useState } from 'react';
import { router } from 'expo-router';
import { Image, Platform, ScrollView } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { LogoutButton } from '@/features/auth/LogoutButton';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Sheet } from '@/shared/ui/Sheet';
import { getBundledBrandMark } from '@/shared/assets/brandBundled';
import { useAppTheme } from '@/shared/theme/useAppTheme';

export function AppMenu() {
  const [open, setOpen] = useState(false);
  const [searchText, setSearchText] = useState('');
  const { isDark } = useAppTheme();
  const menuMark = getBundledBrandMark(isDark ? 'dark' : 'light');

  return (
    <>
      <Button
        size="$4"
        onPress={() => setOpen(true)}
        width={46}
        height={46}
        minWidth={46}
        paddingHorizontal="$0"
        paddingVertical="$0"
        alignSelf="flex-start"
        backgroundColor="rgba(120,120,128,0.16)"
        borderColor="rgba(120,120,128,0.45)"
        borderWidth={1}
        borderRadius={23}
        alignItems="center"
        justifyContent="center"
        pressStyle={{ opacity: 0.85 }}
        style={Platform.OS === 'web' ? ({ paddingTop: 0, paddingBottom: 0 } as any) : undefined}
        aria-label="Open menu"
      >
        <XStack width={26} height={26} alignItems="center" justifyContent="center">
          <Image source={menuMark} style={{ width: 20, height: 20 }} resizeMode="contain" />
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
                  router.push('/profile');
                }}
              >
                Profile
              </Button>
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
                  router.push('/(tabs)/energy');
                }}
              >
                Energy
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
              <LogoutButton
                onComplete={() => {
                  setOpen(false);
                }}
              />
            </YStack>
          </ScrollView>

          <YStack gap="$3" paddingTop="$3">
            <XStack height={1} backgroundColor="rgba(120,120,128,0.28)" />
            <XStack alignItems="center" gap="$2" width="100%" maxWidth={360}>
              <AppTextInput
                flex={1}
                value={searchText}
                onChangeText={setSearchText}
                placeholder="Search"
                placeholderTextColor="#a8adb8"
              />
              <XStack
                width={52}
                minHeight={52}
                alignItems="center"
                justifyContent="center"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.3)"
                borderRadius={20}
              >
                <Text fontSize="$8">
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

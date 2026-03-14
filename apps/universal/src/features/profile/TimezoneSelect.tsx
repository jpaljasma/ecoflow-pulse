import { useEffect, useMemo, useState } from 'react';
import { FlatList, Pressable } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Sheet } from '@/shared/ui/Sheet';
import { searchProfileTimezones } from '@/features/profile/timezone';

type TimezoneSelectProps = {
  value: string;
  onChange: (timezone: string) => void;
  suggestedValue?: string;
  disabled?: boolean;
};

export function TimezoneSelect({ value, onChange, suggestedValue, disabled = false }: TimezoneSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');

  useEffect(() => {
    if (!open) {
      setQuery('');
    }
  }, [open]);

  const timezones = useMemo(
    () => searchProfileTimezones(query, [value, suggestedValue].filter((candidate): candidate is string => Boolean(candidate))),
    [query, suggestedValue, value]
  );

  return (
    <>
      <Button
        unstyled
        disabled={disabled}
        onPress={() => setOpen(true)}
        style={{
          minHeight: 52,
          borderWidth: 1,
          borderColor: 'rgba(76, 186, 161, 0.48)',
          borderRadius: 20,
          paddingHorizontal: 16,
          paddingVertical: 14,
          backgroundColor: 'transparent',
          opacity: disabled ? 0.6 : 1
        }}
      >
        <XStack justifyContent="space-between" alignItems="center" gap="$3">
          <Text fontSize="$5" numberOfLines={1} flex={1}>
            {value}
          </Text>
          <Text color="$colorMuted" fontWeight="700">
            Select
          </Text>
        </XStack>
      </Button>

      <Sheet open={open} onOpenChange={setOpen} title="Choose timezone">
        <YStack gap="$3">
          <AppTextInput
            autoFocus
            value={query}
            onChangeText={setQuery}
            placeholder="Search IANA timezone"
          />
          {suggestedValue && suggestedValue !== value ? (
            <Button
              size="$4"
              onPress={() => {
                onChange(suggestedValue);
                setOpen(false);
              }}
            >
              Use suggested timezone: {suggestedValue}
            </Button>
          ) : null}
          <Text color="$colorMuted">
            Pick a timezone from the official IANA list. Free text entry is disabled.
          </Text>
          <YStack
            borderWidth={1}
            borderColor="rgba(76, 186, 161, 0.24)"
            borderRadius={18}
            overflow="hidden"
            minHeight={240}
            maxHeight={360}
          >
            <FlatList
              data={timezones}
              keyExtractor={(item) => item}
              keyboardShouldPersistTaps="handled"
              renderItem={({ item }) => {
                const selected = item === value;
                return (
                  <Pressable
                    onPress={() => {
                      onChange(item);
                      setOpen(false);
                    }}
                    style={{
                      paddingHorizontal: 16,
                      paddingVertical: 14,
                      backgroundColor: selected ? 'rgba(76, 186, 161, 0.12)' : 'transparent',
                      borderBottomWidth: 1,
                      borderBottomColor: 'rgba(76, 186, 161, 0.12)'
                    }}
                  >
                    <XStack justifyContent="space-between" alignItems="center" gap="$3">
                      <Text fontSize="$5" flex={1}>
                        {item}
                      </Text>
                      {selected ? (
                        <Text color="$colorMuted" fontWeight="700">
                          Selected
                        </Text>
                      ) : null}
                    </XStack>
                  </Pressable>
                );
              }}
            />
          </YStack>
        </YStack>
      </Sheet>
    </>
  );
}

import { Text, XStack, YStack } from 'tamagui';

export function TopBar({
  title,
  subtitle,
  right
}: {
  title: string;
  subtitle?: string;
  right?: React.ReactNode;
}) {
  return (
    <XStack
      alignItems="center"
      justifyContent="space-between"
      paddingHorizontal="$4"
      paddingVertical="$3"
      gap="$3"
    >
      <YStack gap="$1" flex={1}>
        <Text fontSize="$7" fontWeight="800" letterSpacing={-0.4}>
          {title}
        </Text>
        {subtitle ? (
          <Text fontSize="$3" opacity={0.72}>
            {subtitle}
          </Text>
        ) : null}
      </YStack>
      {right}
    </XStack>
  );
}

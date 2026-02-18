import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';

export function ListRow({
  title,
  subtitle,
  right
}: {
  title: string;
  subtitle?: string;
  right?: React.ReactNode;
}) {
  return (
    <Card>
      <XStack alignItems="center" justifyContent="space-between" gap="$3">
        <YStack flex={1} gap="$1">
          <Text fontSize="$4" fontWeight="600">
            {title}
          </Text>
          {subtitle ? (
            <Text fontSize="$2" opacity={0.7}>
              {subtitle}
            </Text>
          ) : null}
        </YStack>
        {right}
      </XStack>
    </Card>
  );
}

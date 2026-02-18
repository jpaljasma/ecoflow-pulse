import { Modal, Pressable } from 'react-native';
import { Text, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';

export function Sheet({
  open,
  onOpenChange,
  title,
  children
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Modal
      transparent
      animationType="slide"
      visible={open}
      onRequestClose={() => onOpenChange(false)}
    >
      <Pressable
        style={{
          flex: 1,
          backgroundColor: 'rgba(0,0,0,0.35)',
          justifyContent: 'flex-end'
        }}
        onPress={() => onOpenChange(false)}
      >
        <Pressable onPress={(e) => e.stopPropagation()}>
          <YStack
            maxHeight="85%"
            paddingHorizontal="$3"
            paddingBottom="$5"
            paddingTop="$2"
          >
            <Card gap="$3" padding="$4">
              <YStack gap="$2">
                <Text fontSize="$6" fontWeight="700">
                  {title}
                </Text>
              </YStack>
              {children}
            </Card>
          </YStack>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

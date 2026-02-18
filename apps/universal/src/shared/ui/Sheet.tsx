import { Sheet as TamaguiSheet, Text, YStack } from 'tamagui';

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
    <TamaguiSheet open={open} onOpenChange={onOpenChange} modal snapPoints={[85]} dismissOnSnapToBottom>
      <TamaguiSheet.Overlay />
      <TamaguiSheet.Frame padding="$4" gap="$3">
        <YStack gap="$2">
          <Text fontSize="$6" fontWeight="700">
            {title}
          </Text>
        </YStack>
        {children}
      </TamaguiSheet.Frame>
    </TamaguiSheet>
  );
}

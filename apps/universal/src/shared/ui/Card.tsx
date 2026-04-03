import { styled, YStack } from 'tamagui';

export const Card = styled(YStack, {
  name: 'Card',
  backgroundColor: '$backgroundElevated',
  borderWidth: 1,
  borderColor: '$borderColor',
  borderRadius: '$4',
  padding: '$5',
  shadowColor: '$shadowColor',
  shadowOpacity: 0.06,
  shadowRadius: 18,
  shadowOffset: { width: 0, height: 10 }
});

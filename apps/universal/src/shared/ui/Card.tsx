import { styled, YStack } from 'tamagui';
import {
  PULSE_PANEL_PADDING,
  PULSE_PANEL_RADIUS
} from '@/shared/ui/navigationShellMetrics';

export const Card = styled(YStack, {
  name: 'Card',
  backgroundColor: '$backgroundElevated',
  borderWidth: 1,
  borderColor: '$borderColor',
  borderRadius: PULSE_PANEL_RADIUS,
  padding: PULSE_PANEL_PADDING,
  shadowColor: '$shadowColor',
  shadowOpacity: 0.04,
  shadowRadius: 12,
  shadowOffset: { width: 0, height: 6 }
});

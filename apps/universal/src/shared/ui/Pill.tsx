import { Text, XStack } from 'tamagui';

const tones = {
  neutral: { bg: 'rgba(120,120,128,0.16)', color: '$color' },
  success: { bg: 'rgba(48,209,88,0.2)', color: '#30d158' },
  warning: { bg: 'rgba(255,159,10,0.2)', color: '#ff9f0a' },
  danger: { bg: 'rgba(255,69,58,0.2)', color: '#ff453a' },
  info: { bg: 'rgba(10,132,255,0.2)', color: '#0a84ff' }
} as const;

export function Pill({
  label,
  tone = 'neutral',
  glyph = false
}: {
  label: string;
  tone?: keyof typeof tones;
  glyph?: boolean;
}) {
  return (
    <XStack
      alignItems="center"
      justifyContent="center"
      paddingHorizontal="$3"
      paddingVertical="$2"
      borderRadius="$5"
      backgroundColor={tones[tone].bg}
    >
      <Text
        fontFamily="$body"
        fontSize={glyph ? '$6' : '$3'}
        lineHeight={glyph ? 26 : undefined}
        fontWeight="700"
        color={tones[tone].color as any}
      >
        {label}
      </Text>
    </XStack>
  );
}

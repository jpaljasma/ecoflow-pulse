import { Input, type InputProps } from 'tamagui';

type AppTextInputProps = InputProps & {
  compact?: boolean;
};

export function AppTextInput({ compact = false, ...props }: AppTextInputProps) {
  return (
    <Input
      size="$5"
      minHeight={compact ? 44 : 52}
      paddingHorizontal={16}
      paddingVertical={compact ? 10 : 14}
      borderRadius={compact ? 18 : 20}
      {...props}
    />
  );
}

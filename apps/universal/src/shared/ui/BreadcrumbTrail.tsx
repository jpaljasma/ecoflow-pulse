import type { ComponentProps } from 'react';
import { Pressable } from 'react-native';
import { useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Text, XStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

export type BreadcrumbItem = {
  label: string;
  href?: string;
  icon?: ComponentProps<typeof MaterialCommunityIcons>['name'];
  hideLabel?: boolean;
  current?: boolean;
};

export function BreadcrumbTrail({
  items
}: {
  items: BreadcrumbItem[];
}) {
  const router = useRouter();
  const semantics = useThemeSemantics();

  return (
    <XStack alignItems="center" gap="$2" flexWrap="wrap">
      {items.map((item, index) => {
        const isLast = item.current || index === items.length - 1;
        const href = item.href;
        const color = isLast ? semantics.navItemActiveText : semantics.navSectionLabel;
        const content = (
          <XStack alignItems="center" gap="$2">
            {item.icon ? <MaterialCommunityIcons name={item.icon} size={14} color={color} /> : null}
            {item.hideLabel ? null : (
              <Text
                fontSize="$2"
                fontWeight={isLast ? '700' : '600'}
                numberOfLines={1}
                style={{ color }}
              >
                {item.label}
              </Text>
            )}
          </XStack>
        );

        return (
          <XStack key={`${item.label}-${index}`} alignItems="center" gap="$2">
            {href && !isLast ? (
              <Pressable
                accessibilityRole="button"
                accessibilityLabel={item.label}
                onPress={() => router.replace(href)}
              >
                {content}
              </Pressable>
            ) : (
              content
            )}
            {index < items.length - 1 ? (
              <MaterialCommunityIcons
                name="chevron-right"
                size={14}
                color={semantics.navSectionLabel}
              />
            ) : null}
          </XStack>
        );
      })}
    </XStack>
  );
}

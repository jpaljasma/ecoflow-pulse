import type { ReactNode } from 'react';
import { XStack, YStack } from 'tamagui';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { PulseSidebarNav, type PulsePrimaryNavKey } from '@/shared/ui/PulseSidebarNav';

export function SecondaryPageShell({
  activeNavKey,
  children
}: {
  activeNavKey: PulsePrimaryNavKey;
  children: ReactNode;
}) {
  const { isSidebarMode } = useNavigationShellMetrics();

  if (!isSidebarMode) {
    return <>{children}</>;
  }

  return (
    <XStack flex={1} backgroundColor="$background">
      <PulseSidebarNav activeKey={activeNavKey} />
      <YStack flex={1} minWidth={0}>
        {children}
      </YStack>
    </XStack>
  );
}

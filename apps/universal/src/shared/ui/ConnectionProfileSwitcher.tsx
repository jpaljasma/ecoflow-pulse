import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { type ConnectionProfileId, readConnectionProfiles } from '@/shared/config/env';
import { useConnectionProfileStore } from '@/shared/config/connectionProfileStore';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';

type ConnectionProfileSwitcherProps = {
  variant?: 'compact' | 'detailed';
  onSelectionChange?: (profileId: ConnectionProfileId) => void;
};

type ConnectionProfilePresentation = {
  iconName: ComponentProps<typeof MaterialCommunityIcons>['name'];
  title: string;
  statusDescription: string;
  detailedDescription: string;
  compactDescription: string;
};

function describeConnectionProfile(profileId: ConnectionProfileId): ConnectionProfilePresentation {
  if (profileId === 'cloud') {
    return {
      iconName: 'cloud-outline',
      title: 'Cloud',
      statusDescription: 'Hosted stack',
      detailedDescription: 'Use the hosted HTTPS API, websocket gateway, and cloud OIDC issuer.',
      compactDescription: 'Hosted API, realtime, and auth.'
    };
  }

  return {
    iconName: 'laptop',
    title: 'k3d',
    statusDescription: 'Local stack',
    detailedDescription: 'Use your local k3d HTTPS edge and development services on this machine or LAN.',
    compactDescription: 'Local k3d edge and development services.'
  };
}

export function ConnectionProfileSwitcher({
  variant = 'detailed',
  onSelectionChange
}: ConnectionProfileSwitcherProps) {
  const connectionProfileId = useConnectionProfileStore((state) => state.profileId);
  const setConnectionProfileId = useConnectionProfileStore((state) => state.setProfileId);
  const connectionProfiles = readConnectionProfiles();
  const { spec, isDark } = useAppTheme();
  const isCompact = variant === 'compact';

  return (
    <XStack gap="$3" flexWrap="wrap" alignItems="stretch">
      {(Object.keys(connectionProfiles) as ConnectionProfileId[]).map((profileId) => {
        const profile = connectionProfiles[profileId];
        const presentation = describeConnectionProfile(profileId);
        const selected = profileId === connectionProfileId;
        const disabled = !profile.configured;
        const cardBackground = selected
          ? hexToRgba(spec.colors.accentColor, isDark ? 0.12 : 0.08)
          : hexToRgba(spec.colors.backgroundHover, isDark ? 0.58 : 0.72);
        const cardBorderColor = selected
          ? hexToRgba(spec.colors.accentColor, isDark ? 0.92 : 0.82)
          : hexToRgba(spec.colors.borderColor, isDark ? 0.72 : 0.82);

        return (
          <Button
            key={profileId}
            unstyled
            disabled={disabled}
            onPress={() => {
              setConnectionProfileId(profileId);
              onSelectionChange?.(profileId);
            }}
            flexGrow={1}
            flexBasis={isCompact ? 0 : '100%'}
            minWidth={isCompact ? 132 : 280}
            borderRadius="$3"
            borderWidth={selected ? 1.5 : 1}
            padding={isCompact ? '$3' : '$4'}
            minHeight={isCompact ? 112 : 140}
            justifyContent="flex-start"
            style={{
              borderColor: cardBorderColor,
              backgroundColor: cardBackground,
              opacity: disabled ? 0.56 : 1
            }}
            hoverStyle={{
              y: disabled ? 0 : -2,
              opacity: disabled ? 0.56 : 1
            }}
            pressStyle={{
              scale: disabled ? 1 : 0.995
            }}
          >
            <YStack flex={1} gap={isCompact ? '$2' : '$3'}>
              <XStack justifyContent="space-between" alignItems="center" gap="$3">
                <XStack alignItems="center" gap="$3" flex={1}>
                  <YStack
                    width={isCompact ? 38 : 42}
                    height={isCompact ? 38 : 42}
                    borderRadius="$4"
                    alignItems="center"
                    justifyContent="center"
                    backgroundColor="$background"
                    borderWidth={1}
                    borderColor="$borderColor"
                  >
                    <MaterialCommunityIcons
                      name={presentation.iconName}
                      size={isCompact ? 18 : 20}
                      color={selected ? spec.colors.accentColor : spec.colors.colorMuted}
                    />
                  </YStack>
                  <YStack gap="$1" flex={1}>
                    <Text fontSize={isCompact ? '$4' : '$5'} fontWeight="800" letterSpacing={-0.1}>
                      {presentation.title}
                    </Text>
                    <Text fontSize="$2" color="$colorMuted">
                      {disabled
                        ? 'Not configured'
                        : selected
                          ? 'Selected'
                          : presentation.statusDescription}
                    </Text>
                  </YStack>
                </XStack>
                <Text fontSize="$2" fontWeight="700" color={selected ? '$accentColor' : '$colorMuted'}>
                  {selected ? 'Active' : disabled ? 'Unavailable' : 'Switch'}
                </Text>
              </XStack>
              <Text fontSize="$3" lineHeight={isCompact ? 20 : 22} color="$colorMuted">
                {isCompact ? presentation.compactDescription : presentation.detailedDescription}
              </Text>
              {isCompact ? null : (
                <Text fontSize="$2" color="$colorMuted" numberOfLines={1}>
                  API: {profile.apiUrl || 'Not configured'}
                </Text>
              )}
            </YStack>
          </Button>
        );
      })}
    </XStack>
  );
}

export function ConnectionProfileHint() {
  const connectionProfiles = readConnectionProfiles();
  const semantics = useThemeSemantics();

  if (connectionProfiles.cloud.configured) {
    return (
      <Text fontSize="$2" color="$colorMuted">
        Switching data source recreates realtime and may ask you to sign in again when the auth issuer changes.
      </Text>
    );
  }

  return (
    <Text fontSize="$2" style={{ color: semantics.subtleText }}>
      Configure `EXPO_PUBLIC_CLOUD_API_URL`, `EXPO_PUBLIC_CLOUD_WS_URL`, and the matching cloud OIDC values to enable the cloud switch.
    </Text>
  );
}

function hexToRgba(hex: string, alpha: number): string {
  const normalized = hex.replace('#', '');
  const value = normalized.length === 3
    ? normalized
        .split('')
        .map((part) => part + part)
        .join('')
    : normalized;
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

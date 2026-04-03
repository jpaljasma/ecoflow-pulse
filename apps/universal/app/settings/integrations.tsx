import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'expo-router';
import { Animated, ScrollView } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import {
  useCreateIntegration,
  useIntegrations,
  useSetIntegrationActive,
  useUpdateIntegration
} from '@/features/integrations/hooks';
import type { Integration } from '@/features/integrations/schema';
import { ApiError } from '@/shared/api/restClient';
import { AppMenu } from '@/shared/ui/AppMenu';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { Card } from '@/shared/ui/Card';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { SecondaryPageShell } from '@/shared/ui/SecondaryPageShell';
import { TopBar } from '@/shared/ui/TopBar';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';
import { useThemeSemantics } from '@/shared/theme/semantic';

const ecoFlowProvider = 'ecoflow';

const CONNECTOR_COPY = {
  title: 'EcoFlow',
  description:
    'Connect your EcoFlow account keys, keep backup credentials inactive, and let Pulse validate provider access plus MQTT before switching the live connector.',
  icon: 'transmission-tower-export' as const
};

export default function IntegrationSettingsScreen() {
  const router = useRouter();
  const {
    compactHeader,
    horizontalPadding,
    isDesktop,
    isSidebarMode,
    layoutMaxWidth
  } = usePageLayoutMetrics();
  const { authReady, allowed, waiting } = useRequireAuth();
  const { token, authKey } = useAuthSession();
  const semantics = useThemeSemantics();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  const integrationsQuery = useIntegrations({
    token,
    authKey,
    enabled: authReady && allowed,
    provider: ecoFlowProvider
  });
  const createIntegration = useCreateIntegration({ token, authKey });
  const updateIntegration = useUpdateIntegration({ token, authKey });
  const setIntegrationActive = useSetIntegrationActive({ token, authKey });

  const integrations = useMemo(
    () => integrationsQuery.data?.integrations ?? [],
    [integrationsQuery.data?.integrations]
  );
  const activeIntegration = useMemo(
    () => integrations.find((item) => item.isActive) ?? null,
    [integrations]
  );
  const hasConfiguredConnector = integrations.length > 0;
  const [selectedCredentialId, setSelectedCredentialId] = useState<string>('new');
  const [accessKey, setAccessKey] = useState('');
  const [accessSecret, setAccessSecret] = useState('');
  const [activateOnSave, setActivateOnSave] = useState(true);
  const [flashMessage, setFlashMessage] = useState('');

  useEffect(() => {
    const firstIntegration = integrations[0];
    if (integrations.length === 0) {
      setSelectedCredentialId('new');
      return;
    }
    if (selectedCredentialId === 'new') {
      setSelectedCredentialId(activeIntegration?.id ?? firstIntegration?.id ?? 'new');
      return;
    }
    const exists = integrations.some((integration) => integration.id === selectedCredentialId);
    if (!exists) {
      setSelectedCredentialId(activeIntegration?.id ?? firstIntegration?.id ?? 'new');
    }
  }, [activeIntegration?.id, integrations, selectedCredentialId]);

  useEffect(() => {
    const selected = integrations.find((integration) => integration.id === selectedCredentialId) ?? null;
    setAccessKey('');
    setAccessSecret('');
    setActivateOnSave(selected?.isActive ?? true);
    setFlashMessage('');
  }, [integrations, selectedCredentialId]);

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

  const selectedIntegration =
    integrations.find((integration) => integration.id === selectedCredentialId) ?? null;
  const pending =
    createIntegration.isPending || updateIntegration.isPending || setIntegrationActive.isPending;
  const creatingNewCredential = selectedIntegration === null;
  const activeConnectionLabel = activeIntegration?.accessKeyMask ?? 'No active key';
  const connectorStatusLabel = activeIntegration
    ? 'Connected'
    : hasConfiguredConnector
      ? 'Saved credentials'
      : 'Not configured';
  const detailTitle = selectedIntegration
    ? selectedIntegration.isActive
      ? `Rotate ${selectedIntegration.accessKeyMask}`
      : `Review ${selectedIntegration.accessKeyMask}`
    : hasConfiguredConnector
      ? 'Add another saved key'
      : 'Configure EcoFlow';
  const detailDescription = selectedIntegration
    ? selectedIntegration.isActive
      ? 'Replace the active key material. Pulse will validate the replacement before switching the live connection.'
      : 'Activate this saved key as-is or rotate its secret before making it live.'
    : hasConfiguredConnector
      ? 'Store another EcoFlow credential as a fallback or future replacement.'
      : 'Paste your EcoFlow Access Key and Access Secret to create the first configured connection.';

  const handleSave = async () => {
    setFlashMessage('');
    try {
      if (selectedIntegration) {
        const updated = await updateIntegration.mutateAsync({
          credentialId: selectedIntegration.id,
          values: {
            accessKey,
            accessSecret,
            isActive: activateOnSave
          }
        });
        setSelectedCredentialId(updated.id);
        setFlashMessage(
          activateOnSave
            ? 'EcoFlow credentials validated and applied to your existing enabled devices.'
            : 'EcoFlow credentials updated without activating this saved connection.'
        );
      } else {
        const created = await createIntegration.mutateAsync({
          provider: ecoFlowProvider,
          accessKey,
          accessSecret,
          isActive: activateOnSave
        });
        setSelectedCredentialId(created.id);
        setFlashMessage(
          activateOnSave
            ? 'EcoFlow connection verified and activated.'
            : 'EcoFlow connection saved as inactive.'
        );
      }
      setAccessKey('');
      setAccessSecret('');
    } catch (error) {
      setFlashMessage(describeIntegrationError(error));
    }
  };

  const handleSetActive = async (integration: Integration) => {
    setFlashMessage('');
    try {
      await setIntegrationActive.mutateAsync({
        credentialId: integration.id,
        values: { isActive: true }
      });
      setSelectedCredentialId(integration.id);
      setFlashMessage('Connection verified and switched to the active EcoFlow integration.');
    } catch (error) {
      setFlashMessage(describeIntegrationError(error));
    }
  };

  return (
    <Animated.View style={containerStyle}>
      <SecondaryPageShell activeNavKey="settings">
        <YStack
          flex={1}
          backgroundColor="$background"
          paddingHorizontal={horizontalPadding}
          paddingVertical="$4"
          gap="$4"
        >
          <TopBar
            left={isSidebarMode ? undefined : <CloseToHomeButton onClose={closeToHome} />}
            eyebrow={(
              <BreadcrumbTrail
                items={[
                  {
                    label: 'Home',
                    href: '/(tabs)/devices',
                    icon: 'home-variant-outline',
                    hideLabel: true
                  },
                  {
                    label: 'Settings',
                    href: '/(tabs)/settings'
                  },
                  {
                    label: 'Integrations',
                    current: true
                  }
                ]}
              />
            )}
            title="Integrations"
            subtitle="Connector catalog, credential rotation, and activation validation."
            titleFlex={compactHeader ? 1 : 3}
            rightFlex={compactHeader ? 0 : 1}
            right={<AppMenu />}
          />

          <ScrollView
            style={{ flex: 1 }}
            contentContainerStyle={{ paddingBottom: 24, alignItems: 'center' }}
            showsVerticalScrollIndicator
          >
            <YStack gap="$4" width="100%" maxWidth={layoutMaxWidth}>
              <Card
                padding={isDesktop ? '$6' : '$5'}
                gap="$4"
                style={{
                  backgroundColor: semantics.energyCardBackground,
                  borderColor: semantics.energyCardBorder
                }}
              >
                <XStack justifyContent="space-between" alignItems="flex-start" gap="$4" flexWrap="wrap">
                  <YStack gap="$3" flex={1} minWidth={280}>
                    <XStack alignItems="center" gap="$3">
                      <YStack
                        width={54}
                        height={54}
                        borderRadius={20}
                        alignItems="center"
                        justifyContent="center"
                        borderWidth={1}
                        style={{
                          backgroundColor: semantics.tileBackground,
                          borderColor: semantics.energyCardBorder
                        }}
                      >
                        <MaterialCommunityIcons
                          name={CONNECTOR_COPY.icon}
                          size={24}
                          color={semantics.actionText}
                        />
                      </YStack>
                      <YStack gap="$2">
                        <Text fontSize={isDesktop ? '$7' : '$6'} fontWeight="800" letterSpacing={-0.3}>
                          Integration workspace
                        </Text>
                        <Text color="$colorMuted">
                          Manage one active connection per connector with safe validation before cutover.
                        </Text>
                      </YStack>
                    </XStack>

                    <Text color="$colorMuted" lineHeight={24}>
                      Pulse keeps connector configuration product-grade: saved fallback keys stay available,
                      duplicate provider keys are rejected, and activation always reuses the same provider plus
                      MQTT validation path before switching away from the current live connection.
                    </Text>
                  </YStack>

                  <XStack gap="$3" flexWrap="wrap" alignItems="stretch">
                    <SummaryMetric
                      label="Configured"
                      value={hasConfiguredConnector ? '1 connector' : '0 connectors'}
                      detail={CONNECTOR_COPY.title}
                    />
                    <SummaryMetric
                      label="Active key"
                      value={activeConnectionLabel}
                      detail={connectorStatusLabel}
                    />
                    <SummaryMetric
                      label="Saved credentials"
                      value={String(integrations.length)}
                      detail={integrations.length === 0 ? 'Add your first key' : 'Fallbacks stay inactive'}
                    />
                  </XStack>
                </XStack>
              </Card>

              <XStack
                gap="$4"
                alignItems="stretch"
                flexWrap={isDesktop ? 'nowrap' : 'wrap'}
              >
                <Card
                  flex={0.92}
                  minWidth={isDesktop ? 320 : 300}
                  gap="$4"
                  padding={isDesktop ? '$5' : '$4'}
                  backgroundColor="$backgroundElevated"
                >
                  <YStack gap="$2">
                    <Text fontSize="$6" fontWeight="800">Configured integrations</Text>
                    <Text color="$colorMuted">
                      Select a connector to review its live status, saved credentials, and activation flow.
                    </Text>
                  </YStack>

                  <ConnectorInventoryItem
                    title={CONNECTOR_COPY.title}
                    description={CONNECTOR_COPY.description}
                    icon={CONNECTOR_COPY.icon}
                    configured={hasConfiguredConnector}
                    activeLabel={activeIntegration?.accessKeyMask}
                    savedCount={integrations.length}
                    selected
                    onPress={() =>
                      setSelectedCredentialId(
                        hasConfiguredConnector ? activeIntegration?.id ?? integrations[0]?.id ?? 'new' : 'new'
                      )
                    }
                  />

                  <YStack
                    gap="$2"
                    padding="$4"
                    borderRadius="$5"
                    borderWidth={1}
                    style={{
                      backgroundColor: semantics.tileBackground,
                      borderColor: semantics.tileBorder
                    }}
                  >
                    <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.7} color="$colorMuted">
                      Connector catalog
                    </Text>
                    <Text fontSize="$5" fontWeight="800">
                      EcoFlow is available in Pulse
                    </Text>
                    <Text color="$colorMuted" lineHeight={22}>
                      Native provider integration with discovery, saved credential rotation, and MQTT validation.
                    </Text>
                    <XStack gap="$2" flexWrap="wrap">
                      <SubtleBadge label={hasConfiguredConnector ? 'Configured' : 'Available'} />
                      <SubtleBadge label="MQTT validation" />
                      <SubtleBadge label="One active key" />
                    </XStack>
                  </YStack>
                </Card>

                <Card
                  flex={1.3}
                  minWidth={isDesktop ? 460 : 320}
                  gap="$4"
                  padding={isDesktop ? '$5' : '$4'}
                  backgroundColor="$backgroundElevated"
                >
                  <XStack justifyContent="space-between" alignItems="flex-start" gap="$4" flexWrap="wrap">
                    <YStack gap="$2" flex={1} minWidth={260}>
                      <XStack alignItems="center" gap="$3">
                        <YStack
                          width={44}
                          height={44}
                          borderRadius={16}
                          alignItems="center"
                          justifyContent="center"
                          borderWidth={1}
                          style={{
                            backgroundColor: semantics.tileBackground,
                            borderColor: semantics.tileBorder
                          }}
                        >
                          <MaterialCommunityIcons
                            name={CONNECTOR_COPY.icon}
                            size={22}
                            color={semantics.actionText}
                          />
                        </YStack>
                        <YStack gap="$1">
                          <Text fontSize="$6" fontWeight="800">{CONNECTOR_COPY.title}</Text>
                          <Text color="$colorMuted">
                            {hasConfiguredConnector
                              ? 'Configured connector with saved credential history'
                              : 'Not configured yet'}
                          </Text>
                        </YStack>
                      </XStack>
                    </YStack>

                    <XStack gap="$2" flexWrap="wrap">
                      <StatusPill
                        label={hasConfiguredConnector ? 'Configured' : 'Available'}
                        icon={hasConfiguredConnector ? 'check-decagram' : 'plus-circle-outline'}
                        backgroundColor={hasConfiguredConnector ? semantics.energyCardBackground : semantics.tileBackground}
                        borderColor={hasConfiguredConnector ? semantics.energyCardBorder : semantics.tileBorder}
                        textColor={hasConfiguredConnector ? semantics.statusSuccess : semantics.subtleStrongText}
                      />
                      <StatusPill
                        label={connectorStatusLabel}
                        icon={activeIntegration ? 'connection' : 'pause-circle-outline'}
                        backgroundColor={activeIntegration ? semantics.periodActiveBackground : semantics.tileBackground}
                        borderColor={activeIntegration ? semantics.periodActiveBorder : semantics.tileBorder}
                        textColor={activeIntegration ? semantics.periodActiveText : semantics.subtleStrongText}
                      />
                    </XStack>
                  </XStack>

                  {flashMessage ? (
                    <FlashNotice message={flashMessage} />
                  ) : null}

                  {integrationsQuery.isLoading ? (
                    <BrandedLoadingState minHeight={220} message="Loading integrations…" />
                  ) : (
                    <>
                      <XStack gap="$3" flexWrap="wrap">
                        <DetailMetric
                          label="Active connection"
                          value={activeConnectionLabel}
                          detail={activeIntegration ? 'Currently serving enabled devices' : 'No live connection yet'}
                        />
                        <DetailMetric
                          label="Saved credentials"
                          value={String(integrations.length)}
                          detail={integrations.length === 1 ? '1 saved key' : `${integrations.length} saved keys`}
                        />
                        <DetailMetric
                          label="Validation"
                          value="Provider + MQTT"
                          detail="Runs before activation"
                        />
                      </XStack>

                      <YStack gap="$3">
                        <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
                          <YStack gap="$1">
                            <Text fontSize="$5" fontWeight="800">Saved credentials</Text>
                            <Text color="$colorMuted">
                              Review active and fallback keys for this connector.
                            </Text>
                          </YStack>
                          <Button
                            size="$3"
                            chromeless={creatingNewCredential}
                            themeInverse={!creatingNewCredential}
                            onPress={() => setSelectedCredentialId('new')}
                            icon={<MaterialCommunityIcons name="plus" size={18} color={creatingNewCredential ? semantics.actionText : '#f8fffb'} />}
                          >
                            Add key
                          </Button>
                        </XStack>

                        {integrations.length === 0 ? (
                          <YStack
                            gap="$2"
                            minHeight={152}
                            justifyContent="center"
                            padding="$4"
                            borderRadius="$5"
                            borderWidth={1}
                            style={{
                              backgroundColor: semantics.tileBackground,
                              borderColor: semantics.tileBorder
                            }}
                          >
                            <Text fontSize="$5" fontWeight="800">No saved credentials yet</Text>
                            <Text color="$colorMuted">
                              Add your first EcoFlow key pair and Pulse will validate it before activation.
                            </Text>
                          </YStack>
                        ) : (
                          <YStack gap="$3">
                            {integrations.map((integration) => (
                              <CredentialInventoryRow
                                key={integration.id}
                                integration={integration}
                                pending={pending}
                                selected={selectedCredentialId === integration.id}
                                onSelect={() => setSelectedCredentialId(integration.id)}
                                onActivate={() => handleSetActive(integration)}
                              />
                            ))}
                          </YStack>
                        )}
                      </YStack>

                      {selectedIntegration && !selectedIntegration.isActive ? (
                        <YStack
                          gap="$3"
                          padding="$4"
                          borderRadius="$5"
                          borderWidth={1}
                          style={{
                            backgroundColor: semantics.tileBackground,
                            borderColor: semantics.tileBorder
                          }}
                        >
                          <YStack gap="$1">
                            <Text fontWeight="800">Activate saved connection</Text>
                            <Text color="$colorMuted">
                              Reuse the stored keys for {selectedIntegration.accessKeyMask} and run the same
                              provider plus MQTT validation before switching away from the current active connection.
                            </Text>
                          </YStack>
                          <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
                            <Text color="$colorMuted">No key re-entry needed for activation.</Text>
                            <Button
                              size="$4"
                              themeInverse
                              disabled={pending}
                              onPress={() => handleSetActive(selectedIntegration)}
                              icon={
                                pending
                                  ? undefined
                                  : (
                                    <MaterialCommunityIcons
                                      name="connection"
                                      size={18}
                                      color="#f8fffb"
                                    />
                                  )
                              }
                            >
                              {pending ? 'Verifying…' : 'Activate saved connection'}
                            </Button>
                          </XStack>
                        </YStack>
                      ) : null}

                      <YStack gap="$3">
                        <YStack gap="$2">
                          <Text fontSize="$5" fontWeight="800">{detailTitle}</Text>
                          <Text color="$colorMuted" lineHeight={22}>
                            {detailDescription}
                          </Text>
                        </YStack>

                        <YStack gap="$2">
                          <Text fontWeight="700">Access Key</Text>
                          <AppTextInput
                            value={accessKey}
                            onChangeText={setAccessKey}
                            autoCapitalize="none"
                            autoCorrect={false}
                            placeholder={selectedIntegration ? 'Paste replacement Access Key' : 'Paste Access Key'}
                          />
                        </YStack>

                        <YStack gap="$2">
                          <Text fontWeight="700">Access Secret</Text>
                          <AppTextInput
                            value={accessSecret}
                            onChangeText={setAccessSecret}
                            autoCapitalize="none"
                            autoCorrect={false}
                            secureTextEntry
                            placeholder={selectedIntegration ? 'Paste replacement Access Secret' : 'Paste Access Secret'}
                          />
                        </YStack>

                        <YStack gap="$2">
                          <Text fontWeight="700">Activation behavior</Text>
                          <XStack gap="$2" flexWrap="wrap">
                            <Button
                              size="$4"
                              themeInverse={activateOnSave}
                              chromeless={!activateOnSave}
                              onPress={() => setActivateOnSave(true)}
                            >
                              Validate and activate
                            </Button>
                            <Button
                              size="$4"
                              themeInverse={!activateOnSave}
                              chromeless={activateOnSave}
                              onPress={() => setActivateOnSave(false)}
                            >
                              Save inactive
                            </Button>
                          </XStack>
                          <Text color="$colorMuted">
                            Activation runs provider discovery and MQTT checks for the EcoFlow devices already enabled on this account.
                          </Text>
                        </YStack>

                        <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
                          <Text color="$colorMuted">
                            {selectedIntegration
                              ? selectedIntegration.isActive
                                ? `Editing ${selectedIntegration.accessKeyMask}`
                                : `Reviewing ${selectedIntegration.accessKeyMask}`
                              : hasConfiguredConnector
                                ? 'Adding a saved fallback credential'
                                : 'Creating the first connector credential'}
                          </Text>
                          <Button
                            size="$4"
                            themeInverse
                            disabled={pending || accessKey.trim().length === 0 || accessSecret.trim().length === 0}
                            onPress={handleSave}
                            icon={
                              pending
                                ? undefined
                                : (
                                  <MaterialCommunityIcons
                                    name={selectedIntegration ? 'content-save-outline' : 'plus-circle-outline'}
                                    size={18}
                                    color="#f8fffb"
                                  />
                                )
                            }
                          >
                            {pending
                              ? 'Verifying…'
                              : selectedIntegration
                                ? activateOnSave ? 'Validate and apply' : 'Save changes'
                                : activateOnSave ? 'Validate and connect' : 'Save connection'}
                          </Button>
                        </XStack>
                      </YStack>
                    </>
                  )}
                </Card>
              </XStack>
            </YStack>
          </ScrollView>
        </YStack>
      </SecondaryPageShell>
    </Animated.View>
  );
}

function SummaryMetric({
  label,
  value,
  detail
}: {
  label: string;
  value: string;
  detail: string;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      minWidth={160}
      gap="$2"
      padding="$3"
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.7} color="$colorMuted">
        {label}
      </Text>
      <Text fontSize="$6" fontWeight="800" letterSpacing={-0.2}>
        {value}
      </Text>
      <Text color="$colorMuted" fontSize="$2">
        {detail}
      </Text>
    </YStack>
  );
}

function DetailMetric({
  label,
  value,
  detail
}: {
  label: string;
  value: string;
  detail: string;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      flexGrow={1}
      flexBasis={180}
      minWidth={160}
      gap="$2"
      padding="$3"
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.7} color="$colorMuted">
        {label}
      </Text>
      <Text fontSize="$5" fontWeight="800" numberOfLines={1}>
        {value}
      </Text>
      <Text color="$colorMuted" fontSize="$2">
        {detail}
      </Text>
    </YStack>
  );
}

function ConnectorInventoryItem({
  title,
  description,
  icon,
  configured,
  activeLabel,
  savedCount,
  selected,
  onPress
}: {
  title: string;
  description: string;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
  configured: boolean;
  activeLabel?: string;
  savedCount: number;
  selected: boolean;
  onPress: () => void;
}) {
  const semantics = useThemeSemantics();

  return (
    <Button
      unstyled
      onPress={onPress}
      borderRadius="$5"
      borderWidth={1}
      padding="$4"
      justifyContent="flex-start"
      pressStyle={{ scale: 0.995, opacity: 0.96 }}
      hoverStyle={{ opacity: 1 }}
      style={{
        backgroundColor: selected ? semantics.energyCardBackground : semantics.tileBackground,
        borderColor: selected ? semantics.energyCardBorder : semantics.tileBorder,
        opacity: 1
      }}
    >
      <YStack gap="$3">
        <XStack justifyContent="space-between" alignItems="flex-start" gap="$3">
          <XStack gap="$3" alignItems="center" flex={1}>
            <YStack
              width={42}
              height={42}
              borderRadius={16}
              alignItems="center"
              justifyContent="center"
              borderWidth={1}
              style={{
                backgroundColor: semantics.tileBackground,
                borderColor: selected ? semantics.energyCardBorder : semantics.tileBorder
              }}
            >
              <MaterialCommunityIcons name={icon} size={20} color={semantics.actionText} />
            </YStack>
            <YStack gap="$1" flex={1}>
              <Text fontSize="$5" fontWeight="800">{title}</Text>
              <Text color="$colorMuted" numberOfLines={2}>
                {configured
                  ? activeLabel
                    ? `Active key ${activeLabel}`
                    : 'Saved connector ready for activation'
                  : 'Available connector in Pulse'}
              </Text>
            </YStack>
          </XStack>
          <MaterialCommunityIcons
            name="chevron-right"
            size={20}
            color={selected ? semantics.actionText : semantics.subtleStrongText}
          />
        </XStack>

        <Text color="$colorMuted" lineHeight={22} numberOfLines={3}>
          {description}
        </Text>

        <XStack justifyContent="space-between" alignItems="center" gap="$3" flexWrap="wrap">
          <XStack gap="$2" flexWrap="wrap">
            <SubtleBadge label={configured ? 'Configured' : 'Available'} />
            <SubtleBadge label={`${savedCount} saved`} />
          </XStack>
          <Text fontSize="$2" color="$colorMuted">
            {configured ? 'Open detail' : 'Add connector'}
          </Text>
        </XStack>
      </YStack>
    </Button>
  );
}

function CredentialInventoryRow({
  integration,
  pending,
  selected,
  onSelect,
  onActivate
}: {
  integration: Integration;
  pending: boolean;
  selected: boolean;
  onSelect: () => void;
  onActivate: () => void;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      gap="$3"
      padding="$4"
      borderRadius="$5"
      borderWidth={1}
      style={{
        backgroundColor: selected ? semantics.energyCardBackground : semantics.tileBackground,
        borderColor: selected ? semantics.energyCardBorder : semantics.tileBorder
      }}
    >
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
        <YStack gap="$2" flex={1} minWidth={220}>
          <XStack alignItems="center" gap="$2" flexWrap="wrap">
            <Text fontSize="$5" fontWeight="800">
              {integration.accessKeyMask}
            </Text>
            <StatusPill
              label={integration.isActive ? 'Active' : 'Inactive'}
              icon={integration.isActive ? 'check-decagram' : 'pause-circle-outline'}
              backgroundColor={integration.isActive ? semantics.energyCardBackground : semantics.tileBackground}
              borderColor={integration.isActive ? semantics.energyCardBorder : semantics.tileBorder}
              textColor={integration.isActive ? semantics.statusSuccess : semantics.subtleStrongText}
            />
          </XStack>
          <Text color="$colorMuted">
            Updated {formatTimestamp(integration.updatedAtUnixMs)}
          </Text>
        </YStack>

        <XStack gap="$2" flexWrap="wrap" justifyContent="flex-end">
          <Button
            size="$3"
            themeInverse={!selected}
            chromeless={selected}
            onPress={onSelect}
          >
            {selected ? 'Selected' : 'View details'}
          </Button>
          <Button
            size="$3"
            themeInverse
            chromeless={integration.isActive}
            disabled={integration.isActive || pending}
            onPress={onActivate}
          >
            {integration.isActive ? 'In use' : 'Activate'}
          </Button>
        </XStack>
      </XStack>
    </YStack>
  );
}

function FlashNotice({ message }: { message: string }) {
  const semantics = useThemeSemantics();
  const isError =
    message.toLowerCase().includes('failed') ||
    message.toLowerCase().includes('invalid') ||
    message.toLowerCase().includes('unable');

  return (
    <YStack
      padding="$3"
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: isError ? semantics.actionBackground : semantics.energyCardBackground,
        borderColor: isError ? semantics.actionBorder : semantics.energyCardBorder
      }}
    >
      <Text color="$color">{message}</Text>
    </YStack>
  );
}

function SubtleBadge({ label }: { label: string }) {
  const semantics = useThemeSemantics();

  return (
    <XStack
      alignItems="center"
      gap="$2"
      paddingHorizontal="$3"
      paddingVertical="$2"
      borderRadius={999}
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
        {label}
      </Text>
    </XStack>
  );
}

function StatusPill({
  label,
  icon,
  backgroundColor,
  borderColor,
  textColor
}: {
  label: string;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
  backgroundColor: string;
  borderColor: string;
  textColor: string;
}) {
  return (
    <XStack
      alignItems="center"
      gap="$2"
      paddingHorizontal="$3"
      paddingVertical="$2"
      borderRadius={999}
      borderWidth={1}
      style={{ backgroundColor, borderColor }}
    >
      <MaterialCommunityIcons name={icon} size={14} color={textColor} />
      <Text fontSize="$2" fontWeight="700" style={{ color: textColor }}>
        {label}
      </Text>
    </XStack>
  );
}

function formatTimestamp(unixMs: string): string {
  const numeric = Number.parseInt(unixMs, 10);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return 'recently';
  }
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  }).format(new Date(numeric));
}

function describeIntegrationError(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') {
      return body.message;
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unable to update the integration right now.';
}

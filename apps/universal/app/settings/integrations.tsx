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

const defaultProvider = 'ecoflow';

const PECRON_REGION_OPTIONS = [
  { id: 'us', label: 'US' },
  { id: 'eu', label: 'EU' },
  { id: 'cn', label: 'CN' }
] as const;

type PecronRegion = (typeof PECRON_REGION_OPTIONS)[number]['id'];

const CONNECTOR_CATALOG = [
  {
    id: 'ecoflow',
    title: 'EcoFlow',
    description:
      'Connect your EcoFlow account keys, keep backup credentials inactive, and let Pulse validate provider access plus MQTT before switching the live connector.',
    icon: 'transmission-tower-export' as const,
    catalogTitle: 'EcoFlow is available in Pulse',
    catalogDescription:
      'Native provider integration with discovery, saved credential rotation, and MQTT validation.',
    validationLabel: 'Provider + MQTT',
    activationDescription:
      'Activation runs provider discovery and MQTT checks for the EcoFlow devices already enabled on this account.',
    configureTitle: 'Configure EcoFlow',
    accessKeyLabel: 'Access Key',
    accessSecretLabel: 'Access Secret',
    accessKeyPlaceholder: 'Paste Access Key',
    accessSecretPlaceholder: 'Paste Access Secret',
    replacementAccessKeyPlaceholder: 'Paste replacement Access Key',
    replacementAccessSecretPlaceholder: 'Paste replacement Access Secret',
    createDescription:
      'Paste your EcoFlow Access Key and Access Secret to create the first configured connection.',
    addFallbackDescription: 'Store another EcoFlow credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active key material. Pulse will validate the replacement before switching the live connection.',
    reviewDescription: 'Activate this saved key as-is or rotate its secret before making it live.',
    emptyStateDescription:
      'Add your first EcoFlow key pair and Pulse will validate it before activation.'
  },
  {
    id: 'pulsemqtt',
    title: 'Pulse MQTT Emulator',
    description:
      'Use the local EcoFlow-compatible emulator for signed discovery, MQTT validation, and emulator-backed DPU-X testing without a real upstream account.',
    icon: 'server-network' as const,
    catalogTitle: 'Pulse MQTT Emulator is available in Pulse',
    catalogDescription:
      'Local provider integration with signed REST discovery, MQTT certification, and emulator-backed quota streaming.',
    validationLabel: 'Signed REST + MQTT',
    activationDescription:
      'Activation runs emulator discovery and MQTT checks for the emulator-backed devices already enabled on this account.',
    configureTitle: 'Configure Pulse MQTT Emulator',
    accessKeyLabel: 'Access Key',
    accessSecretLabel: 'Access Secret',
    accessKeyPlaceholder: 'Paste Access Key',
    accessSecretPlaceholder: 'Paste Access Secret',
    replacementAccessKeyPlaceholder: 'Paste replacement Access Key',
    replacementAccessSecretPlaceholder: 'Paste replacement Access Secret',
    createDescription:
      'Paste the emulator Access Key and Access Secret to create the first configured local connector.',
    addFallbackDescription: 'Store another emulator credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active emulator key material. Pulse will validate the replacement before switching the live connection.',
    reviewDescription: 'Activate this saved emulator credential as-is or rotate its secret before making it live.',
    emptyStateDescription:
      'Add your first Pulse MQTT emulator key pair and Pulse will validate it before activation.'
  },
  {
    id: 'pecron',
    title: 'Pecron',
    description:
      'Connect a Pecron cloud account, select the cloud region, discover E1000LFP devices, and stream read-only telemetry through the shared Pulse pipeline.',
    icon: 'battery-sync-outline' as const,
    catalogTitle: 'Pecron E1000LFP is available in Pulse',
    catalogDescription:
      'Unofficial Pecron cloud integration with region-aware discovery, REST snapshots, and MQTT live telemetry.',
    validationLabel: 'Cloud REST + MQTT',
    activationDescription:
      'Activation signs in to the selected Pecron region, discovers supported devices, and validates the MQTT live feed for enabled E1000LFP units.',
    configureTitle: 'Configure Pecron',
    accessKeyLabel: 'Email',
    accessSecretLabel: 'Password',
    accessKeyPlaceholder: 'Pecron account email',
    accessSecretPlaceholder: 'Pecron account password',
    replacementAccessKeyPlaceholder: 'Replacement Pecron account email',
    replacementAccessSecretPlaceholder: 'Replacement Pecron password',
    createDescription:
      'Enter the Pecron account email and password used by the Pecron app, then choose the matching cloud region.',
    addFallbackDescription: 'Store another Pecron credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active Pecron account credentials. Pulse will validate discovery and MQTT before switching the live connection.',
    reviewDescription: 'Activate this saved Pecron credential as-is or rotate its password before making it live.',
    emptyStateDescription:
      'Add your first Pecron account and Pulse will validate it before activation.'
  }
] as const;

type ConnectorCopy = (typeof CONNECTOR_CATALOG)[number];

function getConnectorCopy(provider: string): ConnectorCopy {
  return CONNECTOR_CATALOG.find((item) => item.id === provider) ?? CONNECTOR_CATALOG[0];
}

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
  const [selectedProvider, setSelectedProvider] = useState(defaultProvider);
  const [selectedRegion, setSelectedRegion] = useState<PecronRegion>('us');

  const integrationsQuery = useIntegrations({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const createIntegration = useCreateIntegration({ token, authKey });
  const updateIntegration = useUpdateIntegration({ token, authKey });
  const setIntegrationActive = useSetIntegrationActive({ token, authKey });

  const allIntegrations = useMemo(
    () => integrationsQuery.data?.integrations ?? [],
    [integrationsQuery.data?.integrations]
  );
  const integrations = useMemo(
    () => allIntegrations.filter((item) => item.provider === selectedProvider),
    [allIntegrations, selectedProvider]
  );
  const activeIntegration = useMemo(
    () => integrations.find((item) => item.isActive) ?? null,
    [integrations]
  );
  const selectedConnector = getConnectorCopy(selectedProvider);
  const connectorCatalog = useMemo(
    () =>
      CONNECTOR_CATALOG.map((connector) => {
        const connectorIntegrations = allIntegrations.filter((item) => item.provider === connector.id);
        return {
          ...connector,
          savedCount: connectorIntegrations.length,
          activeLabel: connectorIntegrations.find((item) => item.isActive)?.accessKeyMask
        };
      }),
    [allIntegrations]
  );
  const configuredConnectorCount = useMemo(
    () => connectorCatalog.filter((connector) => connector.savedCount > 0).length,
    [connectorCatalog]
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
    setSelectedRegion(regionFromIntegration(selected) ?? 'us');
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
      : selectedConnector.configureTitle;
  const detailDescription = selectedIntegration
    ? selectedIntegration.isActive
      ? selectedConnector.rotateDescription
      : selectedConnector.reviewDescription
    : hasConfiguredConnector
      ? selectedConnector.addFallbackDescription
      : selectedConnector.createDescription;

  const handleSave = async () => {
    setFlashMessage('');
    try {
      if (selectedIntegration) {
        const updated = await updateIntegration.mutateAsync({
          credentialId: selectedIntegration.id,
          values: {
            accessKey,
            accessSecret,
            config: buildIntegrationConfig(selectedProvider, selectedRegion),
            isActive: activateOnSave
          }
        });
        setSelectedCredentialId(updated.id);
        setFlashMessage(
          activateOnSave
            ? `${selectedConnector.title} credentials validated and applied to your existing enabled devices.`
            : `${selectedConnector.title} credentials updated without activating this saved connection.`
        );
      } else {
        const created = await createIntegration.mutateAsync({
          provider: selectedProvider,
          accessKey,
          accessSecret,
          config: buildIntegrationConfig(selectedProvider, selectedRegion),
          isActive: activateOnSave
        });
        setSelectedCredentialId(created.id);
        setFlashMessage(
          activateOnSave
            ? `${selectedConnector.title} connection verified and activated.`
            : `${selectedConnector.title} connection saved as inactive.`
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
      setFlashMessage(`Connection verified and switched to the active ${selectedConnector.title} integration.`);
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
                          name={selectedConnector.icon}
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
                      value={`${configuredConnectorCount} ${configuredConnectorCount === 1 ? 'connector' : 'connectors'}`}
                      detail={`${selectedConnector.title} selected`}
                    />
                    <SummaryMetric
                      label="Active key"
                      value={activeConnectionLabel}
                      detail={connectorStatusLabel}
                    />
                    <SummaryMetric
                      label="Saved credentials"
                      value={String(integrations.length)}
                      detail={
                        integrations.length === 0
                          ? 'Add your first key'
                          : `${selectedConnector.title} fallbacks stay inactive`
                      }
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

                  <YStack gap="$3">
                    {connectorCatalog.map((connector) => (
                      <ConnectorInventoryItem
                        key={connector.id}
                        title={connector.title}
                        description={connector.description}
                        icon={connector.icon}
                        configured={connector.savedCount > 0}
                        activeLabel={connector.activeLabel}
                        savedCount={connector.savedCount}
                        selected={selectedProvider === connector.id}
                        onPress={() => {
                          setSelectedProvider(connector.id);
                          setSelectedCredentialId('new');
                          setFlashMessage('');
                        }}
                      />
                    ))}
                  </YStack>

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
                      {selectedConnector.catalogTitle}
                    </Text>
                    <Text color="$colorMuted" lineHeight={22}>
                      {selectedConnector.catalogDescription}
                    </Text>
                    <XStack gap="$2" flexWrap="wrap">
                      <SubtleBadge label={hasConfiguredConnector ? 'Configured' : 'Available'} />
                      <SubtleBadge label={selectedConnector.validationLabel} />
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
                            name={selectedConnector.icon}
                            size={22}
                            color={semantics.actionText}
                          />
                        </YStack>
                        <YStack gap="$1">
                          <Text fontSize="$6" fontWeight="800">{selectedConnector.title}</Text>
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
                          value={selectedConnector.validationLabel}
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
                              {selectedConnector.emptyStateDescription}
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
                          <Text fontWeight="700">{selectedConnector.accessKeyLabel}</Text>
                          <AppTextInput
                            value={accessKey}
                            onChangeText={setAccessKey}
                            autoCapitalize="none"
                            autoCorrect={false}
                            placeholder={
                              selectedIntegration
                                ? selectedConnector.replacementAccessKeyPlaceholder
                                : selectedConnector.accessKeyPlaceholder
                            }
                          />
                        </YStack>

                        <YStack gap="$2">
                          <Text fontWeight="700">{selectedConnector.accessSecretLabel}</Text>
                          <AppTextInput
                            value={accessSecret}
                            onChangeText={setAccessSecret}
                            autoCapitalize="none"
                            autoCorrect={false}
                            secureTextEntry
                            placeholder={
                              selectedIntegration
                                ? selectedConnector.replacementAccessSecretPlaceholder
                                : selectedConnector.accessSecretPlaceholder
                            }
                          />
                        </YStack>

                        {selectedProvider === 'pecron' ? (
                          <YStack gap="$2">
                            <Text fontWeight="700">Cloud region</Text>
                            <XStack gap="$2" flexWrap="wrap">
                              {PECRON_REGION_OPTIONS.map((region) => (
                                <Button
                                  key={region.id}
                                  size="$4"
                                  themeInverse={selectedRegion === region.id}
                                  chromeless={selectedRegion !== region.id}
                                  onPress={() => setSelectedRegion(region.id)}
                                >
                                  {region.label}
                                </Button>
                              ))}
                            </XStack>
                            <Text color="$colorMuted">
                              Select the same Pecron cloud region used by the mobile app for this account.
                            </Text>
                          </YStack>
                        ) : null}

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
                            {selectedConnector.activationDescription}
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
            {integration.provider === 'pecron'
              ? `${formatPecronRegion(regionFromIntegration(integration))} region. Updated ${formatTimestamp(integration.updatedAtUnixMs)}`
              : `Updated ${formatTimestamp(integration.updatedAtUnixMs)}`}
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

function buildIntegrationConfig(provider: string, region: PecronRegion): Record<string, unknown> {
  if (provider !== 'pecron') {
    return {};
  }
  return { region };
}

function regionFromIntegration(integration: Integration | null): PecronRegion | null {
  if (!integration || integration.provider !== 'pecron') {
    return null;
  }
  return normalizePecronRegion(integration.config?.region);
}

function normalizePecronRegion(value: unknown): PecronRegion {
  const text = typeof value === 'string' ? value.trim().toLowerCase() : '';
  return PECRON_REGION_OPTIONS.some((region) => region.id === text) ? (text as PecronRegion) : 'us';
}

function formatPecronRegion(region: PecronRegion | null): string {
  const normalized = region ?? 'us';
  return PECRON_REGION_OPTIONS.find((option) => option.id === normalized)?.label ?? 'US';
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

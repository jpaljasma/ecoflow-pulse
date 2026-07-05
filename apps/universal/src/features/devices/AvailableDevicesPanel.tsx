import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useEffect, useMemo, useState } from 'react';
import { router } from 'expo-router';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import {
  EDGE_DEVICE_SOURCE_STATUS_LINKED,
  EDGE_DEVICE_SOURCE_STATUS_PENDING,
  type AvailableDeviceSummary,
  type DeviceSummary,
  type DeviceMQTTTestResult,
  type EdgeCollector,
  type EdgeDeviceSource
} from '@/features/devices/api';
import {
  useAvailableDevices,
  useApproveEdgeDeviceSource,
  useCreateEdgeCollector,
  useDevices,
  useEdgeCollectors,
  useEdgeDeviceSources,
  useImportAvailableDevice,
  useRevokeEdgeCollectorSetupToken,
  useTestAvailableDeviceMQTT
} from '@/features/devices/hooks';
import { formatAvailableDeviceActionError } from '@/features/devices/actionMessages';
import {
  useConnectEcoFlowBLEAuth,
  useEcoFlowBLEAuthStatus,
  useSetEcoFlowBLEAuthUserID
} from '@/features/integrations/hooks';
import {
  describeAvailableDeviceSupport,
  formatProviderLabel,
  type AvailableDeviceSupport
} from '@/features/integrations/providerCatalog';
import { maskSerialNumber } from '@/features/telemetry/format';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Card } from '@/shared/ui/Card';

type SemanticColors = ReturnType<typeof useThemeSemantics>;
type ReadinessTone = 'neutral' | 'success' | 'warning' | 'danger';

const COLLECTOR_STALE_WARNING_MS = 5 * 60_000;

type AvailableDevicesPanelProps = {
  token?: string;
  authKey?: string;
  enabled: boolean;
  onDeviceEnabled?: (deviceId: string) => void;
};

export function AvailableDevicesPanel({
  token,
  authKey = 'anonymous',
  enabled,
  onDeviceEnabled
}: AvailableDevicesPanelProps) {
  const [activated, setActivated] = useState(false);
  const semantics = useThemeSemantics();
  const availableQuery = useAvailableDevices({
    token,
    authKey,
    enabled: enabled && activated
  });

  return (
    <Card gap="$3" marginTop="$3">
      <XStack
        alignItems="center"
        justifyContent="space-between"
        gap="$3"
        flexWrap="wrap"
      >
        <YStack gap="$1" flex={1} minWidth={240}>
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons
              name="plus-box-multiple-outline"
              size={20}
              color={semantics.actionText}
            />
            <Text fontSize="$6" fontWeight="700">
              Available devices
            </Text>
          </XStack>
          <Text color="$colorMuted">
            Scan for devices linked to your provider credentials, then enable
            and activate them with a live MQTT check.
          </Text>
        </YStack>

        {!activated ? (
          <Button
            size="$3"
            onPress={() => setActivated(true)}
            icon={
              <MaterialCommunityIcons name="radar" size={16} color="white" />
            }
          >
            Find available devices
          </Button>
        ) : (
          <Button
            size="$3"
            chromeless
            onPress={() => {
              void availableQuery.refetch();
            }}
            disabled={availableQuery.isFetching}
            icon={
              availableQuery.isFetching ? (
                <Spinner size="small" color={semantics.actionText} />
              ) : (
                <MaterialCommunityIcons
                  name="refresh"
                  size={16}
                  color={semantics.actionText}
                />
              )
            }
          >
            Refresh
          </Button>
        )}
      </XStack>

      {!activated ? (
        <Text color="$colorMuted">
          This keeps provider discovery explicit and only runs when you ask for
          it.
        </Text>
      ) : null}

      {activated && availableQuery.isLoading && !availableQuery.data ? (
        <XStack alignItems="center" gap="$2" minHeight={72}>
          <Spinner size="small" />
          <Text color="$colorMuted">
            Checking your provider account for new devices…
          </Text>
        </XStack>
      ) : null}

      {activated && availableQuery.isError ? (
        <YStack gap="$1">
          <Text fontWeight="700">Couldn’t load available devices</Text>
          <Text color="$colorMuted">{String(availableQuery.error)}</Text>
        </YStack>
      ) : null}

      {activated ? (
        <EcoFlowBLEEdgeSection
          token={token}
          authKey={authKey}
          enabled={enabled}
          onDeviceEnabled={onDeviceEnabled}
        />
      ) : null}

      {activated &&
      availableQuery.data &&
      availableQuery.data.warningMessage ? (
        <YStack gap="$3">
          <YStack
            gap="$2"
            padding="$3"
            borderRadius="$4"
            borderWidth={1}
            style={{
              backgroundColor: semantics.statusWarningBackground,
              borderColor: semantics.statusWarningBorder
            }}
          >
            <XStack alignItems="center" gap="$2">
              <MaterialCommunityIcons
                name="alert-outline"
                size={18}
                color={semantics.statusWarning}
              />
              <Text fontWeight="700">Connector attention needed</Text>
            </XStack>
            <Text color="$colorMuted">
              {availableQuery.data.warningMessage}
            </Text>
          </YStack>
          <XStack justifyContent="flex-end">
            <Button
              size="$3"
              onPress={() => router.push('/settings/integrations')}
              icon={
                <MaterialCommunityIcons
                  name="cog-outline"
                  size={16}
                  color="white"
                />
              }
            >
              Open Integrations
            </Button>
          </XStack>
        </YStack>
      ) : null}

      {activated &&
      availableQuery.data &&
      !availableQuery.isError &&
      !availableQuery.data.warningMessage &&
      !availableQuery.data.hasActiveCredentials ? (
        <Text color="$colorMuted">
          No active provider credentials are available yet, so there’s nothing
          to scan.
        </Text>
      ) : null}

      {activated &&
      availableQuery.data &&
      !availableQuery.isError &&
      !availableQuery.data.warningMessage &&
      availableQuery.data.hasActiveCredentials &&
      availableQuery.data.devices.length === 0 ? (
        <Text color="$colorMuted">
          No unconfigured devices were found on the latest scan.
        </Text>
      ) : null}

      {activated &&
      availableQuery.data &&
      availableQuery.data.devices.length > 0 ? (
        <YStack gap="$3">
          {availableQuery.data.devices.map((device) => (
            <AvailableDeviceCard
              key={`${device.provider}:${device.providerDeviceId}`}
              device={device}
              token={token}
              authKey={authKey}
              onDeviceEnabled={onDeviceEnabled}
            />
          ))}
        </YStack>
      ) : null}
    </Card>
  );
}

function EcoFlowBLEEdgeSection({
  token,
  authKey,
  enabled,
  onDeviceEnabled
}: {
  token?: string;
  authKey: string;
  enabled: boolean;
  onDeviceEnabled?: (deviceId: string) => void;
}) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [manualUserId, setManualUserId] = useState('');
  const [showManualAuth, setShowManualAuth] = useState(false);
  const [collectorName, setCollectorName] = useState('Raspberry Pi');
  const [setupToken, setSetupToken] = useState('');
  const [setupTokenCollectorId, setSetupTokenCollectorId] = useState('');
  const semantics = useThemeSemantics();
  const authStatusQuery = useEcoFlowBLEAuthStatus({ token, authKey, enabled });
  const collectorsQuery = useEdgeCollectors({ token, authKey, enabled });
  const devicesQuery = useDevices({ token, authKey, enabled });
  const sourcesQuery = useEdgeDeviceSources({
    token,
    authKey,
    enabled,
    includeAllStatuses: true
  });
  const connectAuth = useConnectEcoFlowBLEAuth({ token, authKey });
  const setManualAuth = useSetEcoFlowBLEAuthUserID({ token, authKey });
  const createCollector = useCreateEdgeCollector({ token, authKey });
  const revokeSetupToken = useRevokeEdgeCollectorSetupToken({ token, authKey });
  const approveSource = useApproveEdgeDeviceSource({ token, authKey });
  const connected = authStatusQuery.data?.connected === true;
  const collectors = collectorsQuery.data ?? [];
  const devices = devicesQuery.data?.devices ?? [];
  const sources = useMemo(() => sourcesQuery.data ?? [], [sourcesQuery.data]);
  const pendingSources = useMemo(
    () =>
      sources.filter(
        (source) => source.status === EDGE_DEVICE_SOURCE_STATUS_PENDING
      ),
    [sources]
  );
  const linkedSources = useMemo(
    () => sources.filter(isLinkedSource),
    [sources]
  );
  const readinessSources = useMemo(
    () => [...pendingSources, ...linkedSources],
    [linkedSources, pendingSources]
  );
  const collectorReadiness = classifyCollectorReadiness(
    collectors,
    readinessSources
  );
  const edgeQueries = [
    authStatusQuery,
    collectorsQuery,
    devicesQuery,
    sourcesQuery
  ] as const;
  const edgeRefreshing = edgeQueries.some((query) => query.isFetching);
  const authBusy = connectAuth.isPending || setManualAuth.isPending;

  useEffect(() => {
    setEmail('');
    setPassword('');
    setManualUserId('');
    setShowManualAuth(false);
    setSetupToken('');
    setSetupTokenCollectorId('');
  }, [authKey, token]);

  function handleConnectAuth() {
    connectAuth.mutate(
      { email, password },
      { onSettled: () => setPassword('') }
    );
  }

  function handleManualAuth() {
    setManualAuth.mutate(
      {
        userId: manualUserId
      },
      { onSettled: () => setManualUserId('') }
    );
  }

  function handleCreateCollector() {
    createCollector.mutate(
      {
        displayName: collectorName.trim() || undefined
      },
      {
        onSuccess: (created) => {
          setSetupToken(created.setupToken);
          setSetupTokenCollectorId(created.collector.id);
        }
      }
    );
  }

  function handleRevokeSetupToken() {
    if (!setupTokenCollectorId) {
      return;
    }
    revokeSetupToken.mutate(setupTokenCollectorId, {
      onSuccess: () => {
        setSetupToken('');
        setSetupTokenCollectorId('');
      }
    });
  }

  function refreshEdgeQueries() {
    for (const query of edgeQueries) {
      void query.refetch();
    }
  }

  function handleApproveSource(source: EdgeDeviceSource, deviceId?: string) {
    const selectedDeviceID = deviceId?.trim();
    approveSource.mutate(
      {
        sourceId: source.id,
        deviceId: selectedDeviceID || undefined,
        productName: source.displayName || source.model || 'EcoFlow BLE device',
        model: source.model
      },
      { onSuccess: (approved) => onDeviceEnabled?.(approved.deviceId) }
    );
  }

  return (
    <YStack
      gap="$3"
      padding="$3"
      borderRadius="$4"
      borderWidth={1}
      style={{
        borderColor: semantics.statusSuccessBorder,
        backgroundColor: semantics.statusSuccessBackground
      }}
    >
      <XStack
        alignItems="center"
        justifyContent="space-between"
        gap="$3"
        flexWrap="wrap"
      >
        <YStack gap="$1" flex={1} minWidth={220}>
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons
              name="bluetooth"
              size={18}
              color={semantics.statusSuccess}
            />
            <Text fontSize="$5" fontWeight="700">
              EcoFlow BLE edge
            </Text>
          </XStack>
          <Text color="$colorMuted">
            {connected
              ? `BLE auth connected${authStatusQuery.data?.accountMask ? ` for ${authStatusQuery.data.accountMask}` : ''}.`
              : 'Connect EcoFlow app auth before authenticated BLE devices are enrolled.'}
          </Text>
        </YStack>
        <Button
          size="$3"
          chromeless
          onPress={() => {
            refreshEdgeQueries();
          }}
          disabled={edgeRefreshing}
          icon={
            edgeRefreshing ? (
              <Spinner size="small" color={semantics.statusSuccess} />
            ) : (
              <MaterialCommunityIcons
                name="refresh"
                size={16}
                color={semantics.statusSuccess}
              />
            )
          }
        >
          Refresh BLE
        </Button>
      </XStack>

      {!connected ? (
        <YStack gap="$3">
          <XStack gap="$2" flexWrap="wrap">
            <YStack gap="$1" flex={1} minWidth={210}>
              <Text fontSize="$2" fontWeight="700">
                EcoFlow email
              </Text>
              <AppTextInput
                compact
                value={email}
                onChangeText={setEmail}
                autoCapitalize="none"
                autoComplete="email"
                autoCorrect={false}
                keyboardType="email-address"
                spellCheck={false}
                textContentType="emailAddress"
                accessibilityLabel="EcoFlow email"
              />
            </YStack>
            <YStack gap="$1" flex={1} minWidth={210}>
              <Text fontSize="$2" fontWeight="700">
                EcoFlow password
              </Text>
              <AppTextInput
                compact
                value={password}
                onChangeText={setPassword}
                autoComplete="current-password"
                secureTextEntry
                textContentType="password"
                accessibilityLabel="EcoFlow password"
              />
            </YStack>
            <Button
              size="$3"
              disabled={
                authBusy || email.trim().length === 0 || password.length === 0
              }
              onPress={handleConnectAuth}
              icon={
                connectAuth.isPending ? (
                  <Spinner size="small" color="white" />
                ) : (
                  <MaterialCommunityIcons
                    name="login"
                    size={16}
                    color="white"
                  />
                )
              }
            >
              Connect
            </Button>
          </XStack>
          <YStack gap="$2">
            <Button
              size="$3"
              chromeless
              alignSelf="flex-start"
              onPress={() => setShowManualAuth((value) => !value)}
              icon={
                <MaterialCommunityIcons
                  name={showManualAuth ? 'chevron-up' : 'chevron-down'}
                  size={16}
                  color={semantics.statusSuccess}
                />
              }
            >
              Advanced manual ID
            </Button>
            {showManualAuth ? (
              <XStack gap="$2" flexWrap="wrap">
                <YStack gap="$1" flex={1} minWidth={240}>
                  <Text fontSize="$2" fontWeight="700">
                    BLE user ID
                  </Text>
                  <AppTextInput
                    compact
                    value={manualUserId}
                    onChangeText={setManualUserId}
                    autoCapitalize="none"
                    autoComplete="username"
                    autoCorrect={false}
                    spellCheck={false}
                    textContentType="username"
                    accessibilityLabel="EcoFlow BLE user ID"
                  />
                </YStack>
                <Button
                  size="$3"
                  disabled={authBusy || manualUserId.trim().length === 0}
                  onPress={handleManualAuth}
                  icon={
                    setManualAuth.isPending ? (
                      <Spinner size="small" color="white" />
                    ) : (
                      <MaterialCommunityIcons
                        name="key-outline"
                        size={16}
                        color="white"
                      />
                    )
                  }
                >
                  Save manual ID
                </Button>
              </XStack>
            ) : null}
          </YStack>
          {connectAuth.isError || setManualAuth.isError ? (
            <Text style={{ color: semantics.statusDanger }}>
              {String(connectAuth.error ?? setManualAuth.error)}
            </Text>
          ) : null}
        </YStack>
      ) : null}

      <YStack gap="$2">
        <XStack gap="$2" flexWrap="wrap" alignItems="center">
          <YStack gap="$1" flex={1} minWidth={220}>
            <Text fontSize="$2" fontWeight="700">
              Collector name
            </Text>
            <AppTextInput
              compact
              value={collectorName}
              onChangeText={setCollectorName}
              accessibilityLabel="Collector name"
            />
          </YStack>
          <Button
            size="$3"
            disabled={!connected || createCollector.isPending}
            onPress={handleCreateCollector}
            icon={
              createCollector.isPending ? (
                <Spinner size="small" color="white" />
              ) : (
                <MaterialCommunityIcons
                  name="raspberry-pi"
                  size={16}
                  color="white"
                />
              )
            }
          >
            Create setup token
          </Button>
        </XStack>
        {setupToken ? (
          <XStack gap="$2" flexWrap="wrap" alignItems="flex-end">
            <YStack gap="$1" flex={1} minWidth={240}>
              <Text fontSize="$2" fontWeight="700">
                Setup token
              </Text>
              <AppTextInput
                compact
                value={setupToken}
                editable={false}
                selectTextOnFocus
                accessibilityLabel="Setup token"
              />
            </YStack>
            <Button
              size="$3"
              chromeless
              disabled={revokeSetupToken.isPending || !setupTokenCollectorId}
              onPress={handleRevokeSetupToken}
              icon={
                revokeSetupToken.isPending ? (
                  <Spinner size="small" />
                ) : (
                  <MaterialCommunityIcons
                    name="close-octagon-outline"
                    size={16}
                    color={semantics.statusDanger}
                  />
                )
              }
            >
              Revoke
            </Button>
          </XStack>
        ) : null}
        {createCollector.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {String(createCollector.error)}
          </Text>
        ) : null}
        {revokeSetupToken.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {String(revokeSetupToken.error)}
          </Text>
        ) : null}
        <YStack gap="$1">
          <XStack alignItems="center" gap="$2" flexWrap="wrap">
            <MaterialCommunityIcons
              name={collectorReadiness.icon}
              size={16}
              color={readinessToneColor(collectorReadiness.tone, semantics)}
            />
            <Text
              fontWeight="700"
              style={{
                color: readinessToneColor(collectorReadiness.tone, semantics)
              }}
            >
              {collectorReadiness.label}
            </Text>
          </XStack>
          <Text color="$colorMuted">{collectorReadiness.detail}</Text>
        </YStack>
      </YStack>

      <YStack gap="$2">
        <XStack
          alignItems="center"
          justifyContent="space-between"
          gap="$2"
          flexWrap="wrap"
        >
          <Text fontWeight="700">BLE discoveries</Text>
          <Text color="$colorMuted">
            {formatEdgeSourceSummary(readinessSources)}
          </Text>
        </XStack>
        {sourcesQuery.isLoading && !sourcesQuery.data ? (
          <XStack alignItems="center" gap="$2">
            <Spinner size="small" />
            <Text color="$colorMuted">Loading BLE discoveries…</Text>
          </XStack>
        ) : null}
        {sourcesQuery.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {String(sourcesQuery.error)}
          </Text>
        ) : null}
        {!sourcesQuery.isLoading && pendingSources.length === 0 ? (
          <Text color="$colorMuted">
            No pending BLE sources have checked in yet.
          </Text>
        ) : null}
        {pendingSources.map((source) => (
          <EdgeDeviceSourceRow
            key={`${authKey}:${source.id}`}
            source={source}
            devices={devices}
            busy={approveSource.isPending}
            onApprove={(deviceId) => {
              handleApproveSource(source, deviceId);
            }}
          />
        ))}
        {approveSource.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {String(approveSource.error)}
          </Text>
        ) : null}
      </YStack>
    </YStack>
  );
}

function EdgeDeviceSourceRow({
  source,
  devices,
  busy,
  onApprove
}: {
  source: EdgeDeviceSource;
  devices: DeviceSummary[];
  busy: boolean;
  onApprove: (deviceId?: string) => void;
}) {
  const [selectedDeviceId, setSelectedDeviceId] = useState('');
  const semantics = useThemeSemantics();
  const createNewSelected = selectedDeviceId === '';

  useEffect(() => {
    setSelectedDeviceId('');
  }, [source.id]);

  return (
    <XStack
      alignItems="center"
      justifyContent="space-between"
      gap="$3"
      flexWrap="wrap"
      padding="$3"
      borderRadius="$3"
      borderWidth={1}
      style={{
        borderColor: semantics.statusSuccessBorder,
        backgroundColor: semantics.tileBackground
      }}
    >
      <YStack gap="$2" flex={1} minWidth={220}>
        <Text fontWeight="700">
          {source.displayName || source.model || 'EcoFlow BLE device'}
        </Text>
        <Text color="$colorMuted">
          {source.model || 'EcoFlow BLE'} · RSSI {source.rssiDbm} dBm ·{' '}
          {formatEdgeSourceSeen(source.lastSeenAtUnixMs)}
        </Text>
        <XStack gap="$2" flexWrap="wrap" alignItems="center">
          <Text fontSize="$2" color="$colorMuted">
            Link target
          </Text>
          <Button
            size="$2"
            chromeless={!createNewSelected}
            accessibilityState={{ selected: createNewSelected }}
            onPress={() => setSelectedDeviceId('')}
          >
            Create New Device
          </Button>
          {devices.map((device) => {
            const selected = selectedDeviceId === device.id;
            return (
              <Button
                key={device.id}
                size="$2"
                chromeless={!selected}
                accessibilityState={{ selected }}
                onPress={() => setSelectedDeviceId(device.id)}
              >
                {device.name || device.model || 'Device'}
              </Button>
            );
          })}
        </XStack>
      </YStack>
      <Button
        size="$3"
        disabled={busy}
        onPress={() => onApprove(selectedDeviceId)}
        icon={
          busy ? (
            <Spinner size="small" color="white" />
          ) : (
            <MaterialCommunityIcons
              name="link-variant-plus"
              size={16}
              color="white"
            />
          )
        }
      >
        Approve
      </Button>
    </XStack>
  );
}

function classifyCollectorReadiness(
  collectors: EdgeCollector[],
  sources: EdgeDeviceSource[],
  now = Date.now()
): {
  label: string;
  detail: string;
  tone: ReadinessTone;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
} {
  const sourceSummary = formatEdgeSourceSummary(sources);
  if (collectors.length === 0) {
    return {
      label: 'No collector registered',
      detail: `Create a setup token to link a collector. ${sourceSummary}.`,
      tone: 'neutral',
      icon: 'server-network-outline'
    };
  }
  const heartbeatAges = collectors
    .map((collector) => collectorHeartbeatAgeMs(collector, now))
    .filter((age): age is number => age !== null);
  const active = collectors.filter((collector) => collector.isActive).length;
  const freshActive = collectors.filter((collector) => {
    const age = collectorHeartbeatAgeMs(collector, now);
    return (
      collector.isActive && age !== null && age <= COLLECTOR_STALE_WARNING_MS
    );
  }).length;
  const collectorCount = `${collectors.length} registered collector${collectors.length === 1 ? '' : 's'}`;

  if (freshActive > 0) {
    return {
      label: 'Collector active',
      detail: `${collectorCount} · ${freshActive} active · ${sourceSummary}.`,
      tone: 'success',
      icon: 'check-circle-outline'
    };
  }
  if (heartbeatAges.length === 0) {
    return {
      label: 'Waiting for collector heartbeat',
      detail: `${collectorCount} · no heartbeat yet · ${sourceSummary}.`,
      tone: 'warning',
      icon: 'timer-sand'
    };
  }

  return {
    label: 'Collector stale or offline',
    detail: `${collectorCount} · ${active} active · latest heartbeat ${formatRelativeAge(Math.min(...heartbeatAges))} · ${sourceSummary}.`,
    tone: 'danger',
    icon: 'alert-circle-outline'
  };
}

function collectorHeartbeatAgeMs(
  collector: EdgeCollector,
  now: number
): number | null {
  const heartbeatMs = Number(collector.lastHeartbeatAtUnixMs);
  if (!Number.isFinite(heartbeatMs) || heartbeatMs <= 0) {
    return null;
  }
  return Math.max(0, now - heartbeatMs);
}

function readinessToneColor(
  tone: ReadinessTone,
  semantics: SemanticColors
): string {
  switch (tone) {
    case 'success':
      return semantics.statusSuccess;
    case 'warning':
      return semantics.statusWarning;
    case 'danger':
      return semantics.statusDanger;
    default:
      return semantics.subtleStrongText;
  }
}

function formatEdgeSourceSummary(sources: EdgeDeviceSource[]): string {
  const linked = sources.filter(isLinkedSource).length;
  const pending = sources.filter((source) => !isLinkedSource(source)).length;
  if (linked === 0 && pending === 0) {
    return 'No linked or pending sources';
  }
  return [
    formatCount(pending, 'pending source'),
    formatCount(linked, 'linked source')
  ]
    .filter(Boolean)
    .join(' · ');
}

function isLinkedSource(source: EdgeDeviceSource): boolean {
  return (
    source.status === EDGE_DEVICE_SOURCE_STATUS_LINKED ||
    source.linkedDeviceId.trim().length > 0
  );
}

function formatCount(count: number, label: string): string {
  if (count === 0) {
    return '';
  }
  return `${count} ${label}${count === 1 ? '' : 's'}`;
}

function formatEdgeSourceSeen(value: string): string {
  const ms = Number(value);
  if (!Number.isFinite(ms) || ms <= 0) {
    return 'last seen unknown';
  }
  const minutes = Math.max(0, Math.round((Date.now() - ms) / 60_000));
  if (minutes < 1) {
    return 'seen just now';
  }
  if (minutes < 60) {
    return `seen ${minutes}m ago`;
  }
  const hours = Math.round(minutes / 60);
  return `seen ${hours}h ago`;
}

function formatRelativeAge(ageMs: number): string {
  const minutes = Math.max(0, Math.round(ageMs / 60_000));
  if (minutes < 1) {
    return 'just now';
  }
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.round(minutes / 60);
  return `${hours}h ago`;
}

function AvailableDeviceCard({
  device,
  token,
  authKey,
  onDeviceEnabled
}: {
  device: AvailableDeviceSummary;
  token?: string;
  authKey: string;
  onDeviceEnabled?: (deviceId: string) => void;
}) {
  const [probeResult, setProbeResult] = useState<DeviceMQTTTestResult | null>(
    null
  );
  const semantics = useThemeSemantics();
  const testMutation = useTestAvailableDeviceMQTT({ token, authKey });
  const importMutation = useImportAvailableDevice({ token, authKey });
  const busy = testMutation.isPending || importMutation.isPending;
  const support = describeAvailableDeviceSupport(device);
  const enableableByCatalog =
    device.provider === 'anker_solix' ? support?.enableable === true : true;

  async function runProbe() {
    try {
      const result = await testMutation.mutateAsync({
        provider: device.provider,
        credentialId: device.credentialId,
        providerDeviceId: device.providerDeviceId
      });
      setProbeResult(result);
      if (result.success && result.deviceId) {
        onDeviceEnabled?.(result.deviceId);
      }
    } catch {
      // React Query exposes the error state below.
    }
  }

  async function importDeviceInactive() {
    await importMutation.mutateAsync({
      provider: device.provider,
      credentialId: device.credentialId,
      providerDeviceId: device.providerDeviceId,
      isActive: false,
      ingestDesiredState: 'paused'
    });
  }

  return (
    <Card
      gap="$3"
      style={{
        backgroundColor: semantics.actionBackground,
        borderColor: semantics.actionBorder
      }}
    >
      <XStack
        justifyContent="space-between"
        alignItems="flex-start"
        gap="$3"
        flexWrap="wrap"
      >
        <YStack gap="$1" flex={1} minWidth={220}>
          <XStack alignItems="center" gap="$2" flexWrap="wrap">
            <Text fontSize="$5" fontWeight="700">
              {device.name}
            </Text>
            <XStack
              alignItems="center"
              gap="$1"
              paddingHorizontal="$2"
              paddingVertical="$1"
              borderRadius={999}
              style={{ backgroundColor: semantics.actionBackground }}
            >
              <MaterialCommunityIcons
                name="new-box"
                size={14}
                color={semantics.actionText}
              />
              <Text
                fontSize="$2"
                fontWeight="700"
                style={{ color: semantics.actionText }}
              >
                New
              </Text>
            </XStack>
          </XStack>
          <Text color="$colorMuted">{device.model}</Text>
          <Text color="$colorMuted">
            {formatProviderLabel(device.provider)} ·{' '}
            {maskSerialNumber(device.serialNumber)}
          </Text>
          {support ? (
            <YStack gap="$1" marginTop="$1">
              <SupportBadge support={support} />
              <Text color="$colorMuted" fontSize="$2">
                {support.detail}
              </Text>
            </YStack>
          ) : null}
        </YStack>

        <YStack gap="$2" minWidth={180}>
          <Button
            size="$3"
            onPress={() => {
              void runProbe();
            }}
            disabled={
              busy || !enableableByCatalog || probeResult?.success === true
            }
            icon={
              testMutation.isPending ? (
                <Spinner size="small" color="white" />
              ) : (
                <MaterialCommunityIcons
                  name="check-circle-outline"
                  size={16}
                  color="white"
                />
              )
            }
          >
            Enable and Activate
          </Button>
          <Button
            size="$3"
            onPress={() => {
              void importDeviceInactive();
            }}
            disabled={busy || !enableableByCatalog}
            icon={
              importMutation.isPending ? (
                <Spinner size="small" color="white" />
              ) : (
                <MaterialCommunityIcons
                  name="pause-circle-outline"
                  size={16}
                  color="white"
                />
              )
            }
          >
            Import inactive
          </Button>
        </YStack>
      </XStack>

      <YStack gap="$1" minHeight={46}>
        {probeResult?.success ? (
          <Text style={{ color: semantics.statusSuccess }}>
            {formatProbeStatus(probeResult)}
          </Text>
        ) : probeResult ? (
          <Text color="$colorMuted">{formatProbeStatus(probeResult)}</Text>
        ) : (
          <Text color="$colorMuted">
            {enableableByCatalog
              ? 'Runs a live MQTT probe and activates this device only after telemetry is observed.'
              : 'This model is visible for tracking, but V1 does not enable standalone MQTT ingest for it yet.'}
          </Text>
        )}
        {testMutation.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {formatAvailableDeviceActionError(
              'Enable and Activate',
              testMutation.error
            )}
          </Text>
        ) : null}
        {importMutation.isError ? (
          <Text style={{ color: semantics.statusDanger }}>
            {formatAvailableDeviceActionError(
              'Import device',
              importMutation.error
            )}
          </Text>
        ) : null}
        {importMutation.isSuccess ? (
          <Text style={{ color: semantics.statusSuccess }}>
            Device imported in a paused state. You can activate it later from
            discovery.
          </Text>
        ) : null}
      </YStack>
    </Card>
  );
}

function SupportBadge({ support }: { support: AvailableDeviceSupport }) {
  const semantics = useThemeSemantics();
  const colors = supportToneColors(support.tone, semantics);

  return (
    <XStack
      alignSelf="flex-start"
      alignItems="center"
      gap="$1"
      paddingHorizontal="$2"
      paddingVertical="$1"
      borderRadius={999}
      borderWidth={1}
      style={{
        backgroundColor: colors.backgroundColor,
        borderColor: colors.borderColor
      }}
    >
      <MaterialCommunityIcons
        name={colors.icon}
        size={14}
        color={colors.textColor}
      />
      <Text fontSize="$2" fontWeight="700" style={{ color: colors.textColor }}>
        {support.label}
      </Text>
    </XStack>
  );
}

function supportToneColors(
  tone: AvailableDeviceSupport['tone'],
  semantics: SemanticColors
): {
  backgroundColor: string;
  borderColor: string;
  textColor: string;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
} {
  switch (tone) {
    case 'success':
      return {
        backgroundColor: semantics.statusSuccessBackground,
        borderColor: semantics.statusSuccessBorder,
        textColor: semantics.statusSuccess,
        icon: 'check-circle-outline'
      };
    case 'warning':
      return {
        backgroundColor: semantics.statusWarningBackground,
        borderColor: semantics.statusWarningBorder,
        textColor: semantics.statusWarning,
        icon: 'alert-circle-outline'
      };
    default:
      return {
        backgroundColor: semantics.mutedPanelBackground,
        borderColor: semantics.mutedPanelBorder,
        textColor: semantics.subtleStrongText,
        icon: 'flask-outline'
      };
  }
}

function formatProbeStatus(result: DeviceMQTTTestResult): string {
  if (result.success) {
    if (result.deviceId) {
      return 'MQTT live. Device enabled and activated from the observed payload.';
    }
    const bytes = result.payloadBytes ? Number(result.payloadBytes) : 0;
    const sizeText =
      Number.isFinite(bytes) && bytes > 0 ? `${bytes} bytes` : 'a live payload';
    return `MQTT live. Received ${sizeText}${result.sampleTopic ? ` on ${result.sampleTopic}` : ''}.`;
  }
  switch (result.status) {
    case 'timeout':
    case 'no_messages':
      return 'MQTT connected, but no live data arrived before the probe timed out.';
    case 'connect_rejected':
      return 'MQTT broker rejected the connection. Check provider credentials and device readiness.';
    case 'subscribe_rejected':
      return 'MQTT broker rejected the subscription for this device.';
    case 'connect_failed':
      return 'Could not connect to the MQTT broker for this device.';
    case 'subscribe_failed':
      return 'Connected to MQTT, but subscribing to the device feed failed.';
    default:
      return 'MQTT test failed. Try again in a moment.';
  }
}

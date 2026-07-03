import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { router } from 'expo-router';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import type {
  AvailableDeviceSummary,
  DeviceMQTTTestResult,
  EdgeCollector,
  EdgeDeviceSource
} from '@/features/devices/api';
import {
  useAvailableDevices,
  useApproveEdgeDeviceSource,
  useCreateEdgeCollector,
  useEdgeCollectors,
  useEdgeDeviceSources,
  useImportAvailableDevice,
  useTestAvailableDeviceMQTT
} from '@/features/devices/hooks';
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
import { formatAvailableDeviceActionError } from '@/features/devices/actionMessages';
import { maskSerialNumber } from '@/features/telemetry/format';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Card } from '@/shared/ui/Card';

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
  const availableQuery = useAvailableDevices({
    token,
    authKey,
    enabled: enabled && activated
  });

  return (
    <Card gap="$3" marginTop="$3">
      <XStack alignItems="center" justifyContent="space-between" gap="$3" flexWrap="wrap">
        <YStack gap="$1" flex={1} minWidth={240}>
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons name="plus-box-multiple-outline" size={20} color="rgba(10,132,255,0.9)" />
            <Text fontSize="$6" fontWeight="700">
              Available devices
            </Text>
          </XStack>
          <Text color="$colorMuted">
            Scan for devices linked to your provider credentials, then enable and activate them with a live MQTT check.
          </Text>
        </YStack>

        {!activated ? (
          <Button
            size="$3"
            onPress={() => setActivated(true)}
            icon={<MaterialCommunityIcons name="radar" size={16} color="white" />}
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
                <Spinner size="small" color="rgba(10,132,255,0.9)" />
              ) : (
                <MaterialCommunityIcons name="refresh" size={16} color="rgba(10,132,255,0.9)" />
              )
            }
          >
            Refresh
          </Button>
        )}
      </XStack>

      {!activated ? (
        <Text color="$colorMuted">
          This keeps provider discovery explicit and only runs when you ask for it.
        </Text>
      ) : null}

      {activated && availableQuery.isLoading && !availableQuery.data ? (
        <XStack alignItems="center" gap="$2" minHeight={72}>
          <Spinner size="small" />
          <Text color="$colorMuted">Checking your provider account for new devices…</Text>
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
            backgroundColor="rgba(245, 158, 11, 0.10)"
            borderColor="rgba(245, 158, 11, 0.32)"
          >
            <XStack alignItems="center" gap="$2">
              <MaterialCommunityIcons name="alert-outline" size={18} color="rgba(245, 158, 11, 0.96)" />
              <Text fontWeight="700">Connector attention needed</Text>
            </XStack>
            <Text color="$colorMuted">{availableQuery.data.warningMessage}</Text>
          </YStack>
          <XStack justifyContent="flex-end">
            <Button
              size="$3"
              onPress={() => router.push('/settings/integrations')}
              icon={<MaterialCommunityIcons name="cog-outline" size={16} color="white" />}
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
          No active provider credentials are available yet, so there’s nothing to scan.
        </Text>
      ) : null}

      {activated &&
      availableQuery.data &&
      !availableQuery.isError &&
      !availableQuery.data.warningMessage &&
      availableQuery.data.hasActiveCredentials &&
      availableQuery.data.devices.length === 0 ? (
        <Text color="$colorMuted">No unconfigured devices were found on the latest scan.</Text>
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
  const [collectorName, setCollectorName] = useState('Raspberry Pi');
  const [setupToken, setSetupToken] = useState('');
  const authStatusQuery = useEcoFlowBLEAuthStatus({ token, authKey, enabled });
  const collectorsQuery = useEdgeCollectors({ token, authKey, enabled });
  const sourcesQuery = useEdgeDeviceSources({ token, authKey, enabled, status: 'pending' });
  const connectAuth = useConnectEcoFlowBLEAuth({ token, authKey });
  const setManualAuth = useSetEcoFlowBLEAuthUserID({ token, authKey });
  const createCollector = useCreateEdgeCollector({ token, authKey });
  const approveSource = useApproveEdgeDeviceSource({ token, authKey });
  const connected = authStatusQuery.data?.connected === true;
  const collectors = collectorsQuery.data ?? [];
  const sources = sourcesQuery.data ?? [];
  const authBusy = connectAuth.isPending || setManualAuth.isPending;

  useEffect(() => {
    setEmail('');
    setPassword('');
    setManualUserId('');
    setSetupToken('');
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
        userId: manualUserId,
        accountLabel: 'Manual EcoFlow BLE ID'
      },
      { onSettled: () => setManualUserId('') }
    );
  }

  function handleCreateCollector() {
    createCollector.mutate(
      {
        displayName: collectorName.trim() || undefined
      },
      { onSuccess: (created) => setSetupToken(created.setupToken) }
    );
  }

  function handleApproveSource(source: EdgeDeviceSource) {
    approveSource.mutate(
      {
        sourceId: source.id,
        productName: source.displayName || source.providerDeviceId,
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
      borderColor="rgba(18,140,88,0.18)"
      backgroundColor="rgba(18,140,88,0.05)"
    >
      <XStack alignItems="center" justifyContent="space-between" gap="$3" flexWrap="wrap">
        <YStack gap="$1" flex={1} minWidth={220}>
          <XStack alignItems="center" gap="$2">
            <MaterialCommunityIcons name="bluetooth" size={18} color="rgba(18,140,88,0.96)" />
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
            void authStatusQuery.refetch();
            void collectorsQuery.refetch();
            void sourcesQuery.refetch();
          }}
          disabled={authStatusQuery.isFetching || collectorsQuery.isFetching || sourcesQuery.isFetching}
          icon={
            authStatusQuery.isFetching || collectorsQuery.isFetching || sourcesQuery.isFetching ? (
              <Spinner size="small" color="rgba(18,140,88,0.96)" />
            ) : (
              <MaterialCommunityIcons name="refresh" size={16} color="rgba(18,140,88,0.96)" />
            )
          }
        >
          Refresh BLE
        </Button>
      </XStack>

      {!connected ? (
        <YStack gap="$3">
          <XStack gap="$2" flexWrap="wrap">
            <AppTextInput
              compact
              flex={1}
              minWidth={210}
              value={email}
              onChangeText={setEmail}
              autoCapitalize="none"
              keyboardType="email-address"
              placeholder="EcoFlow email"
            />
            <AppTextInput
              compact
              flex={1}
              minWidth={210}
              value={password}
              onChangeText={setPassword}
              secureTextEntry
              placeholder="EcoFlow password"
            />
            <Button
              size="$3"
              disabled={authBusy || email.trim().length === 0 || password.length === 0}
              onPress={handleConnectAuth}
              icon={connectAuth.isPending ? <Spinner size="small" color="white" /> : <MaterialCommunityIcons name="login" size={16} color="white" />}
            >
              Connect
            </Button>
          </XStack>
          <XStack gap="$2" flexWrap="wrap">
            <AppTextInput
              compact
              flex={1}
              minWidth={240}
              value={manualUserId}
              onChangeText={setManualUserId}
              autoCapitalize="none"
              placeholder="Manual BLE user ID"
            />
            <Button
              size="$3"
              disabled={authBusy || manualUserId.trim().length === 0}
              onPress={handleManualAuth}
              icon={setManualAuth.isPending ? <Spinner size="small" color="white" /> : <MaterialCommunityIcons name="key-outline" size={16} color="white" />}
            >
              Save manual ID
            </Button>
          </XStack>
          {connectAuth.isError || setManualAuth.isError ? (
            <Text color="rgba(185,28,28,0.96)">
              {String(connectAuth.error ?? setManualAuth.error)}
            </Text>
          ) : null}
        </YStack>
      ) : null}

      <YStack gap="$2">
        <XStack gap="$2" flexWrap="wrap" alignItems="center">
          <AppTextInput
            compact
            flex={1}
            minWidth={220}
            value={collectorName}
            onChangeText={setCollectorName}
            placeholder="Collector name"
          />
          <Button
            size="$3"
            disabled={!connected || createCollector.isPending}
            onPress={handleCreateCollector}
            icon={createCollector.isPending ? <Spinner size="small" color="white" /> : <MaterialCommunityIcons name="raspberry-pi" size={16} color="white" />}
          >
            Create setup token
          </Button>
        </XStack>
        {setupToken ? (
          <AppTextInput compact value={setupToken} editable={false} selectTextOnFocus />
        ) : null}
        {createCollector.isError ? (
          <Text color="rgba(185,28,28,0.96)">{String(createCollector.error)}</Text>
        ) : null}
        <Text color="$colorMuted">{formatCollectorSummary(collectors)}</Text>
      </YStack>

      <YStack gap="$2">
        <XStack alignItems="center" justifyContent="space-between" gap="$2" flexWrap="wrap">
          <Text fontWeight="700">BLE discoveries</Text>
          <Text color="$colorMuted">{sources.length === 1 ? '1 pending source' : `${sources.length} pending sources`}</Text>
        </XStack>
        {sourcesQuery.isLoading && !sourcesQuery.data ? (
          <XStack alignItems="center" gap="$2">
            <Spinner size="small" />
            <Text color="$colorMuted">Loading BLE discoveries…</Text>
          </XStack>
        ) : null}
        {sourcesQuery.isError ? (
          <Text color="rgba(185,28,28,0.96)">{String(sourcesQuery.error)}</Text>
        ) : null}
        {!sourcesQuery.isLoading && sources.length === 0 ? (
          <Text color="$colorMuted">No pending BLE sources have checked in yet.</Text>
        ) : null}
        {sources.map((source) => (
          <EdgeDeviceSourceRow
            key={source.id}
            source={source}
            busy={approveSource.isPending}
            onApprove={() => {
              handleApproveSource(source);
            }}
          />
        ))}
        {approveSource.isError ? (
          <Text color="rgba(185,28,28,0.96)">{String(approveSource.error)}</Text>
        ) : null}
      </YStack>
    </YStack>
  );
}

function EdgeDeviceSourceRow({
  source,
  busy,
  onApprove
}: {
  source: EdgeDeviceSource;
  busy: boolean;
  onApprove: () => void;
}) {
  return (
    <XStack
      alignItems="center"
      justifyContent="space-between"
      gap="$3"
      flexWrap="wrap"
      padding="$3"
      borderRadius="$3"
      borderWidth={1}
      borderColor="rgba(18,140,88,0.16)"
      backgroundColor="rgba(255,255,255,0.42)"
    >
      <YStack gap="$1" flex={1} minWidth={220}>
        <Text fontWeight="700">{source.displayName || source.providerDeviceId}</Text>
        <Text color="$colorMuted">
          {source.model || 'EcoFlow BLE'} · RSSI {source.rssiDbm} dBm · {formatEdgeSourceSeen(source.lastSeenAtUnixMs)}
        </Text>
      </YStack>
      <Button
        size="$3"
        disabled={busy}
        onPress={onApprove}
        icon={busy ? <Spinner size="small" color="white" /> : <MaterialCommunityIcons name="link-variant-plus" size={16} color="white" />}
      >
        Approve
      </Button>
    </XStack>
  );
}

function formatCollectorSummary(collectors: EdgeCollector[]): string {
  if (collectors.length === 0) {
    return 'No edge collectors are registered yet.';
  }
  const active = collectors.filter((collector) => collector.isActive).length;
  return `${collectors.length} registered collector${collectors.length === 1 ? '' : 's'} · ${active} active`;
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
  const [probeResult, setProbeResult] = useState<DeviceMQTTTestResult | null>(null);
  const testMutation = useTestAvailableDeviceMQTT({ token, authKey });
  const importMutation = useImportAvailableDevice({ token, authKey });
  const busy = testMutation.isPending || importMutation.isPending;
  const support = describeAvailableDeviceSupport(device);
  const enableableByCatalog = device.provider === 'anker_solix' ? support?.enableable === true : true;

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
    <Card gap="$3" backgroundColor="rgba(10,132,255,0.04)" borderColor="rgba(10,132,255,0.16)">
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
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
              backgroundColor="rgba(10,132,255,0.12)"
            >
              <MaterialCommunityIcons name="new-box" size={14} color="rgba(10,132,255,0.92)" />
              <Text fontSize="$2" fontWeight="700" color="rgba(10,132,255,0.92)">
                New
              </Text>
            </XStack>
          </XStack>
          <Text color="$colorMuted">{device.model}</Text>
          <Text color="$colorMuted">
            {formatProviderLabel(device.provider)} · {maskSerialNumber(device.serialNumber)}
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
            disabled={busy || !enableableByCatalog || probeResult?.success === true}
            icon={
              testMutation.isPending ? (
                <Spinner size="small" color="white" />
              ) : (
                <MaterialCommunityIcons name="check-circle-outline" size={16} color="white" />
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
                <MaterialCommunityIcons name="pause-circle-outline" size={16} color="white" />
              )
            }
          >
            Import inactive
          </Button>
        </YStack>
      </XStack>

      <YStack gap="$1" minHeight={46}>
        {probeResult ? (
          <Text color={probeResult.success ? 'rgba(18,140,88,0.96)' : '$colorMuted'}>
            {formatProbeStatus(probeResult)}
          </Text>
        ) : (
          <Text color="$colorMuted">
            {enableableByCatalog
              ? 'Runs a live MQTT probe and activates this device only after telemetry is observed.'
              : 'This model is visible for tracking, but V1 does not enable standalone MQTT ingest for it yet.'}
          </Text>
        )}
        {testMutation.isError ? (
          <Text color="rgba(185,28,28,0.96)">
            {formatAvailableDeviceActionError('Enable and Activate', testMutation.error)}
          </Text>
        ) : null}
        {importMutation.isError ? (
          <Text color="rgba(185,28,28,0.96)">
            {formatAvailableDeviceActionError('Import device', importMutation.error)}
          </Text>
        ) : null}
        {importMutation.isSuccess ? (
          <Text color="rgba(18,140,88,0.96)">Device imported in a paused state. You can activate it later from discovery.</Text>
        ) : null}
      </YStack>
    </Card>
  );
}

function SupportBadge({ support }: { support: AvailableDeviceSupport }) {
  const colors = supportToneColors(support.tone);

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
      <MaterialCommunityIcons name={colors.icon} size={14} color={colors.textColor} />
      <Text fontSize="$2" fontWeight="700" style={{ color: colors.textColor }}>
        {support.label}
      </Text>
    </XStack>
  );
}

function supportToneColors(tone: AvailableDeviceSupport['tone']): {
  backgroundColor: string;
  borderColor: string;
  textColor: string;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
} {
  switch (tone) {
    case 'success':
      return {
        backgroundColor: 'rgba(18,140,88,0.12)',
        borderColor: 'rgba(18,140,88,0.26)',
        textColor: 'rgba(18,140,88,0.96)',
        icon: 'check-circle-outline'
      };
    case 'warning':
      return {
        backgroundColor: 'rgba(245,158,11,0.12)',
        borderColor: 'rgba(245,158,11,0.30)',
        textColor: 'rgba(180,83,9,0.96)',
        icon: 'alert-circle-outline'
      };
    default:
      return {
        backgroundColor: 'rgba(107,114,128,0.10)',
        borderColor: 'rgba(107,114,128,0.24)',
        textColor: 'rgba(75,85,99,0.96)',
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
    const sizeText = Number.isFinite(bytes) && bytes > 0 ? `${bytes} bytes` : 'a live payload';
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

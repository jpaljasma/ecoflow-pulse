import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useState } from 'react';
import { router } from 'expo-router';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import type { AvailableDeviceSummary, DeviceMQTTTestResult } from '@/features/devices/api';
import {
  useAvailableDevices,
  useEnableAvailableDevice,
  useImportAvailableDevice,
  useTestAvailableDeviceMQTT
} from '@/features/devices/hooks';
import {
  describeAvailableDeviceSupport,
  formatProviderLabel,
  type AvailableDeviceSupport
} from '@/features/integrations/providerCatalog';
import { maskSerialNumber } from '@/features/telemetry/format';
import { Card } from '@/shared/ui/Card';

type AvailableDevicesPanelProps = {
  token?: string;
  authKey?: string;
  enabled: boolean;
};

export function AvailableDevicesPanel({
  token,
  authKey = 'anonymous',
  enabled
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
            Scan for devices linked to your provider credentials, test live MQTT, then enable them.
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
            />
          ))}
        </YStack>
      ) : null}
    </Card>
  );
}

function AvailableDeviceCard({
  device,
  token,
  authKey
}: {
  device: AvailableDeviceSummary;
  token?: string;
  authKey: string;
}) {
  const [probeResult, setProbeResult] = useState<DeviceMQTTTestResult | null>(null);
  const testMutation = useTestAvailableDeviceMQTT({ token });
  const enableMutation = useEnableAvailableDevice({ token, authKey });
  const importMutation = useImportAvailableDevice({ token, authKey });
  const busy = testMutation.isPending || enableMutation.isPending || importMutation.isPending;
  const canEnable = probeResult?.success === true;
  const support = describeAvailableDeviceSupport(device);
  const enableableByCatalog = device.provider === 'anker_solix' ? support?.enableable === true : true;

  async function runProbe() {
    const result = await testMutation.mutateAsync({
      provider: device.provider,
      credentialId: device.credentialId,
      providerDeviceId: device.providerDeviceId
    });
    setProbeResult(result);
  }

  async function enableDevice() {
    await enableMutation.mutateAsync({
      provider: device.provider,
      credentialId: device.credentialId,
      providerDeviceId: device.providerDeviceId
    });
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
            disabled={busy || !enableableByCatalog}
            icon={
              testMutation.isPending ? (
                <Spinner size="small" color="white" />
              ) : (
                <MaterialCommunityIcons name="access-point" size={16} color="white" />
              )
            }
          >
            {probeResult ? 'Retest MQTT' : 'Test MQTT'}
          </Button>
          <Button
            size="$3"
            themeInverse
            onPress={() => {
              void enableDevice();
            }}
            disabled={!canEnable || busy || !enableableByCatalog}
            icon={
              enableMutation.isPending ? (
                <Spinner size="small" color="rgba(10,132,255,0.9)" />
              ) : (
                <MaterialCommunityIcons name="check-circle-outline" size={16} color="rgba(10,132,255,0.9)" />
              )
            }
          >
            Enable device
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
              ? 'Run a short MQTT probe first. Enable stays locked until live data is observed.'
              : 'This model is visible for tracking, but V1 does not enable standalone MQTT ingest for it yet.'}
          </Text>
        )}
        {enableMutation.isSuccess ? (
          <Text color="rgba(18,140,88,0.96)">Device enabled. It will move into the configured list after refresh.</Text>
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

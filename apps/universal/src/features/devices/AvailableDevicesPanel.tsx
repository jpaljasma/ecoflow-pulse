import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useState } from 'react';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import type { AvailableDeviceSummary, DeviceMQTTTestResult } from '@/features/devices/api';
import {
  useAvailableDevices,
  useEnableAvailableDevice,
  useTestAvailableDeviceMQTT
} from '@/features/devices/hooks';
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
      !availableQuery.isError &&
      !availableQuery.data.hasActiveCredentials ? (
        <Text color="$colorMuted">
          No active provider credentials are available yet, so there’s nothing to scan.
        </Text>
      ) : null}

      {activated &&
      availableQuery.data &&
      !availableQuery.isError &&
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
  const busy = testMutation.isPending || enableMutation.isPending;
  const canEnable = probeResult?.success === true;

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
            {device.provider.toUpperCase()} · {maskSerialNumber(device.serialNumber)}
          </Text>
        </YStack>

        <YStack gap="$2" minWidth={180}>
          <Button
            size="$3"
            onPress={() => {
              void runProbe();
            }}
            disabled={busy}
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
            disabled={!canEnable || busy}
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
        </YStack>
      </XStack>

      <YStack gap="$1" minHeight={46}>
        {probeResult ? (
          <Text color={probeResult.success ? 'rgba(18,140,88,0.96)' : '$colorMuted'}>
            {formatProbeStatus(probeResult)}
          </Text>
        ) : (
          <Text color="$colorMuted">
            Run a short MQTT probe first. Enable stays locked until live data is observed.
          </Text>
        )}
        {enableMutation.isSuccess ? (
          <Text color="rgba(18,140,88,0.96)">Device enabled. It will move into the configured list after refresh.</Text>
        ) : null}
      </YStack>
    </Card>
  );
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

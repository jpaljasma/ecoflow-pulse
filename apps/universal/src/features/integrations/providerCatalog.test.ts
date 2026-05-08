import { describe, expect, it } from 'vitest';

import type { AvailableDeviceSummary } from '@/features/devices/schema';
import type { Integration } from './schema';
import {
  buildProviderCredentialConfig,
  createProviderConfigDraft,
  describeAvailableDeviceSupport,
  formatIntegrationConfigSummary,
  formatProviderLabel
} from './providerCatalog';

describe('provider catalog helpers', () => {
  it('builds non-secret Anker SOLIX server and country config', () => {
    expect(
      buildProviderCredentialConfig(
        'anker_solix',
        createProviderConfigDraft('anker_solix', {
          server: 'eu',
          country: 'de'
        })
      )
    ).toEqual({ server: 'eu', country: 'DE' });

    expect(
      buildProviderCredentialConfig(
        'anker_solix',
        createProviderConfigDraft('anker_solix', {
          server: 'invalid',
          country: 'not-a-country'
        })
      )
    ).toEqual({ server: 'com', country: 'US' });
  });

  it('formats provider labels and saved credential config summaries', () => {
    expect(formatProviderLabel('anker_solix')).toBe('Anker SOLIX Cloud MQTT');
    expect(formatProviderLabel('pulsemqtt')).toBe('Pulse MQTT Emulator');
    expect(formatProviderLabel('custom_provider')).toBe('Custom Provider');

    const integration: Integration = {
      id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      provider: 'anker_solix',
      accessKeyMask: 'owne...test',
      config: { server: 'eu', country: 'DE' },
      isActive: true,
      createdAtUnixMs: '1773430000000',
      updatedAtUnixMs: '1773430800000'
    };

    expect(formatIntegrationConfigSummary(integration)).toBe('EU cloud, DE account');
  });

  it('describes Anker SOLIX support status from metadata without affecting other providers', () => {
    expect(
      describeAvailableDeviceSupport(makeAvailableDevice({
        provider: 'ecoflow',
        model: 'DELTA 2 Max'
      }))
    ).toBeNull();

    expect(
      describeAvailableDeviceSupport(makeAvailableDevice({
        provider: 'anker_solix',
        model: 'A1783',
        metadata: { support_status: 'partial' }
      }))
    ).toMatchObject({
      label: 'Partial support',
      tone: 'warning'
    });

    expect(
      describeAvailableDeviceSupport(makeAvailableDevice({
        provider: 'anker_solix',
        model: 'A7320',
        metadata: { support_status: 'companion' }
      }))
    ).toMatchObject({
      label: 'Companion telemetry',
      tone: 'warning'
    });

    expect(
      describeAvailableDeviceSupport(makeAvailableDevice({
        provider: 'anker_solix',
        model: 'A1785',
        metadata: { supportStatus: 'unsupported' }
      }))
    ).toMatchObject({
      label: 'Needs sample',
      tone: 'neutral'
    });
  });
});

function makeAvailableDevice(overrides: Partial<AvailableDeviceSummary>): AvailableDeviceSummary {
  return {
    provider: 'anker_solix',
    providerDeviceId: 'A1783:REDACTED',
    credentialId: 'cred-1',
    serialNumber: 'ANKER-A1783-001',
    name: 'Anker SOLIX C2000 Gen 2',
    model: 'A1783',
    ...overrides
  };
}

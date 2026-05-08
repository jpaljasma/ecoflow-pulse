import { describe, expect, it } from 'vitest';

import {
  CreateIntegrationPayloadSchema,
  IntegrationSchema,
  UpdateIntegrationPayloadSchema
} from './schema';

describe('integration schemas', () => {
  it('preserves Pecron region config while keeping credentials write-only', () => {
    const payload = CreateIntegrationPayloadSchema.parse({
      provider: 'pecron',
      accessKey: 'owner@example.test',
      accessSecret: 'pecron-password',
      config: { region: 'eu' },
      isActive: true
    });

    expect(payload).toMatchObject({
      provider: 'pecron',
      config: { region: 'eu' },
      isActive: true
    });

    const integration = IntegrationSchema.parse({
      id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      provider: 'pecron',
      accessKeyMask: 'owne...test',
      config: { region: 'eu' },
      isActive: true,
      createdAtUnixMs: '1773430000000',
      updatedAtUnixMs: '1773430800000'
    });

    expect(integration).not.toHaveProperty('accessKey');
    expect(integration).not.toHaveProperty('accessSecret');
    expect(integration.config.region).toBe('eu');
  });

  it('defaults provider config for legacy EcoFlow integration responses', () => {
    const integration = IntegrationSchema.parse({
      id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      provider: 'ecoflow',
      accessKeyMask: 'AK12...7890',
      isActive: true,
      createdAtUnixMs: '1773430000000',
      updatedAtUnixMs: '1773430800000'
    });

    expect(integration.config).toEqual({});
  });

  it('validates Pecron credential update submissions with region config', () => {
    const payload = UpdateIntegrationPayloadSchema.parse({
      accessKey: 'owner@example.test',
      accessSecret: 'new-password',
      config: { region: 'cn' },
      isActive: false
    });

    expect(payload.config).toEqual({ region: 'cn' });
    expect(() =>
      UpdateIntegrationPayloadSchema.parse({
        accessKey: '',
        accessSecret: 'new-password',
        config: { region: 'cn' }
      })
    ).toThrow();
  });

  it('preserves Anker SOLIX Cloud MQTT config while keeping credentials write-only', () => {
    const payload = CreateIntegrationPayloadSchema.parse({
      provider: 'anker_solix',
      accessKey: 'owner@example.test',
      accessSecret: 'anker-password',
      config: { server: 'com', country: 'US' },
      isActive: true
    });

    expect(payload).toMatchObject({
      provider: 'anker_solix',
      config: { server: 'com', country: 'US' },
      isActive: true
    });

    const integration = IntegrationSchema.parse({
      id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      provider: 'anker_solix',
      accessKeyMask: 'owne...test',
      config: { server: 'com', country: 'US' },
      isActive: true,
      createdAtUnixMs: '1773430000000',
      updatedAtUnixMs: '1773430800000'
    });

    expect(integration).not.toHaveProperty('accessKey');
    expect(integration).not.toHaveProperty('accessSecret');
    expect(integration.config).toEqual({ server: 'com', country: 'US' });
  });
});

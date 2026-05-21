import { describe, expect, it } from 'vitest';
import { AvailableDeviceSchema, DeviceMQTTTestResultSchema, DeviceSchema } from '@/features/devices/schema';

describe('device api schema', () => {
  it('preserves extended system-signal and diagnostics detail fields', () => {
    const device = DeviceSchema.parse({
      id: 'device-1',
      serialNumber: 'delta-test',
      name: 'Bench Delta',
      model: 'Delta Pro Ultra',
      online: true,
      batteryPct: 78,
      state: 'idle',
      etaMinutes: 0,
      details: {
        xBoostOn: true,
        solarMode: 'Solar Only',
        passthroughMode: 'L14 Transfer Switch',
        acAutoOnMode: 'Always On',
        energyManagementOn: true,
        diagnostics: [
          {
            key: 'dpu-time-task-conflict',
            label: 'Time Task Conflict',
            value: 'No Conflict',
            tone: 'success'
          }
        ]
      }
    });

    expect(device.details).toEqual(
      expect.objectContaining({
        xBoostOn: true,
        solarMode: 'Solar Only',
        passthroughMode: 'L14 Transfer Switch',
        acAutoOnMode: 'Always On',
        energyManagementOn: true,
        diagnostics: [
          expect.objectContaining({
            key: 'dpu-time-task-conflict',
            label: 'Time Task Conflict',
            value: 'No Conflict',
            tone: 'success'
          })
        ]
      })
    );
  });

  it('preserves available-device metadata for provider discovery badges', () => {
    const device = AvailableDeviceSchema.parse({
      provider: 'anker_solix',
      providerDeviceId: 'A1783:REDACTED',
      credentialId: 'cred-1',
      serialNumber: 'ANKER-A1783-001',
      name: 'Anker SOLIX C2000 Gen 2',
      model: 'A1783',
      capabilities: { mqttTelemetry: 'basic' },
      metadata: { support_status: 'partial' }
    });

    expect(device.capabilities).toEqual({ mqttTelemetry: 'basic' });
    expect(device.metadata).toEqual({ support_status: 'partial' });
  });

  it('preserves the enabled device id returned by a successful MQTT probe', () => {
    const result = DeviceMQTTTestResultSchema.parse({
      success: true,
      status: 'ok',
      sampleTopic: 'redacted',
      payloadBytes: '42',
      observedAtUnixMs: '1779318200000',
      deviceId: '22222222-2222-7222-8222-222222222222'
    });

    expect(result.deviceId).toBe('22222222-2222-7222-8222-222222222222');
  });
});

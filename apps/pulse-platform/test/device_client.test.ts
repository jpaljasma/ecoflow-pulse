import { describe, expect, it, vi } from 'vitest';
import type { FastifyRequest } from 'fastify';

import { createDeviceClient } from '../src/grpc/deviceClient.js';
import type { ControlPlaneClient, ProviderDevice } from '../src/grpc/controlPlaneClient.js';
import type { TelemetrySnapshotClient } from '../src/grpc/telemetryClient.js';
import type { AppConfig } from '../src/config.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    energyGrpcApiAddr: '127.0.0.1:9091',
    grpcDeadlineMs: 2500,
    devUserSubject: 'dev-user@example.com',
    publicPreconnectOrigins: [],
    historyRateLimit: {
      max: 120,
      timeWindowMs: 60000
    },
    auth: { mode: 'noop', allowMissingJwt: true }
  };
}

function makeRequest(): FastifyRequest {
  return {
    headers: {},
    id: 'req-1'
  } as FastifyRequest;
}

function makeProviderDevice(): ProviderDevice {
  return {
    id: 'pdev-1',
    deviceId: '22222222-2222-7222-8222-222222222222',
    provider: 'ecoflow',
    providerDeviceId: 'DEMOD2M00001057',
    credentialId: 'cred-1',
    canonicalSn: 'DEMOD2M00001057',
    productName: 'Kitchen Delta 2 Max',
    model: 'DELTA 2 Max',
    isActive: true,
    ingestDesiredState: 'active',
    capabilities: {
      battery_pack_count: 2,
      pv_input_count: 2,
      supports_ac_output: true,
      supports_dc_output: true,
      supports_usb_output: true
    },
    metadata: {
      groups: {
        pd: {
          soc: 53.53,
          remainTime: 1331,
          dcOutState: 1,
          typec1Watts: 42,
          pv2ChargeWatts: 0
        },
        inv: {
          cfgAcEnabled: 1,
          outputWatts: 101,
          fanState: 0
        },
        mppt: {
          inVol: 10502,
          inAmp: 195,
          outWatts: 5,
          chgState: 2,
          pv2InVol: 10500,
          pv2InAmp: 95,
          pv2ChgState: 1
        },
        bms_bmsStatus: {
          targetSoc: 53.53,
          inputWatts: 0,
          outputWatts: 0,
          temp: 29,
          fullCap: 39101
        },
        bms_emsStatus: {
          f32LcdShowSoc: 25.49,
          minDsgSoc: 10,
          maxChargeSoc: 90,
          minOpenOilEb: 15
        }
      }
    }
  };
}

describe('device client', () => {
  it('falls back to normalized solar port watts when snapshot pv metrics are absent', async () => {
    const controlPlaneClient: ControlPlaneClient = {
      listUserDevices: vi.fn(),
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ]),
      close: vi.fn()
    };
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'params.soc': 53.53,
            'params.wattsOutSum': 101,
            'params.typec1Watts': 62,
            'params.temp': 29
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.pvW).toBeCloseTo(5.9975, 4);
    expect(device?.netW).toBeCloseTo(-95.0025, 4);
    expect(device?.details?.solarPorts?.[0]).toEqual(
      expect.objectContaining({
        volts: 10.502,
        amps: 0.195,
        watts: 5
      })
    );
  });

  it('prefers normalized solar port watts when raw snapshot pv metrics are inflated', async () => {
    const controlPlaneClient: ControlPlaneClient = {
      listUserDevices: vi.fn(),
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ]),
      close: vi.fn()
    };
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            pvW: 1427592,
            'params.soc': 53.53,
            'params.wattsOutSum': 101,
            'params.typec1Watts': 62,
            'params.temp': 29
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.pvW).toBeCloseTo(5.9975, 4);
    expect(device?.netW).toBeCloseTo(-95.0025, 4);
  });

  it('prefers explicit zero solar ports over stale raw snapshot pv watts', async () => {
    const zeroSolarDevice = makeProviderDevice();
    const groups = (zeroSolarDevice.metadata?.groups ?? {}) as Record<string, Record<string, number>>;
    const mppt = groups.mppt ?? {};
    mppt.inVol = 0;
    mppt.inAmp = 0;
    mppt.outWatts = 0;
    mppt.pv2InVol = 0;
    mppt.pv2InAmp = 0;
    mppt.pv2InWatts = 0;
    groups.mppt = mppt;
    const pd = groups.pd ?? {};
    pd.pv2ChargeWatts = 0;
    groups.pd = pd;
    zeroSolarDevice.metadata = {
      ...(zeroSolarDevice.metadata ?? {}),
      groups
    };

    const controlPlaneClient: ControlPlaneClient = {
      listUserDevices: vi.fn(),
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [zeroSolarDevice]
        }
      ]),
      close: vi.fn()
    };
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            pvW: 260,
            'params.soc': 53.53,
            'params.wattsOutSum': 101,
            'params.temp': 29
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.details?.solarPorts?.[0]?.watts).toBe(0);
    expect(device?.details?.solarPorts?.[1]?.watts).toBe(0);
    expect(device?.pvW).toBe(0);
    expect(device?.netW).toBe(-101);
  });

  it('prefers aggregate device soc from quota-derived details over main-pack target soc', async () => {
    const controlPlaneClient: ControlPlaneClient = {
      listUserDevices: vi.fn(),
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ]),
      close: vi.fn()
    };
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'params.soc': 22.94,
            'params.targetSoc': 22.94,
            'params.wattsOutSum': 101
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.batteryPct).toBeCloseTo(25.49, 2);
    expect(device?.details?.overallSocPct).toBeCloseTo(25.49, 2);
  });
});

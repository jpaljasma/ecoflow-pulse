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
    corsAllowedOrigins: [],
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

function makePecronProviderDevice(): ProviderDevice {
  return {
    id: 'pdev-pecron-1',
    deviceId: '33333333-3333-7333-8333-333333333333',
    provider: 'pecron',
    providerDeviceId: 'p11vxg:redacted',
    credentialId: 'cred-1',
    canonicalSn: 'PECRON-P11VXG-REDACTED',
    productName: 'Pecron E1000LFP',
    model: 'E1000LFP',
    isActive: true,
    ingestDesiredState: 'active',
    capabilities: {
      battery_capacity_wh: 1024,
      battery_pack_count: 1,
      pv_input_count: 1,
      pv_input_max_watts: 600,
      pv_input_max_volts: 60,
      pv_input_max_amps: 20,
      supports_ac_output: true,
      supports_dc_output: true,
      supports_usb_output: true,
      supports_ups_mode: true,
      supports_battery_heating: true
    },
    metadata: {
      provider: 'pecron',
      field_names: [
        'battery_percentage',
        'host_packet_data_jdb.host_packet_voltage',
        'host_packet_data_jdb.host_packet_current',
        'dc_data_input_hm.dc_input_power',
        'ac_switch_hm',
        'dc_switch_hm',
        'ups_status_hm'
      ]
    }
  };
}

function makeControlPlaneClient(
  overrides: Partial<ControlPlaneClient> = {}
): ControlPlaneClient {
  return {
    getCurrentUser: vi.fn(),
    updateCurrentUser: vi.fn(),
    refreshCurrentUserIdentity: vi.fn(),
    listProviderCredentials: vi.fn(async () => []),
    createProviderCredential: vi.fn(),
    updateProviderCredential: vi.fn(),
    setProviderCredentialActive: vi.fn(),
    listUserDevices: vi.fn(),
    listDevices: vi.fn(async () => []),
    listAvailableProviderDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    testProviderDeviceMQTT: vi.fn(),
    enableProviderDevice: vi.fn(),
    importProviderDevice: vi.fn(),
    close: vi.fn(),
    ...overrides
  };
}

describe('device client', () => {
  const futureStormEnd = () => Math.floor((Date.now() + 60 * 60 * 1000) / 1000);
  const pastStormEnd = () => Math.floor((Date.now() - 60 * 60 * 1000) / 1000);

  it('preserves available device metadata for provider discovery UI', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listAvailableProviderDevices: vi.fn(async () => ({
        hasActiveCredentials: true,
        devices: [
          {
            provider: 'anker_solix',
            providerDeviceId: 'A1783:REDACTED',
            credentialId: 'cred-1',
            canonicalSn: 'ANKER-A1783-001',
            productName: 'Anker SOLIX C2000 Gen 2',
            model: 'A1783',
            capabilities: { mqttTelemetry: 'basic' },
            metadata: { family: 'power_station', support_status: 'partial' }
          }
        ]
      }))
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(),
      close: vi.fn()
    };
    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);

    const result = await client.listAvailableDevices(makeRequest());

    expect(result).toEqual({
      hasActiveCredentials: true,
      devices: [
        {
          provider: 'anker_solix',
          providerDeviceId: 'A1783:REDACTED',
          credentialId: 'cred-1',
          serialNumber: 'ANKER-A1783-001',
          name: 'Anker SOLIX C2000 Gen 2',
          model: 'A1783',
          capabilities: { mqttTelemetry: 'basic' },
          metadata: { family: 'power_station', support_status: 'partial' }
        }
      ]
    });
  });

  it('falls back to normalized solar port watts when snapshot pv metrics are absent', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ])
    });
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

    expect(device?.pvW).toBeCloseTo(3.04539, 4);
    expect(device?.netW).toBeCloseTo(-97.95461, 4);
    expect(device?.details?.solarPorts?.[0]).toEqual(
      expect.objectContaining({
        volts: 10.502,
        amps: 0.195,
        watts: 2.04789
      })
    );
  });

  it('prefers normalized solar port watts when raw snapshot pv metrics are inflated', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ])
    });
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

    expect(device?.pvW).toBeCloseTo(3.04539, 4);
    expect(device?.netW).toBeCloseTo(-97.95461, 4);
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

    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [zeroSolarDevice]
        }
      ])
    });
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

  it('hydrates Pecron live MQTT readings into PV and battery-pack details', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'pecron',
          devices: [makePecronProviderDevice()]
        }
      ])
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '33333333-3333-7333-8333-333333333333',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'params.soc': 68,
            'params.f32ShowSoc': 68,
            'params.batVol': 52.5,
            'params.batAmp': -0.8,
            'params.temp': 22,
            'params.wattsInSum': 184,
            'params.pv1ChargeWatts': 184,
            'params.outAcTtPwr': 50,
            'params.wattsOutSum': 50,
            'params.cfgAcEnabled': 1,
            'params.dcOutState': 0,
            'params.upsMode': 1
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.batteryPct).toBe(68);
    expect(device?.pvW).toBe(184);
    expect(device?.capabilities).toEqual(
      expect.objectContaining({
        batteryCapacityKWh: 1.024,
        pvInputCount: 1
      })
    );
    expect(device?.details?.packs?.[0]).toEqual(
      expect.objectContaining({
        id: 'main',
        socPct: 68,
        powerW: -42,
        tempC: 22,
        energyWh: 1024
      })
    );
    expect(device?.details?.solarPorts?.[0]).toEqual(
      expect.objectContaining({
        id: 'pv-1',
        watts: 184,
        maxWatts: 600,
        maxVolts: 60,
        maxAmps: 20
      })
    );
    expect(device?.details?.acOn).toBe(true);
    expect(device?.details?.dcOn).toBe(false);
    expect(device?.details?.solarChargingOn).toBe(true);
  });

  it('prefers aggregate device soc from quota-derived details over main-pack target soc', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ])
    });
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

  it('normalizes milli-unit battery voltage and current before hydrating pack watts', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ])
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'params.f32LcdShowSoc': 69.5,
            'params.batVol': 50288,
            'params.batAmp': -316.8,
            'params.temp': 32,
            'params.wattsOutSum': 0
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.details?.packs?.[0]).toEqual(
      expect.objectContaining({
        id: 'main',
        socPct: 69.5,
        powerW: expect.closeTo(-15.93, 2),
        tempC: 32
      })
    );
  });

  it('uses provider remain-time details when live snapshot ETA fields are absent', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [makeProviderDevice()]
        }
      ])
    });
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
            'params.temp': 29
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [device] = await client.listDevices(makeRequest());

    expect(device?.etaMinutes).toBe(1331);
    expect(device?.details?.estimateEtaMin).toBe(1331);
    expect(device?.details?.remainGlobalMin).toBe(1331);
  });

  it('surfaces storm guard from live snapshot metrics when metadata groups do not include it', async () => {
    const device = makeProviderDevice();
    device.metadata = {
      groups: {
        ...(device.metadata?.groups ?? {})
      }
    };

    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [device]
        }
      ])
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'param.stormPatternEnable': 1,
            'param.stormPatternOpenFlag': 1,
            'param.stormPatternEndTime': futureStormEnd(),
            'params.soc': 53.53,
            'params.wattsOutSum': 101
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [hydrated] = await client.listDevices(makeRequest());

    expect(hydrated?.details?.stormGuardActive).toBe(true);
    expect(hydrated?.details?.stormGuardEndsAtUnixMs).toBe(futureStormEnd() * 1000);
  });

  it('does not surface storm guard from snapshot metrics when the feature is enabled but inactive', async () => {
    const device = makeProviderDevice();
    device.metadata = {
      groups: {
        ...(device.metadata?.groups ?? {})
      }
    };

    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [device]
        }
      ])
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'param.stormPatternEnable': 1,
            'param.stormPatternOpenFlag': 0,
            'param.stormPatternEndTime': 0,
            'params.soc': 53.53,
            'params.wattsOutSum': 101
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [hydrated] = await client.listDevices(makeRequest());

    expect(hydrated?.details?.stormGuardActive).toBe(false);
    expect(hydrated?.details?.stormGuardEndsAtUnixMs).toBeUndefined();
  });

  it('does not surface storm guard from expired snapshot windows when the open flag is false', async () => {
    const device = makeProviderDevice();
    device.metadata = {
      groups: {
        ...(device.metadata?.groups ?? {})
      }
    };

    const controlPlaneClient = makeControlPlaneClient({
      listDevices: vi.fn(async () => [
        {
          provider: 'ecoflow',
          devices: [device]
        }
      ])
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(async () => ({
        snapshot: {
          deviceId: '22222222-2222-7222-8222-222222222222',
          cursor: {
            seq: '1',
            tsUnixMs: String(Date.now())
          },
          metrics: {
            'param.stormPatternEnable': 0,
            'param.stormPatternOpenFlag': 0,
            'param.stormPatternEndTime': pastStormEnd(),
            'params.soc': 53.53,
            'params.wattsOutSum': 101
          }
        }
      })),
      close: vi.fn()
    };

    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const [hydrated] = await client.listDevices(makeRequest());

    expect(hydrated?.details?.stormGuardActive).toBe(false);
    expect(hydrated?.details?.stormGuardEndsAtUnixMs).toBe(pastStormEnd() * 1000);
  });

  it('maps inactive provider-device imports through the control-plane client', async () => {
    const importProviderDevice = vi.fn(async () => ({
      providerDevice: makeProviderDevice(),
      userDevice: {
        deviceId: '22222222-2222-7222-8222-222222222222',
        ecoflowSn: 'PULSEDPUX24K001',
        productName: 'DPU-X 24 kWh',
        model: 'DELTA Pro Ultra X',
        role: 'admin',
        createdAtUnixMs: '1772197190000',
        updatedAtUnixMs: '1772197190000'
      }
    }));
    const controlPlaneClient = makeControlPlaneClient({ importProviderDevice });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(),
      close: vi.fn()
    };
    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);

    const result = await client.importAvailableDevice(makeRequest(), {
      provider: 'pulsemqtt',
      credentialId: 'cred-1',
      providerDeviceId: 'PULSEDPUX24K001',
      isActive: false,
      ingestDesiredState: 'paused'
    });

    expect(importProviderDevice).toHaveBeenCalledWith(
      expect.objectContaining({
        userSubject: 'dev-user@example.com',
        provider: 'pulsemqtt',
        credentialId: 'cred-1',
        providerDeviceId: 'PULSEDPUX24K001',
        isActive: false,
        ingestDesiredState: 'paused'
      })
    );
    expect(result).toEqual({ deviceId: '22222222-2222-7222-8222-222222222222' });
  });

  it('uses the MQTT validation deadline budget for active provider-device enablement', async () => {
    const testProviderDeviceMQTT = vi.fn(async () => ({
      success: true,
      status: 'ok',
      sampleTopic: 'redacted',
      payloadBytes: '128',
      observedAtUnixMs: '1779318200000',
      deviceId: '22222222-2222-7222-8222-222222222222'
    }));
    const enableProviderDevice = vi.fn(async () => ({
      providerDevice: makeProviderDevice(),
      userDevice: {
        deviceId: '22222222-2222-7222-8222-222222222222',
        ecoflowSn: 'PECRON-P11VXG-TESTDEVICE0001',
        productName: 'Pecron E1000LFP',
        model: 'E1000LFP',
        role: 'admin',
        createdAtUnixMs: '1779318200000',
        updatedAtUnixMs: '1779318200000'
      }
    }));
    const importProviderDevice = vi.fn(async () => ({
      providerDevice: makeProviderDevice(),
      userDevice: {
        deviceId: '22222222-2222-7222-8222-222222222222',
        ecoflowSn: 'PECRON-P11VXG-TESTDEVICE0001',
        productName: 'Pecron E1000LFP',
        model: 'E1000LFP',
        role: 'admin',
        createdAtUnixMs: '1779318200000',
        updatedAtUnixMs: '1779318200000'
      }
    }));
    const controlPlaneClient = makeControlPlaneClient({
      testProviderDeviceMQTT,
      enableProviderDevice,
      importProviderDevice
    });
    const telemetryClient: TelemetrySnapshotClient = {
      getSnapshot: vi.fn(),
      close: vi.fn()
    };
    const client = createDeviceClient(baseConfig(), controlPlaneClient, telemetryClient);
    const input = {
      provider: 'pecron',
      credentialId: 'cred-1',
      providerDeviceId: 'p11vxg:testdevice0001'
    };

    await client.testAvailableDeviceMQTT(makeRequest(), input);
    await client.enableAvailableDevice(makeRequest(), input);
    await client.importAvailableDevice(makeRequest(), {
      ...input,
      isActive: true,
      ingestDesiredState: 'active'
    });
    await client.importAvailableDevice(makeRequest(), {
      ...input,
      isActive: false,
      ingestDesiredState: 'paused'
    });

    expect(testProviderDeviceMQTT).toHaveBeenCalledWith(expect.objectContaining({ deadlineMs: 14_500 }));
    expect(enableProviderDevice).toHaveBeenCalledWith(expect.objectContaining({ deadlineMs: 14_500 }));
    expect(importProviderDevice).toHaveBeenNthCalledWith(1, expect.objectContaining({ deadlineMs: 14_500 }));
    expect(importProviderDevice).toHaveBeenNthCalledWith(2, expect.objectContaining({ deadlineMs: 2500 }));
  });
});

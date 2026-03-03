import { describe, expect, it } from 'vitest';

import { buildProviderDevicePresentation } from '../src/devices/providerDeviceMapper.js';
import type { ProviderDevice } from '../src/grpc/controlPlaneClient.js';

function baseProviderDevice(overrides: Partial<ProviderDevice> = {}): ProviderDevice {
  return {
    id: 'pdev-1',
    deviceId: '019ca747-392b-720e-9ff3-09c4130c838f',
    provider: 'ecoflow',
    providerDeviceId: 'Y711ZABA9H2P0294',
    credentialId: 'cred-1',
    canonicalSn: 'Y711ZABA9H2P0294',
    productName: 'DPU A 12 kWh',
    model: 'DELTA Pro Ultra',
    isActive: true,
    ingestDesiredState: 'active',
    capabilities: {},
    metadata: {},
    ...overrides
  };
}

describe('provider device mapper', () => {
  it('maps DPU quota metadata into capabilities and details', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        capabilities: {
          battery_pack_count: 2,
          pv_input_count: 2,
          supports_ac_output: true,
          supports_dc_output: true,
          supports_usb_output: true,
          supports_parallel: true,
          supports_extra_battery: true
        },
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              inLvMpptPwr: 77,
              inHvMpptPwr: 0,
              outAdsPwr: 24,
              outUsb1Pwr: 18,
              outPrPwr: 0
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 64.2,
              inLvMpptAmp: 1.2,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 1
            },
            hs_yj751_pd_app_set_info_addr: {
              bmsModeSet: 1
            },
            hs_yj751_pd_bp_addr: {
              bpInfo: [
                { bpNo: 'bp1', bpSoc: 48.5, bpPwr: 12.1, bpTemp: 19.5, heatTime: 120 },
                { bpNo: 'bp2', bpSoc: 47.9, bpPwr: 11.8, bpTemp: 19.3, heatTime: 0 }
              ]
            }
          }
        }
      })
    );

    expect(presentation.serialNumber).toBe('Y711ZABA9H2P0294');
    expect(presentation.capabilities).toEqual(
      expect.objectContaining({
        batteryPacks: 2,
        pvInputCount: 2,
        batteryCapacityKWh: 12,
        evCharging: true,
        batteryHeating: true,
        preconditioning: true,
        acOutput: true,
        dcOutput: true,
        usbOutput: true,
        parallel: true,
        extraBattery: true
      })
    );
    expect(presentation.details).toEqual(
      expect.objectContaining({
        bpCount: 2,
        fanOn: true,
        dc12vOn: true,
        usbOn: true,
        solarChargingOn: true,
        batteryHeatingOn: true,
        packs: [
          expect.objectContaining({ id: 'bp1', heatingOn: true }),
          expect.objectContaining({ id: 'bp2', heatingOn: false })
        ],
        solarPorts: [
          expect.objectContaining({ id: 'pv-low', state: 'charging', maxWatts: 1600 }),
          expect.objectContaining({ id: 'pv-high', state: 'inactive', maxWatts: 4000 })
        ]
      })
    );
  });

  it('maps D2M quota metadata into capabilities and details', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b562',
        providerDeviceId: 'R351ZABAPH331057',
        canonicalSn: 'R351ZABAPH331057',
        productName: 'Kitchen Delta 2 Max',
        model: 'DELTA 2 Max',
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
              soc: 31.5,
              dcOutState: 1,
              typec1Watts: 42,
              pv2ChargeWatts: 212
            },
            inv: {
              cfgAcEnabled: 1,
              outputWatts: 138,
              fanState: 1
            },
            mppt: {
              inVol: 16.4,
              inAmp: 5,
              outWatts: 82,
              chgState: 2,
              pv2InVol: 40.9,
              pv2InAmp: 5.18,
              pv2ChgState: 1
            },
            bms_bmsStatus: {
              targetSoc: 30.8,
              inputWatts: 10,
              outputWatts: 79,
              temp: 13
            },
            bms_kitInfo: {
              kitNum: 1,
              watts: [
                {
                  avaFlag: 1,
                  sn: 'bp1',
                  targetSoc: 31.2,
                  curPower: 22,
                  temp: 13.4
                }
              ]
            }
          }
        }
      })
    );

    expect(presentation.serialNumber).toBe('R351ZABAPH331057');
    expect(presentation.capabilities).toEqual(
      expect.objectContaining({
        batteryPacks: 2,
        pvInputCount: 2,
        batteryCapacityKWh: 4.096,
        acOutput: true,
        dcOutput: true,
        usbOutput: true
      })
    );
    expect(presentation.details).toEqual(
      expect.objectContaining({
        bpCount: 2,
        acOn: true,
        dcOn: true,
        usbOn: true,
        fanOn: true,
        solarChargingOn: true,
        batteryHeatingOn: false,
        packs: expect.arrayContaining([
          expect.objectContaining({ id: 'main', powerW: -69 }),
          expect.objectContaining({ id: 'bp1', powerW: 22 })
        ]),
        solarPorts: [
          expect.objectContaining({ id: 'pv-1', state: 'charging', watts: 82 }),
          expect.objectContaining({ id: 'pv-2', state: 'charging', watts: 212 })
        ]
      })
    );
  });
});

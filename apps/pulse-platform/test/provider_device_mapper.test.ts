import { describe, expect, it } from 'vitest';

import { buildProviderDevicePresentation } from '../src/devices/providerDeviceMapper.js';
import type { ProviderDevice } from '../src/grpc/controlPlaneClient.js';

function baseProviderDevice(overrides: Partial<ProviderDevice> = {}): ProviderDevice {
  return {
    id: 'pdev-1',
    deviceId: '019ca747-392b-720e-9ff3-09c4130c838f',
    provider: 'ecoflow',
    providerDeviceId: 'DEMODPU0000294',
    credentialId: 'cred-1',
    canonicalSn: 'DEMODPU0000294',
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
  const futureStormEnd = () => Math.floor((Date.now() + 60 * 60 * 1000) / 1000);
  const pastStormEnd = () => Math.floor((Date.now() - 60 * 60 * 1000) / 1000);

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
              outPrPwr: 0,
              access_5p8InType: 3,
              timeTaskConflictFlag: 0,
              chgTimeTaskType: 0,
              dsgTimeTaskType: -1,
              stormPatternEnable: 1,
              stormPatternOpenFlag: 1,
              stormPatternEndTime: futureStormEnd()
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 64.2,
              inLvMpptAmp: 1.2,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 1,
              sysWorkSta: 1,
              pcsWorkSta: 2,
              work_5p8Mode: 2
            },
            hs_yj751_pd_app_set_info_addr: {
              sysWorkMode: 2,
              acXboost: 1,
              bmsModeSet: 1,
              acOftenOpenFlg: 1,
              solarOnlyFlg: 1,
              energyManageEnable: 1,
              dsgMinSoc: 12,
              chgMaxSoc: 95,
              sysBackupSoc: 18,
              sysTimezone: -500,
              sysTimezoneId: 'America/New_York',
              timezoneSettype: 0
            },
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: [
                  {
                    bpNo: 'bp1',
                    bpSoc: 48.5,
                    bpPwr: 12.1,
                    bpTemp: 19.5,
                    heatTime: 120,
                    bpEnergy: 5980,
                    remainTime: 322,
                    bpChgSta: 1,
                    bpErrCode: 0,
                    bpSocMin: 10,
                    bpSocMax: 95
                  },
                  {
                    bpNo: 'bp2',
                    bpSoc: 47.9,
                    bpPwr: 11.8,
                    bpTemp: 19.3,
                    heatTime: 0,
                    bpEnergy: 6015,
                    remainTime: 317,
                    bpChgSta: 0,
                    bpErrCode: 2,
                    bpSocMin: 10,
                    bpSocMax: 95
                  }
                ]
              }
            }
          }
        }
      })
    );

    expect(presentation.serialNumber).toBe('DEMODPU0000294');
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
        xBoostOn: true,
        solarMode: 'Solar Only',
        passthroughMode: 'L14 Transfer Switch',
        acAutoOnMode: 'Always On',
        energyManagementOn: true,
        socWindowMinPct: 12,
        socWindowMaxPct: 95,
        backupReservePct: 18,
        packs: [
          expect.objectContaining({
            id: 'bp1',
            heatingOn: true,
            energyWh: 5980,
            remainMinutes: 322,
            socMinPct: 10,
            socMaxPct: 95
          }),
          expect.objectContaining({
            id: 'bp2',
            heatingOn: false,
            energyWh: 6015,
            remainMinutes: 317,
            socMinPct: 10,
            socMaxPct: 95
          })
        ],
        solarPorts: [
          expect.objectContaining({ id: 'pv-low', state: 'charging', maxWatts: 1600 }),
          expect.objectContaining({ id: 'pv-high', state: 'inactive', maxWatts: 4000 })
        ],
        diagnostics: expect.arrayContaining([
          expect.objectContaining({ key: 'dpu-sys-work-mode', value: '2', tone: 'info' }),
          expect.objectContaining({ key: 'dpu-time-task-conflict', value: 'No Conflict', tone: 'success' }),
          expect.objectContaining({ key: 'dpu-charge-task-type', value: 'AC Charge', tone: 'info' }),
          expect.objectContaining({ key: 'bp-err-2', value: '2', tone: 'danger' })
        ]),
        stormGuardActive: true,
        stormGuardEndsAtUnixMs: futureStormEnd() * 1000,
        timezoneId: 'America/New_York',
        timezoneOffsetMinutes: -300,
        timezoneMode: 'manual'
      })
    );
  });

  it('uses the Ultra X high-PV envelope for DELTA Pro Ultra X devices', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        model: 'DELTA Pro Ultra X',
        productName: 'DPU-X 24 kWh',
        capabilities: {
          battery_pack_count: 4
        },
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              soc: 72.6,
              inLvMpptPwr: 4100
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 311.2,
              inLvMpptAmp: 13.17,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 1
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        overallSocPct: 72.6,
        solarPorts: expect.arrayContaining([
          expect.objectContaining({
            id: 'pv-1',
            name: 'PV 1',
            state: 'charging',
            maxWatts: 5000,
            maxVolts: 500,
            maxAmps: 15
          }),
          expect.objectContaining({
            id: 'pv-2',
            name: 'PV 2',
            state: 'inactive',
            maxWatts: 5000,
            maxVolts: 500,
            maxAmps: 15
          })
        ])
      })
    );
    expect(presentation.capabilities).toEqual(
      expect.objectContaining({
        batteryPacks: 4,
        batteryCapacityKWh: 24.576
      })
    );
  });

  it('keeps DPU passthrough visible when Power I/O access types are present but idle', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              access_5p8InType: 0,
              access_5p8OutType: 0
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 0,
              inLvMpptAmp: 0,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 0
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        passthroughMode: 'Idle'
      })
    );
  });

  it('infers DPU AC pass-through from live AC power when access flags stay idle', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              access_5p8InType: 0,
              access_5p8OutType: 0,
              inAc5p8Pwr: 128,
              outAc_5p8Pwr: 128
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 0,
              inLvMpptAmp: 0,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 0
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        passthroughMode: 'AC Pass-Through'
      })
    );
  });

  it('infers DPU L14 transfer mode from live AC input plus L14 output power', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              inAcC20Pwr: 605,
              outAcL14Pwr: 357
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 0,
              inLvMpptAmp: 0,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 0
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        passthroughMode: 'L14 Transfer Switch'
      })
    );
  });

  it('treats DPU L14 and Power I/O AC outputs as active AC output paths', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              outAcL14Pwr: 320,
              outAc_5p8Pwr: 0
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 0,
              inLvMpptAmp: 0,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 0
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        acOn: true
      })
    );
  });

  it('treats DPU L14 and Power I/O AC outputs as active AC output paths', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        metadata: {
          groups: {
            hs_yj751_pd_appshow_addr: {
              outAcL14Pwr: 320,
              outAc_5p8Pwr: 0
            },
            hs_yj751_pd_backend_addr: {
              inLvMpptVol: 0,
              inLvMpptAmp: 0,
              inHvMpptVol: 0,
              inHvMpptAmp: 0,
              fanState: 0
            },
            hs_yj751_pd_app_set_info_addr: {},
            hs_yj751_pd_bp_addr: {
              bpInfo: {
                values: []
              }
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        acOn: true
      })
    );
  });

  it('maps D2M quota metadata into capabilities and details', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b562',
        providerDeviceId: 'DEMOD2M00001057',
        canonicalSn: 'DEMOD2M00001057',
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
              pv2ChargeWatts: 212,
              chgDsgState: 1,
              pvChargePrioSet: 1,
              newAcAutoOnCfg: 1
            },
            inv: {
              cfgAcEnabled: 1,
              cfgAcXboost: 1,
              acPassbyAutoEn: 1,
              outputWatts: 138,
              fanState: 1
            },
            mppt: {
              inVol: 16400,
              inAmp: 5000,
              outWatts: 82,
              chgState: 2,
              pv2InVol: 40900,
              pv2InAmp: 5180,
              pv2ChgState: 1
            },
            bms_bmsStatus: {
              targetSoc: 30.8,
              inputWatts: 10,
              outputWatts: 79,
              temp: 13,
              fullCap: 2048,
              remainTime: 301
            },
            bms_emsStatus: {
              f32LcdShowSoc: 25.49,
              minDsgSoc: 5,
              maxChargeSoc: 85,
              minOpenOilEb: 21,
              bmsWarState: 4,
              emsIsNormalFlag: 1
            },
            bms_kitInfo: {
              kitNum: 1,
              watts: {
                values: [
                  {
                    avaFlag: 1,
                    sn: 'bp1',
                    targetSoc: 31.2,
                    curPower: 22,
                    temp: 13.4,
                    energy: 2048,
                    remainTime: 287,
                    socMin: 5,
                    socMax: 85
                  }
                ]
              }
            }
          }
        }
      })
    );

    expect(presentation.serialNumber).toBe('DEMOD2M00001057');
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
        xBoostOn: true,
        solarMode: 'Charge Priority',
        passthroughMode: 'Auto Passby',
        acAutoOnMode: 'Auto On',
        socWindowMinPct: 5,
        socWindowMaxPct: 85,
        backupReservePct: 21,
        overallSocPct: 25.49,
        diagnostics: expect.arrayContaining([
          expect.objectContaining({ key: 'd2m-bms-warning', value: 'Overload', tone: 'warning' }),
          expect.objectContaining({ key: 'd2m-energy-state', value: 'Normal', tone: 'success' }),
          expect.objectContaining({ key: 'd2m-charge-discharge-state', value: 'Charging', tone: 'info' })
        ]),
        packs: expect.arrayContaining([
          expect.objectContaining({
            id: 'main',
            powerW: -69,
            energyWh: 2048,
            remainMinutes: 301,
            socMinPct: 5,
            socMaxPct: 85
          }),
          expect.objectContaining({
            id: 'bp1',
            powerW: 22,
            energyWh: 2048,
            remainMinutes: 287,
            socMinPct: 5,
            socMaxPct: 85
          })
        ]),
        solarPorts: [
          expect.objectContaining({ id: 'pv-1', state: 'charging', watts: 82, volts: 16.4, amps: 5 }),
          expect.objectContaining({ id: 'pv-2', state: 'charging', watts: 212, volts: 40.9, amps: 5.18 })
        ]
      })
    );
  });

  it('derives numbered D2M solar ports dynamically for future multi-port models', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b564',
        providerDeviceId: 'DEMOD2M00001058',
        canonicalSn: 'DEMOD2M00001058',
        productName: 'Delta 2 Max Future',
        model: 'DELTA 2 Max',
        capabilities: {
          battery_pack_count: 2,
          supports_ac_output: true
        },
        metadata: {
          groups: {
            pd: {
              pv3ChargeWatts: 155
            },
            mppt: {
              inVol: 16400,
              inAmp: 5000,
              outWatts: 82,
              chgState: 2,
              pv2InVol: 40900,
              pv2InAmp: 5180,
              pv2ChargeWatts: 212,
              pv2ChgState: 1,
              pv3InVol: 38100,
              pv3InAmp: 4070,
              pv3ChgState: 2
            }
          }
        }
      })
    );

    expect(presentation.capabilities).toEqual(
      expect.objectContaining({
        pvInputCount: 3
      })
    );
    expect(presentation.details?.solarPorts).toEqual([
      expect.objectContaining({ id: 'pv-1', name: 'PV 1', watts: 82 }),
      expect.objectContaining({ id: 'pv-2', name: 'PV 2', watts: 212 }),
      expect.objectContaining({ id: 'pv-3', name: 'PV 3', watts: 155 })
    ]);
  });

  it('does not use total mppt watts as pv-1 watts for multi-port devices', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b564',
        providerDeviceId: 'DEMOD2M00001059',
        canonicalSn: 'DEMOD2M00001059',
        productName: 'Delta 2 Max Multi Port',
        model: 'DELTA 2 Max',
        capabilities: {
          pv_input_count: 2
        },
        metadata: {
          groups: {
            mppt: {
              inVol: 48000,
              inAmp: 1000,
              outWatts: 710,
              inWatts: 710,
              chgState: 2,
              pv2InVol: 40900,
              pv2InAmp: 5180,
              pv2ChargeWatts: 212,
              pv2ChgState: 1
            }
          }
        }
      })
    );

    expect(presentation.details?.solarPorts).toEqual([
      expect.objectContaining({ id: 'pv-1', watts: 48, volts: 48, amps: 1 }),
      expect.objectContaining({ id: 'pv-2', watts: 212, volts: 40.9, amps: 5.18 })
    ]);
  });

  it('maps Delta 2 quota metadata into capabilities and details', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b563',
        providerDeviceId: 'DEMODELTA200001',
        canonicalSn: 'DEMODELTA200001',
        productName: 'Office Delta 2',
        model: 'DELTA 2',
        capabilities: {
          battery_pack_count: 1,
          pv_input_count: 1,
          supports_ac_output: true,
          supports_dc_output: true,
          supports_usb_output: true,
          supports_extra_battery: true
        },
        metadata: {
          groups: {
            pd: {
              soc: 62,
              bpPowerSoc: 27,
              minAcSoc: 21,
              acAutoOnCfg: 1,
              chgDsgState: 0,
              dcOutState: 1,
              typec1Watts: 58,
              usb1Watts: 12,
              wireWatts: 19
            },
            inv: {
              cfgAcEnabled: 1,
              cfgAcXboost: 1,
              outputWatts: 184,
              fanState: 1
            },
            mppt: {
              inVol: 23100,
              inAmp: 4200,
              outWatts: 91,
              chgState: 1
            },
            ems: {
              f32LcdShowSoc: 61.8,
              minDsgSoc: 15,
              maxChargeSoc: 92,
              emsIsNormalFlag: 0,
              bmsWarState: 1
            },
            bmsMaster: {
              soc: 61,
              inputWatts: 45,
              outputWatts: 133,
              temp: 19,
              fullCap: 1024,
              remainTime: 211
            }
          }
        }
      })
    );

    expect(presentation.capabilities).toEqual(
      expect.objectContaining({
        batteryPacks: 1,
        pvInputCount: 1,
        batteryCapacityKWh: 1.024,
        acOutput: true,
        dcOutput: true,
        usbOutput: true,
        extraBattery: true
      })
    );
    expect(presentation.details).toEqual(
      expect.objectContaining({
        bpCount: 1,
        acOn: true,
        dcOn: true,
        usbOn: true,
        fanOn: true,
        solarChargingOn: true,
        xBoostOn: true,
        acAutoOnMode: 'Auto On',
        overallSocPct: 61.8,
        socWindowMinPct: 15,
        socWindowMaxPct: 92,
        backupReservePct: 27,
        diagnostics: expect.arrayContaining([
          expect.objectContaining({ key: 'd2-bms-warning', value: 'High Temp', tone: 'warning' }),
          expect.objectContaining({ key: 'd2-energy-state', value: 'Sleep', tone: 'warning' }),
          expect.objectContaining({ key: 'd2-charge-discharge-state', value: 'Discharging', tone: 'info' })
        ]),
        packs: [
          expect.objectContaining({
            id: 'main',
            socPct: 61,
            powerW: -88,
            energyWh: 1024,
            remainMinutes: 211,
            socMinPct: 15,
            socMaxPct: 92
          })
        ],
        solarPorts: [expect.objectContaining({ id: 'pv-1', state: 'charging', watts: 91, volts: 23.1, amps: 4.2 })]
      })
    );
  });

  it('treats zero-amp locked D2M PV ports as non-producing', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b562',
        providerDeviceId: 'DEMOD2M00001057',
        canonicalSn: 'DEMOD2M00001057',
        productName: 'Kitchen Delta 2 Max',
        model: 'DELTA 2 Max',
        metadata: {
          groups: {
            pd: {
              pv2ChargeWatts: 0
            },
            mppt: {
              inVol: 22000,
              inAmp: 0,
              outWatts: 7,
              chgState: 1,
              pv2InVol: 26000,
              pv2InAmp: 0,
              pv2InWatts: 0,
              pv2ChgState: 0
            }
          }
        }
      })
    );

    expect(presentation.details?.solarPorts).toEqual([
      expect.objectContaining({ id: 'pv-1', state: 'locked', volts: 22, amps: 0, watts: 0 }),
      expect.objectContaining({ id: 'pv-2', state: 'idle', volts: 26, amps: 0, watts: 0 })
    ]);
  });

  it('does not advertise preconditioning on Delta 2 Max or Delta 2', () => {
    const d2m = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b562',
        providerDeviceId: 'DEMOD2M00001057',
        canonicalSn: 'DEMOD2M00001057',
        productName: 'Kitchen Delta 2 Max',
        model: 'DELTA 2 Max',
        metadata: {
          groups: {
            pd: {},
            inv: {},
            mppt: {},
            bms_bmsStatus: {},
            bms_emsStatus: {},
            bms_kitInfo: {}
          }
        }
      })
    );
    const d2 = buildProviderDevicePresentation(
      baseProviderDevice({
        deviceId: '019ca747-3923-7d05-ac88-090bb4c7b563',
        providerDeviceId: 'DEMOD200001001',
        canonicalSn: 'DEMOD200001001',
        productName: 'Delta 2',
        model: 'DELTA 2',
        metadata: {
          groups: {
            pd: {},
            inv: {},
            mppt: {},
            ems: {},
            bmsMaster: {}
          }
        }
      })
    );

    expect(d2m.details).not.toHaveProperty('batteryHeatingOn');
    expect(d2.details).not.toHaveProperty('batteryHeatingOn');
  });

  it('derives storm guard from alternate EcoFlow storm field names for future devices', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        providerDeviceId: 'DEMOSTREAM0001',
        canonicalSn: 'DEMOSTREAM0001',
        productName: 'Stream Ultra',
        model: 'STREAM Ultra',
        metadata: {
          groups: {
            pd: {
              stormIsEnable: 1,
              inStormMode: 1,
              stormEndTimestamp: futureStormEnd()
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        stormGuardActive: true,
        stormGuardEndsAtUnixMs: futureStormEnd() * 1000
      })
    );
  });

  it('treats storm guard as active when EcoFlow reports an enabled protection window without an open flag', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        providerDeviceId: 'DEMODPUSTORM0001',
        canonicalSn: 'DEMODPUSTORM0001',
        productName: 'DPU A 12 kWh',
        model: 'DELTA Pro Ultra',
        metadata: {
          groups: {
            hs_yj751_pd_app_set_info_addr: {
              stormPatternEnable: 1,
              stormPatternOpenFlag: 0,
              stormPatternEndTime: futureStormEnd()
            }
          }
        }
      })
    );

    expect(presentation.details).toEqual(
      expect.objectContaining({
        stormGuardActive: true,
        stormGuardEndsAtUnixMs: futureStormEnd() * 1000
      })
    );
  });

  it('does not treat an expired storm guard window as active when open flag is false', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        providerDeviceId: 'DEMODPUSTORMEXPIRED0001',
        canonicalSn: 'DEMODPUSTORMEXPIRED0001',
        productName: 'DPU A 12 kWh',
        model: 'DELTA Pro Ultra',
        metadata: {
          groups: {
            hs_yj751_pd_app_set_info_addr: {
              stormPatternEnable: 0,
              stormPatternOpenFlag: 0,
              stormPatternEndTime: pastStormEnd()
            }
          }
        }
      })
    );

    expect(presentation.details?.stormGuardActive).toBe(false);
    expect(presentation.details?.stormGuardEndsAtUnixMs).toBe(pastStormEnd() * 1000);
  });

  it('does not treat storm guard as active for DPU d_addr enable-only payloads', () => {
    const presentation = buildProviderDevicePresentation(
      baseProviderDevice({
        providerDeviceId: 'Y711ZABA9H2P0294',
        canonicalSn: 'Y711ZABA9H2P0294',
        productName: 'DPU A 12 kWh',
        model: 'DELTA Pro Ultra',
        metadata: {
          groups: {
            d_addr: {
              stormPatternEnable: true,
              stormPatternOpenFlag: false,
              stormPatternEndTime: 0,
              backupReverseSoc: 40
            }
          }
        }
      })
    );

    expect(presentation.details).not.toHaveProperty('stormGuardActive', true);
  });
});

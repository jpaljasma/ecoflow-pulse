export type DeviceState = 'charging' | 'discharging' | 'idle';

export type MockDevice = {
  id: string;
  serialNumber: string;
  name: string;
  model: string;
  online: boolean;
  batteryPct: number;
  state: DeviceState;
  etaMinutes: number;
  pvW?: number;
  acInW?: number;
  dcW?: number;
  loadW?: number;
  netW?: number;
  tempC?: number;
  telemetryTsMs?: number;
  capabilities?: Record<string, unknown>;
};

// Seeded from recent mqtt.log energy_summary snapshots.
export const mockDevices: MockDevice[] = [
  {
    id: 'Y711ZABA9H2P0294',
    serialNumber: 'Y711ZABA9H2P0294',
    name: 'DPU A 12 kWh',
    model: 'DELTA Pro Ultra',
    online: true,
    batteryPct: 31.5,
    state: 'discharging',
    etaMinutes: 3168,
    pvW: 0,
    acInW: 0,
    dcW: 0,
    loadW: 0,
    netW: 0,
    tempC: 0,
    telemetryTsMs: 0,
    capabilities: {
      batteryPacks: 2,
      pvInputs: ['low', 'high'],
      acOutputWMax: 7200
    }
  },
  {
    id: 'R351ZABAPH331057',
    serialNumber: 'R351ZABAPH331057',
    name: 'Kitchen Delta 2 Max',
    model: 'DELTA 2 Max',
    online: true,
    batteryPct: 35.0,
    state: 'discharging',
    etaMinutes: 1761,
    pvW: 0,
    acInW: 0,
    dcW: 0,
    loadW: 0,
    netW: 0,
    tempC: 0,
    telemetryTsMs: 0,
    capabilities: {
      batteryPacks: 2,
      pvInputs: ['port1', 'port2'],
      acOutputWMax: 2400
    }
  }
];

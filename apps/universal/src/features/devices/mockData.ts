export type DeviceState = 'charging' | 'discharging' | 'idle';

export type BatteryPackDetail = {
  id: string;
  socPct?: number;
  powerW?: number;
  tempC?: number;
  heatingOn?: boolean;
};

export type SolarPortDetail = {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
  maxVolts?: number;
  maxAmps?: number;
  maxWatts?: number;
};

export type DeviceTelemetryDetails = {
  bpCount?: number;
  packs?: BatteryPackDetail[];
  solarPorts?: SolarPortDetail[];
  estimateMode?: string;
  estimateSource?: string;
  estimateEtaMin?: number;
  remainChargeMin?: number;
  remainDischargeMin?: number;
  remainGlobalMin?: number;
  mpptLowState?: string;
  mpptHighState?: string;
  acOn?: boolean;
  dcOn?: boolean;
  usbOn?: boolean;
  dc12vOn?: boolean;
  evChargingOn?: boolean;
  fanOn?: boolean;
  solarChargingOn?: boolean;
  batteryHeatingOn?: boolean;
  mqttQueueDepth?: number;
  mqttQueueDroppedOldest?: number;
};

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
  solarTodayWh?: number;
  solarGeneratedSeriesWh?: number[];
  tempC?: number;
  telemetryTsMs?: number;
  capabilities?: Record<string, unknown>;
  details?: DeviceTelemetryDetails;
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
    solarTodayWh: 724,
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
    solarTodayWh: 0,
    tempC: 0,
    telemetryTsMs: 0,
    capabilities: {
      batteryPacks: 2,
      pvInputs: ['port1', 'port2'],
      acOutputWMax: 2400
    }
  }
];

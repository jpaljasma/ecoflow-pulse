import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceLiveSignals, DeviceLiveSolarPort } from '@/features/telemetry/engine/types';

type DeviceDetails = NonNullable<DeviceSummary['details']>;
export type StaticSolarPortDetail = NonNullable<DeviceDetails['solarPorts']>[number];
type StaticSolarPortDetails = StaticSolarPortDetail[];

export function resolveLiveBatteryHeatingOn(
  liveSignals: DeviceLiveSignals | undefined,
  details: DeviceSummary['details'] | undefined
): boolean | undefined {
  if (liveSignals?.batteryHeatingOn !== undefined) {
    return liveSignals.batteryHeatingOn;
  }
  if (!details) {
    return undefined;
  }
  return details.batteryHeatingOn === true || (details.packs ?? []).some((pack) => pack.heatingOn === true);
}

function defaultSolarPortName(id: string): string {
  switch (id) {
    case 'pv-low':
      return 'PV Low';
    case 'pv-high':
      return 'PV High';
    case 'pv-1':
      return 'PV 1';
    case 'pv-2':
      return 'PV 2';
    default:
      return id;
  }
}

function liveSolarPortToStaticDetail(port: DeviceLiveSolarPort): StaticSolarPortDetail {
  return {
    id: port.id,
    name: port.name || defaultSolarPortName(port.id),
    state: port.state,
    volts: port.volts,
    amps: port.amps,
    watts: port.watts
  };
}

export function mergeDeviceDetailSolarPorts(
  staticPorts: StaticSolarPortDetails | undefined,
  livePorts: DeviceLiveSolarPort[] | undefined
): StaticSolarPortDetail[] | undefined {
  if (!livePorts || livePorts.length === 0) {
    return staticPorts;
  }
  if (!staticPorts || staticPorts.length === 0) {
    return livePorts.map((port) => liveSolarPortToStaticDetail(port));
  }

  const liveById = new Map(livePorts.map((port) => [port.id, port] as const));
  let matched = 0;
  const mergedById = staticPorts.map((port) => {
    const live = liveById.get(port.id);
    if (!live) {
      return port;
    }
    matched += 1;
    return {
      ...port,
      state: live.state ?? port.state,
      volts: live.volts ?? port.volts,
      amps: live.amps ?? port.amps,
      watts: live.watts ?? port.watts
    };
  });
  if (matched > 0) {
    return mergedById;
  }
  if (staticPorts.length === livePorts.length) {
    return staticPorts.map((port, index) => {
      const live = livePorts[index];
      if (!live) {
        return port;
      }
      return {
        ...port,
        state: live.state ?? port.state,
        volts: live.volts ?? port.volts,
        amps: live.amps ?? port.amps,
        watts: live.watts ?? port.watts
      };
    });
  }
  return staticPorts;
}

export function sumSolarPortWatts(
  ports: Array<{ watts?: number }> | undefined
): number | undefined {
  if (!ports || ports.length === 0) {
    return undefined;
  }
  let found = false;
  let total = 0;
  for (const port of ports) {
    if (typeof port.watts === 'number' && Number.isFinite(port.watts)) {
      total += port.watts;
      found = true;
    }
  }
  return found ? total : undefined;
}

import { useMemo } from 'react';
import type { ComponentProps } from 'react';
import type { ImageSourcePropType } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { getCapacityKWh } from '@/features/devices/capacity';
import { resolveNetPowerW } from '@/features/devices/net';
import { resolveDeviceVisualAssets } from '@/features/devices/deviceVisuals';
import { resolveDeviceSocPct } from '@/features/devices/soc';

export type FleetSummaryStats = {
  totalCapacityKWh: number | null;
  avgSocPct: number | null;
  acInW: number | undefined;
  dcW: number | undefined;
  pvW: number | undefined;
  loadW: number | undefined;
  netW: number | undefined;
};

export type FleetTypeIcon = {
  key: string;
  label: string;
  uri?: string;
  fallback?: ImageSourcePropType;
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  active: boolean;
};

function typeKey(model: string): string {
  const m = model.toLowerCase();
  if (m.includes('delta 2 max')) return 'delta_2_max';
  if (m.includes('delta pro ultra x')) return 'delta_pro_ultra_x';
  if (m.includes('delta pro ultra')) return 'delta_pro_ultra';
  if (m.includes('delta pro 3')) return 'delta_pro_3';
  if (m.includes('delta pro')) return 'delta_pro';
  if (m.includes('delta 3 ultra')) return 'delta_3_ultra';
  if (m.includes('delta 3 max')) return 'delta_3_max';
  if (m.includes('delta 3 plus')) return 'delta_3_plus';
  if (m.includes('delta 3')) return 'delta_3';
  return m;
}

export function useFleetSummaryViewModel({
  devices,
  byId,
  useRemoteImage
}: {
  devices: DeviceSummary[];
  byId: Record<string, DeviceSnapshot>;
  useRemoteImage: boolean;
}): {
  summary: FleetSummaryStats;
  uniqueTypes: FleetTypeIcon[];
} {
  const summary = useMemo<FleetSummaryStats>(() => {
    if (!devices.length) {
      return {
        totalCapacityKWh: null,
        avgSocPct: null,
        acInW: undefined,
        dcW: undefined,
        pvW: undefined,
        loadW: undefined,
        netW: undefined
      };
    }

    let totalCapacity = 0;
    let capacityCount = 0;
    let socSum = 0;
    let acInW = 0;
    let dcW = 0;
    let pvW = 0;
    let loadW = 0;
    let netW = 0;

    for (const device of devices) {
      const cap = getCapacityKWh(device);
      if (cap !== null) {
        totalCapacity += cap;
        capacityCount += 1;
      }

      const snapshot = byId[device.id];
      const soc = resolveDeviceSocPct({ device, snapshot }) ?? 0;
      socSum += soc;

      acInW += snapshot?.metrics?.acW ?? device.acInW ?? 0;
      dcW += snapshot?.metrics?.dcW ?? device.dcW ?? 0;
      pvW += snapshot?.metrics?.pvW ?? device.pvW ?? 0;
      loadW += snapshot?.metrics?.loadW ?? device.loadW ?? 0;
      netW += resolveNetPowerW({
        acInW: snapshot?.metrics?.acW ?? device.acInW,
        pvW: snapshot?.metrics?.pvW ?? device.pvW,
        dcW: snapshot?.metrics?.dcW ?? device.dcW,
        loadW: snapshot?.metrics?.loadW ?? device.loadW,
        fallbackNetW: device.netW ?? 0
      }) ?? 0;
    }

    return {
      totalCapacityKWh: capacityCount ? totalCapacity : null,
      avgSocPct: socSum / devices.length,
      acInW,
      dcW,
      pvW,
      loadW,
      netW
    };
  }, [devices, byId]);

  const uniqueTypes = useMemo<FleetTypeIcon[]>(() => {
    const seen = new Set<string>();
    const out: FleetTypeIcon[] = [];

    for (const device of devices) {
      const { match, imageUri, fallbackSource } = resolveDeviceVisualAssets(device, {
        useRemoteImage,
        imageContext: 'list'
      });
      const key = typeKey(device.model);
      if (seen.has(key)) continue;
      seen.add(key);
      const devicesOfType = devices.filter((d) => typeKey(d.model) === key);
      const hasActive = devicesOfType.some((d) => {
        const snap = byId[d.id];
        if (snap) return !snap.inactive;
        return false;
      });
      out.push({
        key,
        label: match.glyph.label,
        uri: imageUri,
        fallback: fallbackSource,
        icon: match.glyph.icon,
        active: hasActive
      });
    }
    return out;
  }, [devices, byId, useRemoteImage]);

  return { summary, uniqueTypes };
}

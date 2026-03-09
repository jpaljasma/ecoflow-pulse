import { formatW } from '@/features/telemetry/format';
import {
  clampPercent,
  formatSolarState,
  isInactivePvPort,
  toneFromState,
  toPctOfMax
} from '@/shared/ui/uiMappings';
import type { UiTone } from '@/shared/ui/uiMappings';

export function solarPortView(port: {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
  maxVolts?: number;
  maxAmps?: number;
  maxWatts?: number;
}) {
  const hasFlow =
    (Number.isFinite(port.watts as number) && (port.watts as number) > 1) ||
    (Number.isFinite(port.amps as number) && (port.amps as number) > 0.03);
  const stateLabelRaw = formatSolarState(port.state).toLowerCase();
  const inactive = isInactivePvPort(port.volts) && !hasFlow;
  const stateLabel =
    inactive
      ? 'inactive'
      : hasFlow && (stateLabelRaw === 'unknown' || stateLabelRaw === 'inactive' || stateLabelRaw === 'idle')
        ? 'active'
        : stateLabelRaw;
  const stateTone: UiTone = inactive ? 'neutral' : toneFromState(stateLabel);
  const pvLoadPct = toPctOfMax(port.watts, port.maxWatts);

  return {
    id: port.id,
    name: port.name,
    stateLabel,
    stateTone,
    inactive,
    wattsText: formatW(port.watts),
    voltsText: Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—',
    ampsText: Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—',
    capText: port.maxWatts ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A` : '—',
    watts: port.watts,
    volts: port.volts,
    amps: port.amps,
    maxWatts: port.maxWatts,
    pvLoadPct,
    pvLoadClamped: clampPercent(pvLoadPct ?? 0)
  };
}

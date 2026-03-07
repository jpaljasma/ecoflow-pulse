import {
  deriveTelemetryDetail,
  deriveTelemetryMetrics,
  mergeRawMetrics,
  type RawTelemetryMetrics
} from '../telemetryMap.js';
import type { ServerDeviceStatusMessage, ServerTelemetryMessage } from '../schemas.js';

export type DeliveryStage = 'fast' | 'steady' | 'slow' | 'key-only' | 'paused';

export type DeliveryConfig = {
  fastIntervalMs: number;
  steadyIntervalMs: number;
  slowIntervalMs: number;
  highWatermark: number;
  bufferedAmountHighWaterBytes: number;
  quietTicksToRecover: number;
};

type StageDescriptor = {
  name: DeliveryStage;
  intervalMs: number;
  keyOnly: boolean;
  paused: boolean;
};

const stages = (config: DeliveryConfig): StageDescriptor[] => [
  { name: 'fast', intervalMs: config.fastIntervalMs, keyOnly: false, paused: false },
  { name: 'steady', intervalMs: config.steadyIntervalMs, keyOnly: false, paused: false },
  { name: 'slow', intervalMs: config.slowIntervalMs, keyOnly: false, paused: false },
  { name: 'key-only', intervalMs: config.slowIntervalMs, keyOnly: true, paused: false },
  { name: 'paused', intervalMs: config.slowIntervalMs, keyOnly: true, paused: true }
];

const keyMetricPrefixes = [
  'soc',
  'params.soc',
  'param.soc',
  'params.f32',
  'params.inLvMpptPwr',
  'params.inHvMpptPwr',
  'param.powGetPv',
  'params.pv1ChargeWatts',
  'params.pv2ChargeWatts',
  'params.chgSunPower',
  'acW',
  'dcW',
  'loadW',
  'batteryW',
  'params.wattsInSum',
  'param.wattsInSum',
  'params.wattsOutSum',
  'param.wattsOutSum',
  'params.invInWatts',
  'params.invOutWatts',
  'params.outAc',
  'params.outUsb',
  'params.outTypec',
  'params.outPrPwr',
  'params.outAdsPwr',
  'params.carWatts',
  'params.wireWatts',
  'params.usb',
  'params.typec',
  'params.qcUsb',
  'params.batAmp',
  'params.batVol',
  'params.bmsInputWatts',
  'params.bmsOutputWatts'
];

export class DeliveryLane {
  private readonly deviceId: string;
  private readonly socket: { bufferedAmount?: number };
  private readonly config: DeliveryConfig;
  private readonly emitTelemetry: (message: ServerTelemetryMessage) => void;
  private readonly emitStatus: (message: ServerDeviceStatusMessage) => void;
  private readonly onStageChange?: (stage: DeliveryStage) => void;
  private rawMetrics: RawTelemetryMetrics = {};
  private pendingDirty = false;
  private pendingCoreDirty = false;
  private pendingTs = 0;
  private coalescedSinceFlush = 0;
  private quietTicks = 0;
  private stageIndex = 0;
  private interval: ReturnType<typeof setInterval> | null = null;
  private readonly stageDescriptors: StageDescriptor[];

  constructor(input: {
    deviceId: string;
    socket: { bufferedAmount?: number };
    config: DeliveryConfig;
    emitTelemetry: (message: ServerTelemetryMessage) => void;
    emitStatus: (message: ServerDeviceStatusMessage) => void;
    onStageChange?: (stage: DeliveryStage) => void;
  }) {
    this.deviceId = input.deviceId;
    this.socket = input.socket;
    this.config = input.config;
    this.emitTelemetry = input.emitTelemetry;
    this.emitStatus = input.emitStatus;
    this.onStageChange = input.onStageChange;
    this.stageDescriptors = stages(this.config);
    this.ensureInterval();
  }

  close(): void {
    if (this.interval) {
      clearInterval(this.interval);
      this.interval = null;
    }
  }

  applySnapshot(ts: number, rawMetrics: RawTelemetryMetrics): void {
    this.rawMetrics = { ...rawMetrics };
    this.pendingDirty = false;
    this.pendingCoreDirty = false;
    this.pendingTs = ts;
    this.coalescedSinceFlush = 0;
    this.quietTicks = 0;
    this.setStage(0);
    this.emitStatus({ type: 'device_status', deviceId: this.deviceId, ts, online: true });
    this.emitTelemetryNow(ts);
  }

  applyDelta(ts: number, changed: Record<string, number>, cleared: string[]): void {
    this.rawMetrics = mergeRawMetrics(this.rawMetrics, changed, cleared);
    this.pendingTs = ts;
    if (this.pendingDirty) {
      this.coalescedSinceFlush += 1;
    }
    this.pendingDirty = true;
    if (touchesKeyMetrics(changed, cleared)) {
      this.pendingCoreDirty = true;
    }
    if (this.isPressured()) {
      this.promoteStage();
    }
  }

  applyHeartbeat(ts: number): void {
    this.emitStatus({ type: 'device_status', deviceId: this.deviceId, ts, online: true });
  }

  private ensureInterval(): void {
    if (this.interval) {
      clearInterval(this.interval);
    }
    const descriptor = this.currentStage();
    this.interval = setInterval(() => {
      this.flushTick();
    }, descriptor.intervalMs);
  }

  private flushTick(): void {
    const descriptor = this.currentStage();

    if (descriptor.paused) {
      this.coalescedSinceFlush = 0;
      if (!this.isPressured()) {
        this.quietTicks += 1;
        if (this.quietTicks >= this.config.quietTicksToRecover) {
          this.demoteStage();
        }
      } else {
        this.quietTicks = 0;
      }
      return;
    }

    if (!this.pendingDirty) {
      this.quietTicks += 1;
      if (this.quietTicks >= this.config.quietTicksToRecover) {
        this.demoteStage();
      }
      return;
    }

    if (descriptor.keyOnly && !this.pendingCoreDirty) {
      this.pendingDirty = false;
      this.coalescedSinceFlush = 0;
      this.quietTicks += 1;
      if (this.quietTicks >= this.config.quietTicksToRecover) {
        this.demoteStage();
      }
      return;
    }

    this.emitTelemetryNow(this.pendingTs);
    this.pendingDirty = false;
    this.pendingCoreDirty = false;
    if (this.coalescedSinceFlush >= this.config.highWatermark || this.isPressured()) {
      this.promoteStage();
      this.quietTicks = 0;
    } else {
      this.quietTicks += 1;
      if (this.quietTicks >= this.config.quietTicksToRecover) {
        this.demoteStage();
      }
    }
    this.coalescedSinceFlush = 0;
  }

  private emitTelemetryNow(ts: number): void {
    this.emitTelemetry({
      type: 'telemetry',
      deviceId: this.deviceId,
      ts,
      metrics: deriveTelemetryMetrics(this.rawMetrics),
      detail: deriveTelemetryDetail(this.rawMetrics)
    });
  }

  private isPressured(): boolean {
    return (this.socket.bufferedAmount ?? 0) >= this.config.bufferedAmountHighWaterBytes;
  }

  private promoteStage(): void {
    const maxIndex = this.stageDescriptors.length - 1;
    this.setStage(Math.min(maxIndex, this.stageIndex + 1));
  }

  private demoteStage(): void {
    this.setStage(Math.max(0, this.stageIndex - 1));
    this.quietTicks = 0;
  }

  private setStage(index: number): void {
    if (index === this.stageIndex) {
      return;
    }
    this.stageIndex = index;
    this.ensureInterval();
    const descriptor = this.currentStage();
    this.onStageChange?.(descriptor.name);
  }

  private currentStage(): StageDescriptor {
    return this.stageDescriptors[this.stageIndex] ?? this.stageDescriptors[0]!;
  }
}

function touchesKeyMetrics(changed: Record<string, number>, cleared: string[]): boolean {
  return [...Object.keys(changed), ...cleared].some((key) => keyMetricPrefixes.some((prefix) => key.startsWith(prefix)));
}

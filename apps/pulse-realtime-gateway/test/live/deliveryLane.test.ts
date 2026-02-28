import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DeliveryLane, type DeliveryStage } from '../../src/live/deliveryLane.js';

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('DeliveryLane', () => {
  it('emits snapshot immediately and coalesces deltas', () => {
    const telemetry = vi.fn();
    const status = vi.fn();
    const lane = new DeliveryLane({
      deviceId: 'dev-1',
      socket: { bufferedAmount: 0 },
      config: {
        fastIntervalMs: 250,
        steadyIntervalMs: 500,
        slowIntervalMs: 1000,
        highWatermark: 3,
        bufferedAmountHighWaterBytes: 4096,
        quietTicksToRecover: 2
      },
      emitTelemetry: telemetry,
      emitStatus: status
    });

    lane.applySnapshot(1000, { 'params.soc': 25 });
    expect(status).toHaveBeenCalledTimes(1);
    expect(telemetry).toHaveBeenCalledTimes(1);

    lane.applyDelta(1100, { 'params.wattsInSum': 120 }, []);
    lane.applyDelta(1150, { 'params.pv1ChargeWatts': 30 }, []);
    vi.advanceTimersByTime(249);
    expect(telemetry).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(telemetry).toHaveBeenCalledTimes(2);

    lane.close();
  });

  it('promotes under pressure and recovers when quiet', () => {
    const stages: DeliveryStage[] = [];
    const lane = new DeliveryLane({
      deviceId: 'dev-1',
      socket: { bufferedAmount: 0 },
      config: {
        fastIntervalMs: 250,
        steadyIntervalMs: 500,
        slowIntervalMs: 1000,
        highWatermark: 1,
        bufferedAmountHighWaterBytes: 1,
        quietTicksToRecover: 1
      },
      emitTelemetry: vi.fn(),
      emitStatus: vi.fn(),
      onStageChange(stage) {
        stages.push(stage);
      }
    });

    lane.applySnapshot(1000, { 'params.soc': 25 });
    lane.applyDelta(1100, { 'params.wattsInSum': 120 }, []);
    lane.applyDelta(1150, { 'params.pv1ChargeWatts': 30 }, []);
    vi.advanceTimersByTime(250);
    expect(stages).toContain('steady');

    vi.advanceTimersByTime(500);
    expect(stages.at(-1)).toBe('fast');
    lane.close();
  });

  it('suppresses non-core deltas while in key-only mode', () => {
    const telemetry = vi.fn();
    const stages: DeliveryStage[] = [];
    const lane = new DeliveryLane({
      deviceId: 'dev-1',
      socket: { bufferedAmount: 0 },
      config: {
        fastIntervalMs: 250,
        steadyIntervalMs: 500,
        slowIntervalMs: 1000,
        highWatermark: 1,
        bufferedAmountHighWaterBytes: 4096,
        quietTicksToRecover: 2
      },
      emitTelemetry: telemetry,
      emitStatus: vi.fn(),
      onStageChange(stage) {
        stages.push(stage);
      }
    });

    lane.applySnapshot(1000, { 'params.soc': 25 });
    lane.applyDelta(1100, { 'params.wattsInSum': 120 }, []);
    lane.applyDelta(1150, { 'params.wattsInSum': 120.5 }, []);
    vi.advanceTimersByTime(250);
    lane.applyDelta(1200, { 'params.wattsInSum': 121 }, []);
    lane.applyDelta(1250, { 'params.wattsInSum': 121.5 }, []);
    vi.advanceTimersByTime(500);
    lane.applyDelta(1300, { 'params.wattsInSum': 122 }, []);
    lane.applyDelta(1350, { 'params.wattsInSum': 122.5 }, []);
    vi.advanceTimersByTime(1000);
    expect(stages.at(-1)).toBe('key-only');

    lane.applyDelta(1400, { 'params.wifiRssi': -41 }, []);
    vi.advanceTimersByTime(1000);
    expect(telemetry).toHaveBeenCalledTimes(4);
    expect(stages.at(-1)).toBe('key-only');

    lane.applyDelta(1500, { 'params.wattsInSum': 150 }, []);
    vi.advanceTimersByTime(1000);
    expect(telemetry).toHaveBeenCalledTimes(5);

    lane.close();
  });

  it('pauses delivery under sustained pressure and recovers after quiet ticks', () => {
    const telemetry = vi.fn();
    const stages: DeliveryStage[] = [];
    const lane = new DeliveryLane({
      deviceId: 'dev-1',
      socket: { bufferedAmount: 0 },
      config: {
        fastIntervalMs: 250,
        steadyIntervalMs: 500,
        slowIntervalMs: 1000,
        highWatermark: 1,
        bufferedAmountHighWaterBytes: 4096,
        quietTicksToRecover: 1
      },
      emitTelemetry: telemetry,
      emitStatus: vi.fn(),
      onStageChange(stage) {
        stages.push(stage);
      }
    });

    lane.applySnapshot(1000, { 'params.soc': 25 });
    lane.applyDelta(1100, { 'params.wattsInSum': 120 }, []);
    lane.applyDelta(1150, { 'params.wattsInSum': 120.5 }, []);
    vi.advanceTimersByTime(250);
    lane.applyDelta(1200, { 'params.wattsInSum': 121 }, []);
    lane.applyDelta(1250, { 'params.wattsInSum': 121.5 }, []);
    vi.advanceTimersByTime(500);
    lane.applyDelta(1300, { 'params.wattsInSum': 122 }, []);
    lane.applyDelta(1350, { 'params.wattsInSum': 122.5 }, []);
    vi.advanceTimersByTime(1000);
    lane.applyDelta(1400, { 'params.wattsInSum': 123 }, []);
    lane.applyDelta(1450, { 'params.wattsInSum': 123.5 }, []);
    vi.advanceTimersByTime(1000);
    expect(stages.at(-1)).toBe('paused');

    lane.applyDelta(1500, { 'params.wattsInSum': 160 }, []);
    vi.advanceTimersByTime(1000);
    expect(telemetry).toHaveBeenCalledTimes(5);

    vi.advanceTimersByTime(1000);
    vi.advanceTimersByTime(1000);
    vi.advanceTimersByTime(1000);
    vi.advanceTimersByTime(500);
    expect(stages).toContain('paused');
    expect(stages.at(-1)).toBe('fast');

    lane.close();
  });
});

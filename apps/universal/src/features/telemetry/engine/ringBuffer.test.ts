import { describe, expect, it } from 'vitest';
import { RingBuffer } from '@/features/telemetry/engine/ringBuffer';

describe('RingBuffer', () => {
  it('keeps only fixed capacity and overwrites oldest points', () => {
    const buffer = new RingBuffer(3);
    buffer.push({ ts: 1, value: 10 });
    buffer.push({ ts: 2, value: 20 });
    buffer.push({ ts: 3, value: 30 });
    buffer.push({ ts: 4, value: 40 });

    expect(buffer.size()).toBe(3);
    expect(buffer.toArray()).toEqual([
      { ts: 2, value: 20 },
      { ts: 3, value: 30 },
      { ts: 4, value: 40 }
    ]);
  });

  it('downsamples to requested point count', () => {
    const buffer = new RingBuffer(10);
    for (let i = 0; i < 10; i += 1) {
      buffer.push({ ts: i, value: i });
    }

    const down = buffer.downsample(5);
    expect(down).toHaveLength(5);
    expect(down[0]?.value).toBe(0);
    expect(down[4]?.value).toBe(9);
  });
});

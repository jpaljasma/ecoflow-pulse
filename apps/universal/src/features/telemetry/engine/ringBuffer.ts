export type TimePoint = {
  ts: number;
  value: number;
};

export class RingBuffer {
  private readonly data: TimePoint[];
  private index = 0;
  private count = 0;

  constructor(private readonly capacity: number) {
    this.data = new Array(capacity);
  }

  push(point: TimePoint): void {
    this.data[this.index] = point;
    this.index = (this.index + 1) % this.capacity;
    this.count = Math.min(this.count + 1, this.capacity);
  }

  size(): number {
    return this.count;
  }

  toArray(): TimePoint[] {
    if (this.count === 0) {
      return [];
    }

    const start = (this.index - this.count + this.capacity) % this.capacity;
    const out: TimePoint[] = new Array(this.count);
    for (let i = 0; i < this.count; i += 1) {
      out[i] = this.data[(start + i) % this.capacity] as TimePoint;
    }
    return out;
  }

  downsample(targetPoints: number): TimePoint[] {
    const arr = this.toArray();
    if (arr.length <= targetPoints) {
      return arr;
    }

    if (targetPoints <= 1) {
      return [arr[arr.length - 1] as TimePoint];
    }

    const step = (arr.length - 1) / (targetPoints - 1);
    const sampled: TimePoint[] = [];

    for (let i = 0; i < targetPoints; i += 1) {
      const idx = Math.round(i * step);
      sampled.push(arr[idx] as TimePoint);
    }

    return sampled;
  }
}

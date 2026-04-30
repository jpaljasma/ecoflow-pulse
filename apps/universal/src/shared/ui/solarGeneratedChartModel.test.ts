import { describe, expect, it } from 'vitest';
import {
  buildSvgStepPath,
  normalizeSolarBucketSeries
} from '@/shared/ui/solarGeneratedChartModel';

describe('solar generated chart model', () => {
  it('builds square step paths without smoothing curves', () => {
    const path = buildSvgStepPath([
      { x: 0, y: 10 },
      { x: 10, y: 4 },
      { x: 20, y: 8 }
    ]);

    expect(path).toBe('M 0.00 10.00 H 10.00 V 4.00 H 20.00 V 8.00');
    expect(path).not.toContain(' C ');
  });

  it('pads shorter time-of-day solar series at the end so bucket labels stay anchored', () => {
    const series = normalizeSolarBucketSeries([4, 8, 12], 5);

    expect(series).toEqual([4, 8, 12, 0, 0]);
  });
});

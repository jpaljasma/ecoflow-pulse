import { describe, expect, it, vi } from 'vitest';
import { SolarHistoryViewSchema } from '@/features/history/api';

const restClientMock = vi.hoisted(() => ({
  requestJson: vi.fn()
}));

vi.mock('@/shared/api/restClient', () => restClientMock);

const solarHistoryPayload = {
  todayWh: 41,
  yesterdayWh: 8550,
  yesterdayRunningWh: 120,
  deltaPct: -65.8,
  seriesWh: [41],
  yesterdaySeriesWh: [120, 300, 450]
};

describe('solar history API contract', () => {
  it('requires the backend-computed yesterday running total', () => {
    const payloadWithoutRunningTotal: Partial<typeof solarHistoryPayload> = { ...solarHistoryPayload };
    delete payloadWithoutRunningTotal.yesterdayRunningWh;

    expect(() => SolarHistoryViewSchema.parse(payloadWithoutRunningTotal)).toThrow();
  });

  it('accepts solar history payloads with yesterdayRunningWh', () => {
    expect(SolarHistoryViewSchema.parse(solarHistoryPayload).yesterdayRunningWh).toBe(120);
  });
});

import { describe, expect, it, vi } from 'vitest';
import { fetchEnergyCalendar } from '@/features/energy/api';

const restClientMock = vi.hoisted(() => ({
  requestJson: vi.fn()
}));

vi.mock('@/shared/api/restClient', () => restClientMock);

describe('energy calendar api parsing', () => {
  it('parses visible days and selected month totals from the calendar endpoint', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      scope: {
        mode: 'device',
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        resolvedDeviceIds: ['019c9f0e-4521-775d-873e-e80039f16d75']
      },
      year: 2026,
      month: 5,
      timezone: 'America/New_York',
      selectedMonth: {
        year: 2026,
        month: 5,
        totals: {
          solarGeneratedKwh: 42.5,
          estimatedValue: 13.7,
          currency: 'USD'
        }
      },
      visibleDays: [
        {
          dateIso: '2026-05-09',
          year: 2026,
          month: 5,
          day: 9,
          solarGeneratedKwh: 2.5,
          estimatedValue: 0.81,
          currency: 'USD',
          isCurrentMonth: true,
          hasData: true,
          isToday: false,
          isFuture: false
        }
      ]
    });

    const result = await fetchEnergyCalendar({
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
      year: 2026,
      month: 5,
      timezone: 'America/New_York',
      gridPricePerKwh: 0.24,
      currency: 'USD',
      token: 'token-123'
    });

    expect(restClientMock.requestJson).toHaveBeenCalledWith(
      '/api/v1/energy/calendar?scope=device&deviceId=019c9f0e-4521-775d-873e-e80039f16d75&year=2026&month=5&timezone=America%2FNew_York&gridPricePerKwh=0.24&currency=USD',
      { token: 'token-123' }
    );
    expect(result.selectedMonth.totals.solarGeneratedKwh).toBe(42.5);
    expect(result.visibleDays).toHaveLength(1);
    expect(result.visibleDays[0]?.dateIso).toBe('2026-05-09');
  });
});

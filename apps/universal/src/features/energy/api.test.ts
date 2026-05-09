import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  fetchEnergyCalendar,
  fetchEnergyComparisonInsight,
  fetchEnergyDashboard,
  fetchEnergyPvPortHistory
} from '@/features/energy/api';

const restClientMock = vi.hoisted(() => ({
  requestJson: vi.fn()
}));

vi.mock('@/shared/api/restClient', () => restClientMock);

describe('energy calendar api parsing', () => {
  beforeEach(() => {
    restClientMock.requestJson.mockReset();
  });

  it('does not send timezone in energy dashboard requests', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({});

    await expect(
      fetchEnergyDashboard({
        scope: 'all',
        preset: 'today',
        includeComparison: true,
        date: '2026-05-09',
        token: 'token-123'
      })
    ).rejects.toThrow();

    expect(restClientMock.requestJson).toHaveBeenCalledWith(
      '/api/v1/energy/dashboard?scope=all&preset=today&includeComparison=true&date=2026-05-09',
      { token: 'token-123' }
    );
  });

  it('does not send timezone in auxiliary energy requests', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({ pvPortHistory: [] });
    restClientMock.requestJson.mockResolvedValueOnce({ status: 'pending', statusDetail: 'warming' });

    await fetchEnergyPvPortHistory({
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
      preset: 'today',
      date: '2026-05-09',
      token: 'token-123'
    });
    await fetchEnergyComparisonInsight({
      scope: 'all',
      preset: 'today',
      date: '2026-05-09',
      gridPricePerKwh: 0.24,
      currency: 'USD',
      token: 'token-123'
    });

    expect(restClientMock.requestJson).toHaveBeenNthCalledWith(
      1,
      '/api/v1/energy/pv-history?scope=device&preset=today&deviceId=019c9f0e-4521-775d-873e-e80039f16d75&date=2026-05-09',
      { token: 'token-123' }
    );
    expect(restClientMock.requestJson).toHaveBeenNthCalledWith(
      2,
      '/api/v1/energy/comparison-insight?scope=all&preset=today&date=2026-05-09&gridPricePerKwh=0.24&currency=USD',
      { token: 'token-123' }
    );
  });

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
      gridPricePerKwh: 0.24,
      currency: 'USD',
      token: 'token-123'
    });

    expect(restClientMock.requestJson).toHaveBeenCalledWith(
      '/api/v1/energy/calendar?scope=device&deviceId=019c9f0e-4521-775d-873e-e80039f16d75&year=2026&month=5&gridPricePerKwh=0.24&currency=USD',
      { token: 'token-123' }
    );
    expect(result.selectedMonth.totals.solarGeneratedKwh).toBe(42.5);
    expect(result.visibleDays).toHaveLength(1);
    expect(result.visibleDays[0]?.dateIso).toBe('2026-05-09');
  });
});

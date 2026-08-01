import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { fetchOrderDailyStats } from '../orders';

const requestMock = vi.mocked(request);

describe('fetchOrderDailyStats', () => {
  it('requests the daily stats endpoint with default 30 days', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { generatedAt: '2026-08-01T00:00:00Z', days: 30, items: [] },
    });

    await expect(fetchOrderDailyStats()).resolves.toEqual({
      generatedAt: '2026-08-01T00:00:00Z',
      days: 30,
      items: [],
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/orders/stats/daily?days=30', { method: 'GET' });
  });

  it('passes a custom days window', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { generatedAt: '', days: 7, items: [] } });

    await fetchOrderDailyStats(7);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/orders/stats/daily?days=7', { method: 'GET' });
  });
});

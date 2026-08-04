import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { fetchInventoryReport, fetchProcurementReport, fetchProfitReport } from '../reports';

const requestMock = vi.mocked(request);

describe('deep report services', () => {
  it('requests the profit report with dimension and days', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { rows: [] } });

    await fetchProfitReport('product', { days: 7 });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/reports/profit?dimension=product&days=7', {
      method: 'GET',
    });
  });

  it('prefers a custom range over days for the profit report', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { rows: [] } });

    await fetchProfitReport('order', { days: 30, start: '2026-07-01', end: '2026-07-31' });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/reports/profit?dimension=order&start=2026-07-01&end=2026-07-31',
      { method: 'GET' },
    );
  });

  it('requests the procurement report without params by default', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { daily: [] } });

    await fetchProcurementReport({});

    expect(requestMock).toHaveBeenCalledWith('/api/v1/reports/procurement', { method: 'GET' });
  });

  it('passes slowDays to the inventory report', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { slowMoving: [] } });

    await fetchInventoryReport(60);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/reports/inventory?slowDays=60', { method: 'GET' });
  });
});

import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { fetchReportCurrencySettings, saveReportCurrencySettings } from '../settings';

const requestMock = vi.mocked(request);

describe('report currency settings service', () => {
  it('fetches the report currency configuration', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { provider: 'manual', baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] },
    });

    await expect(fetchReportCurrencySettings()).resolves.toEqual({
      provider: 'manual',
      baseCurrency: 'CNY',
      rates: [{ currency: 'USD', rate: '7.13' }],
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/settings/report-currency', { method: 'GET' });
  });

  it('saves base currency and manual rates via PUT', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { provider: 'manual', baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] },
    });

    await saveReportCurrencySettings({ baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/settings/report-currency', {
      method: 'PUT',
      data: { baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] },
    });
  });
});

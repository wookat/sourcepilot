import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  fetchCandidateInsights,
  fetchCandidatePriceTrend,
  fetchMarketSources,
  fetchSelectionCompare,
} from '../selection';

const requestMock = vi.mocked(request);

describe('selection insights services', () => {
  it('requests candidate insights by id', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { collected: {} } });

    await fetchCandidateInsights('cand-1');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/selection/candidates/cand-1/insights', {
      method: 'GET',
    });
  });

  it('requests the candidate price trend by id', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { points: [] } });

    await fetchCandidatePriceTrend('cand-1');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/selection/candidates/cand-1/price-trend', {
      method: 'GET',
    });
  });

  it('joins candidate ids into the compare query', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: [] });

    await fetchSelectionCompare(['a', 'b', 'c']);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/selection/compare', {
      method: 'GET',
      params: { ids: 'a,b,c' },
    });
  });

  it('requests external market source status', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: [] });

    await fetchMarketSources();

    expect(requestMock).toHaveBeenCalledWith('/api/v1/selection/market-sources', { method: 'GET' });
  });
});

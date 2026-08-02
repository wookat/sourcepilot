import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { getJSON, getWithParams, postJSON } from '../request';

const requestMock = vi.mocked(request);

describe('request helpers', () => {
  it('unwraps successful GET envelopes', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'p1' } });

    await expect(getJSON('/api/v1/products/p1')).resolves.toEqual({ id: 'p1' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1', { method: 'GET' });
  });

  it('sends POST data through the backend envelope', async () => {
    const body = { title: '测试商品' };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'created' } });

    await expect(postJSON('/api/v1/products', body)).resolves.toEqual({ id: 'created' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products', { method: 'POST', data: body });
  });

  it('passes query params without dropping undefined boundary keys', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    await getWithParams('/api/v1/products/p1/readiness', { platform: 'douyin_shop', shopId: undefined, mode: 'draft' });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1/readiness', {
      method: 'GET',
      params: { platform: 'douyin_shop', shopId: undefined, mode: 'draft' },
    });
  });

  it('throws backend business errors with message fallback', async () => {
    requestMock.mockResolvedValueOnce({ code: 40001, message: '商品不存在', data: null });

    await expect(getJSON('/api/v1/products/missing')).rejects.toThrow('商品不存在');
  });

  it('surfaces backend envelope message on non-2xx HTTP errors', async () => {
    const axiosLikeError = Object.assign(new Error('Request failed with status code 400'), {
      response: {
        status: 400,
        data: { code: 40001, message: '请配置 base_url', data: null },
      },
    });
    requestMock.mockRejectedValueOnce(axiosLikeError);

    await expect(
      postJSON('/api/v1/products/p1/ai/optimize-title', {}),
    ).rejects.toThrow('请配置 base_url');
  });

  it('rethrows non-envelope HTTP errors unchanged', async () => {
    const networkError = new Error('Network Error');
    requestMock.mockRejectedValueOnce(networkError);

    await expect(getJSON('/api/v1/products')).rejects.toThrow('Network Error');
  });
});

import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  CODE_COLLECTOR_UNREACHABLE,
  getJSON,
  getWithParams,
  isCollectorUnreachableError,
  postJSON,
} from '../request';

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

  it('surfaces the backend Chinese message on non-2xx axios errors', async () => {
    const axiosErr = Object.assign(new Error('Request failed with status code 400'), {
      response: { status: 400, data: { code: 40001, message: '接口密钥无效，请检查后重试', data: null } },
    });
    requestMock.mockRejectedValueOnce(axiosErr);

    await expect(postJSON('/api/v1/settings/test-ai', {})).rejects.toThrow('接口密钥无效，请检查后重试');
  });

  it('recognizes collector-unreachable envelope errors (502 环境项)', async () => {
    const axiosErr = Object.assign(new Error('Request failed with status code 502'), {
      response: {
        status: 502,
        data: { code: CODE_COLLECTOR_UNREACHABLE, message: '采集服务未启动或不可达，请先启动采集服务（collector）后重试', data: null },
      },
    });
    requestMock.mockRejectedValueOnce(axiosErr);

    const err = await getJSON('/api/collector/providers/1688/auth-status').catch((e) => e);
    expect(isCollectorUnreachableError(err)).toBe(true);
    expect((err as Error).message).toContain('采集服务未启动');
  });

  it('does not flag other errors as collector unreachable', async () => {
    requestMock.mockResolvedValueOnce({ code: 40001, message: '商品不存在', data: null });
    const err = await getJSON('/api/v1/products/missing').catch((e) => e);
    expect(isCollectorUnreachableError(err)).toBe(false);
    expect(isCollectorUnreachableError(new Error('Network Error'))).toBe(false);
  });

  it('rethrows non-envelope transport errors unchanged', async () => {
    const netErr = new Error('Network Error');
    requestMock.mockRejectedValueOnce(netErr);

    await expect(getJSON('/api/v1/settings')).rejects.toBe(netErr);
  });
});

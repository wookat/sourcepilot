import { describe, expect, it } from 'vitest';
import { httpErrorCopy } from '../errorMessages';

describe('httpErrorCopy', () => {
  it('maps known raw backend messages to Chinese copy', () => {
    expect(httpErrorCopy(new Error('conflict: supplier has bound sources'), '删除失败')).toBe(
      '该供应商已绑定商品货源，请先解绑货源后再删除',
    );
    expect(httpErrorCopy(new Error('record not found'), '删除失败')).toBe('记录不存在或已被删除');
  });

  it('keeps already-localized Chinese messages as-is', () => {
    expect(httpErrorCopy(new Error('该供应商不存在'), '删除失败')).toBe('该供应商不存在');
  });

  it('maps uppercase error codes via error code map', () => {
    expect(httpErrorCopy(new Error('DOUYIN_STORE_NOT_AUTHORIZED'), '操作失败')).toBe(
      '店铺尚未授权：请先在店铺管理中完成抖店授权。',
    );
  });

  it('prefers the response envelope message on HTTP errors', () => {
    const axiosLike = Object.assign(new Error('Request failed with status code 409'), {
      response: { data: { code: 409, message: 'conflict: supplier has bound sources' } },
    });
    expect(httpErrorCopy(axiosLike, '删除失败')).toBe('该供应商已绑定商品货源，请先解绑货源后再删除');
  });

  it('falls back to the provided Chinese copy for unknown English messages', () => {
    expect(httpErrorCopy(new Error('unexpected pq error: relation missing'), '删除失败')).toBe(
      '删除失败',
    );
    expect(httpErrorCopy(undefined, '删除失败')).toBe('删除失败');
    expect(httpErrorCopy(new Error(''), '删除失败')).toBe('删除失败');
  });
});

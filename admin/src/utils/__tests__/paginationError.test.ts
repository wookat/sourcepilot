import { describe, expect, it } from 'vitest';

import {
  isPaginationOffsetTooDeepError,
  PAGINATION_OFFSET_TOO_DEEP,
} from '../paginationError';

describe('isPaginationOffsetTooDeepError', () => {
  it('识别后端 message 为稳定 code 的深分页错误', () => {
    expect(isPaginationOffsetTooDeepError({ message: PAGINATION_OFFSET_TOO_DEEP })).toBe(true);
  });

  it('data.errorCode 命中时同样识别（message 被改写场景）', () => {
    expect(
      isPaginationOffsetTooDeepError({
        message: '请求参数错误',
        data: { errorCode: PAGINATION_OFFSET_TOO_DEEP },
      }),
    ).toBe(true);
  });

  it('其他业务错误与非对象错误不误判', () => {
    expect(isPaginationOffsetTooDeepError({ message: 'bad request', data: null })).toBe(false);
    expect(isPaginationOffsetTooDeepError(PAGINATION_OFFSET_TOO_DEEP)).toBe(false);
    expect(isPaginationOffsetTooDeepError(undefined)).toBe(false);
  });
});

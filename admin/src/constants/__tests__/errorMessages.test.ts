import { describe, expect, it } from 'vitest';
import {
  formatRequestError,
  formatUserErrorMessage,
  httpErrorCopy,
  mapErrorCodeToUserMessage,
} from '../errorMessages';

describe('恢复安全门错误码映射（R58）', () => {
  it.each([
    ['RESTORE_TARGET_FORBIDDEN', '禁止恢复到生产环境'],
    ['RESTORE_APP_ENV_FORBIDDEN', '生产环境禁止恢复演练'],
    ['RESTORE_VERIFY_APP_ENV_FORBIDDEN', '生产环境禁止恢复验证'],
    ['RESTORE_TARGET_NOT_ISOLATED', '目标环境未确认隔离'],
    ['RESTORE_TARGET_NOT_EXPLICIT', '目标数据库未明确指定'],
    ['RESTORE_TARGET_PREFIX_REJECTED', '目标数据库名前缀不符合要求'],
    ['RESTORE_CONFIRMATION_REQUIRED', '缺少高风险操作确认'],
    ['RESTORE_BACKUP_NOT_VERIFIED', '备份尚未通过校验'],
    ['RESTORE_TARGET_NOT_EMPTY', '目标数据库不是空库'],
  ])('%s 映射为「%s」', (code, title) => {
    expect(mapErrorCodeToUserMessage(code)?.title).toBe(title);
  });

  it('带英文详情的结构化 message 也能按前缀映射为中文', () => {
    const msg = formatUserErrorMessage(
      'RESTORE_APP_ENV_FORBIDDEN: P6-V restore drill is forbidden in production',
      '创建恢复验证失败',
    );
    expect(msg).toContain('生产环境禁止恢复演练');
    expect(msg).toContain('APP_ENV=production');
  });

  it('formatRequestError 透传后端安全门拒绝的具体原因', () => {
    const e = {
      response: {
        status: 400,
        data: {
          message:
            'RESTORE_TARGET_PREFIX_REJECTED: target database must use trademind_p6v_restore_ prefix',
        },
      },
    };
    const msg = formatRequestError(e, '创建恢复验证失败');
    expect(msg).toContain('目标数据库名前缀不符合要求');
    expect(msg).toContain('trademind_p6v_restore_');
  });

  it('未知错误仍回退到中文兜底', () => {
    expect(formatRequestError({}, '创建恢复验证失败')).toBe('创建恢复验证失败');
  });
});



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

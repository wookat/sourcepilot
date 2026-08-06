/** 与后端 pagination.ErrCodeOffsetTooDeep 对齐：offset 超过深分页保护上限 */
export const PAGINATION_OFFSET_TOO_DEEP = 'pagination_offset_too_deep';

/** 深分页被后端拒绝时展示给用户的中文提示 */
export const PAGINATION_OFFSET_TOO_DEEP_MESSAGE = '页码过深，请缩小筛选范围或降低页码后重试';

/** 判断错误是否为「深分页 offset 超限」（后端 400 envelope 的 message/data.errorCode 均为稳定 code） */
export function isPaginationOffsetTooDeepError(e: unknown): boolean {
  if (!e || typeof e !== 'object') return false;
  const { message, data } = e as { message?: unknown; data?: { errorCode?: unknown } | null };
  if (message === PAGINATION_OFFSET_TOO_DEEP) return true;
  return Boolean(data && typeof data === 'object' && data.errorCode === PAGINATION_OFFSET_TOO_DEEP);
}

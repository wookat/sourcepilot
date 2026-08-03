import { describe, expect, it } from 'vitest';
import { formatDateTime, formatDateTimeShort } from '../formatTime';

describe('formatDateTimeShort（R77 列表短格式）', () => {
  it('省略年份与秒（MM-DD HH:mm）', () => {
    expect(formatDateTimeShort('2026-08-03T06:57:12Z')).toMatch(/^\d{2}-\d{2} \d{2}:\d{2}$/);
  });

  it('空值返回 fallback', () => {
    expect(formatDateTimeShort(undefined)).toBe('—');
    expect(formatDateTimeShort(null)).toBe('—');
    expect(formatDateTimeShort('')).toBe('—');
    expect(formatDateTimeShort('', '')).toBe('');
  });

  it('无效值原样返回（与 formatDateTime 口径一致）', () => {
    expect(formatDateTimeShort('not-a-date')).toBe('not-a-date');
    expect(formatDateTime('not-a-date')).toBe('not-a-date');
  });
});

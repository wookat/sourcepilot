import { afterEach, describe, expect, it } from 'vitest';
import { formatHourTick, readStoredNumber } from '../index';

describe('经营大屏 formatHourTick', () => {
  it('把 ISO 时间格式化为本地整点 HH:00', () => {
    const iso = new Date(2026, 7, 6, 9, 0, 0).toISOString();
    expect(formatHourTick(iso)).toBe('09:00');
  });

  it('非法时间原样返回，不抛错', () => {
    expect(formatHourTick('not-a-date')).toBe('not-a-date');
    expect(formatHourTick(undefined)).toBe('');
  });
});

describe('经营大屏 readStoredNumber', () => {
  afterEach(() => {
    localStorage.clear();
  });

  it('无存储或非法值时回退默认间隔', () => {
    expect(readStoredNumber('tm_test_refresh', 30, [0, 15, 30, 60])).toBe(30);
    localStorage.setItem('tm_test_refresh', 'abc');
    expect(readStoredNumber('tm_test_refresh', 30, [0, 15, 30, 60])).toBe(30);
    localStorage.setItem('tm_test_refresh', '45');
    expect(readStoredNumber('tm_test_refresh', 30, [0, 15, 30, 60])).toBe(30);
  });

  it('存储值在允许列表内时生效（含暂停 0）', () => {
    localStorage.setItem('tm_test_refresh', '15');
    expect(readStoredNumber('tm_test_refresh', 30, [0, 15, 30, 60])).toBe(15);
    localStorage.setItem('tm_test_refresh', '0');
    expect(readStoredNumber('tm_test_refresh', 30, [0, 15, 30, 60])).toBe(0);
  });
});

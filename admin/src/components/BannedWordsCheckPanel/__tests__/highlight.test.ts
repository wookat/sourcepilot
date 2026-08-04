import { describe, expect, it } from 'vitest';
import { highlightSegments } from '../index';

describe('highlightSegments', () => {
  it('splits text into plain and highlighted segments by code point offsets', () => {
    const out = highlightSegments('全网最低价，最佳选择', [
      { start: 0, end: 4, level: 'forbidden' },
      { start: 6, end: 8, level: 'warning' },
    ]);
    expect(out).toEqual([
      { text: '全网最低', level: 'forbidden' },
      { text: '价，' },
      { text: '最佳', level: 'warning' },
      { text: '选择' },
    ]);
  });

  it('merges overlapping ranges without duplicating characters', () => {
    const out = highlightSegments('abcdef', [
      { start: 0, end: 3, level: 'forbidden' },
      { start: 2, end: 5, level: 'warning' },
    ]);
    expect(out.map((s) => s.text).join('')).toBe('abcdef');
  });

  it('ignores out-of-range positions', () => {
    const out = highlightSegments('短文本', [{ start: 10, end: 12, level: 'forbidden' }]);
    expect(out).toEqual([{ text: '短文本' }]);
  });

  it('returns a single plain segment when no ranges', () => {
    expect(highlightSegments('普通文案', [])).toEqual([{ text: '普通文案' }]);
  });
});

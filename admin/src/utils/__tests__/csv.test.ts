import { describe, expect, it } from 'vitest';
import { buildCSV } from '../csv';

describe('buildCSV', () => {
  it('joins rows with CRLF and cells with comma', () => {
    expect(
      buildCSV([
        ['指标', '候选A'],
        ['价格', 12.9],
      ]),
    ).toBe('指标,候选A\r\n价格,12.9');
  });

  it('escapes quotes, commas and newlines', () => {
    expect(buildCSV([['a"b', 'c,d', 'e\nf']])).toBe('"a""b","c,d","e\nf"');
  });

  it('renders null/undefined as empty cells (未采集 stays blank, not 0)', () => {
    expect(buildCSV([[null, undefined, 0]])).toBe(',,0');
  });

  it('neutralizes formula-injection prefixes (=+-@ and tab/CR)', () => {
    expect(buildCSV([['=CMD()', '+SUM(A1)', '@foo', '-bar']])).toBe("'=CMD(),'+SUM(A1),'@foo,'-bar");
    expect(buildCSV([['\thi', '\rhi']])).toBe(`'\thi,"'\rhi"`);
  });

  it('keeps plain negative/positive numbers unescaped', () => {
    expect(buildCSV([['-12.9', '+3', -5]])).toBe('-12.9,+3,-5');
  });
});

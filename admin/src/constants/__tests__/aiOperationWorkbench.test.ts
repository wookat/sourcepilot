import { describe, expect, it } from 'vitest';
import { WORKBENCH_PRIORITY_OPTIONS, workbenchPriorityMeta } from '../aiOperationWorkbench';

describe('工作台优先级色彩层级（R65）', () => {
  it('仅最高档 P0 使用红色', () => {
    const reds = WORKBENCH_PRIORITY_OPTIONS.filter((x) => x.color === 'red');
    expect(reds.map((x) => x.value)).toEqual(['P0']);
  });

  it('未知优先级兜底为中性色', () => {
    expect(workbenchPriorityMeta('P9')).toEqual({ value: 'P9', label: 'P9', color: 'default' });
    expect(workbenchPriorityMeta(undefined).label).toBe('—');
  });
});

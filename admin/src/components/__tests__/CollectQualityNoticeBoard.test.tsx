import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  CollectQualityNoticeBoard,
  collectNoticeSummary,
  sortCollectNoticeItems,
  type CollectNoticeItem,
} from '../CollectQualityNoticeBoard';

const items: CollectNoticeItem[] = [
  { key: 'i1', severity: 'info', title: '来源说明', content: '说明内容' },
  { key: 'w1', severity: 'warning', title: '采集质量提示（2 条）', content: '提示内容' },
  { key: 'e1', severity: 'error', title: '发布前必须处理（1 项）', content: '阻断内容' },
];

describe('sortCollectNoticeItems（R66 提示区优先级排序）', () => {
  it('按 error > warning > info > success 排序', () => {
    expect(sortCollectNoticeItems(items).map((x) => x.key)).toEqual(['e1', 'w1', 'i1']);
  });
});

describe('collectNoticeSummary', () => {
  it('含 error 时总体为 error 并统计各档数量', () => {
    const s = collectNoticeSummary(items);
    expect(s.type).toBe('error');
    expect(s.message).toContain('必须处理 1 项');
    expect(s.message).toContain('建议检查 1 项');
    expect(s.message).toContain('说明 1 项');
  });

  it('仅 warning 时总体为 warning', () => {
    expect(collectNoticeSummary([items[1]]).type).toBe('warning');
  });

  it('仅 info 时总体为 info', () => {
    expect(collectNoticeSummary([items[0]]).type).toBe('info');
  });

  it('无条目时为 success', () => {
    const s = collectNoticeSummary([]);
    expect(s.type).toBe('success');
    expect(s.message).toBe('未返回采集质量问题');
  });
});

describe('CollectQualityNoticeBoard', () => {
  it('渲染单一提示区，阻断与建议条目默认展开，写操作提示不丢失', () => {
    render(<CollectQualityNoticeBoard items={items} />);
    expect(screen.getByText(/采集质量提示：必须处理 1 项/)).toBeInTheDocument();
    expect(screen.getByText('发布前必须处理（1 项）')).toBeInTheDocument();
    expect(screen.getByText('阻断内容')).toBeInTheDocument();
    expect(screen.getByText('提示内容')).toBeInTheDocument();
    expect(screen.getByText('来源说明')).toBeInTheDocument();
  });

  it('空条目时不渲染', () => {
    const { container } = render(<CollectQualityNoticeBoard items={[]} />);
    expect(container.firstChild).toBeNull();
  });
});

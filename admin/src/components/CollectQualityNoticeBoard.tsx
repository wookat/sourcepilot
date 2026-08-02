import { useMemo } from 'react';
import { Alert, Collapse, Tag } from 'antd';
import type { ReactNode } from 'react';

export type CollectNoticeSeverity = 'error' | 'warning' | 'info' | 'success';

export type CollectNoticeItem = {
  key: string;
  severity: CollectNoticeSeverity;
  title: string;
  content?: ReactNode;
};

const SEVERITY_ORDER: CollectNoticeSeverity[] = ['error', 'warning', 'info', 'success'];

const SEVERITY_TAG: Record<CollectNoticeSeverity, { label: string; color: string }> = {
  error: { label: '必须处理', color: 'error' },
  warning: { label: '建议检查', color: 'warning' },
  info: { label: '说明', color: 'default' },
  success: { label: '通过', color: 'success' },
};

export function sortCollectNoticeItems(items: CollectNoticeItem[]): CollectNoticeItem[] {
  return [...items].sort(
    (a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity),
  );
}

export function collectNoticeSummary(items: CollectNoticeItem[]): {
  type: CollectNoticeSeverity;
  message: string;
} {
  const errorCount = items.filter((x) => x.severity === 'error').length;
  const warningCount = items.filter((x) => x.severity === 'warning').length;
  const infoCount = items.filter((x) => x.severity === 'info').length;
  if (errorCount > 0 || warningCount > 0) {
    const parts: string[] = [];
    if (errorCount > 0) parts.push(`必须处理 ${errorCount} 项`);
    if (warningCount > 0) parts.push(`建议检查 ${warningCount} 项`);
    if (infoCount > 0) parts.push(`说明 ${infoCount} 项`);
    return { type: errorCount > 0 ? 'error' : 'warning', message: `采集质量提示：${parts.join('，')}` };
  }
  if (infoCount > 0) {
    return { type: 'info', message: `采集质量提示：说明 ${infoCount} 项` };
  }
  return { type: 'success', message: '未返回采集质量问题' };
}

/**
 * Consolidates collect-quality notices into a single alert with a
 * severity-sorted collapsible list; blocking items stay expanded.
 */
export function CollectQualityNoticeBoard({ items }: { items: CollectNoticeItem[] }) {
  const sorted = useMemo(() => sortCollectNoticeItems(items), [items]);
  const summary = useMemo(() => collectNoticeSummary(sorted), [sorted]);

  if (sorted.length === 0) return null;

  return (
    <Alert
      className="collect-quality-notice-board"
      type={summary.type}
      showIcon
      message={summary.message}
      description={
        <Collapse
          ghost
          size="small"
          className="collect-quality-notice-board__list"
          defaultActiveKey={sorted
            .filter((x) => x.severity === 'error' || x.severity === 'warning')
            .map((x) => x.key)}
          items={sorted.map((x) => ({
            key: x.key,
            label: (
              <span className="collect-quality-notice-board__label">
                <Tag color={SEVERITY_TAG[x.severity].color} style={{ marginInlineEnd: 8 }}>
                  {SEVERITY_TAG[x.severity].label}
                </Tag>
                {x.title}
              </span>
            ),
            children: x.content ?? null,
            showArrow: !!x.content,
            collapsible: x.content ? undefined : ('disabled' as const),
          }))}
        />
      }
    />
  );
}

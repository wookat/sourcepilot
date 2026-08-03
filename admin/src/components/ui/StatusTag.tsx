import { forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import { Tag } from 'antd';
import { commonStatusLabel } from '@/constants/copywriting';
import { COLLECT_TASK_STATUS } from '@/constants/status';

type StatusColor = 'default' | 'processing' | 'success' | 'error' | 'warning' | 'blue' | 'cyan';

export type StatusTagProps = {
  status?: string | null;
  /** 直接指定文案，优先于 status 映射 */
  text?: string;
  color?: StatusColor;
  className?: string;
} & Omit<HTMLAttributes<HTMLSpanElement>, 'color'>;

const STATUS_COLOR_MAP: Record<string, StatusColor> = {
  pending: 'processing',
  running: 'processing',
  success: 'success',
  partial_success: 'warning',
  failed: 'error',
  cancelled: 'default',
  enabled: 'success',
  disabled: 'default',
  configured: 'success',
  unconfigured: 'default',
  authorized: 'success',
  expired: 'warning',
  need_check: 'warning',
  bound: 'success',
  unmatched: 'default',
  ambiguous: 'warning',
  skipped: 'default',
  partial: 'warning',
  matched: 'success',
  manual_bound: 'processing',
  completed: 'success',
  passed: 'success',
  manual_review: 'warning',
  queued: 'processing',
  succeeded: 'success',
  verified: 'success',
  deferred: 'default',
  ready: 'success',
  not_ready: 'error',
  ready_with_warning: 'warning',
  active: 'success',
  revoked: 'default',
};

/** 统一状态 Tag；转发 ref 与 DOM 属性，可被 Tooltip 等组件直接包裹 */
const StatusTag = forwardRef<HTMLElement, StatusTagProps>(function StatusTag(
  { status, text, color, className, ...rest },
  ref,
) {
  const k = (status ?? '').trim().toLowerCase();
  const collectMeta = k ? COLLECT_TASK_STATUS[k as keyof typeof COLLECT_TASK_STATUS] : undefined;
  const label = text ?? collectMeta?.text ?? commonStatusLabel(status);
  const tagColor = color ?? collectMeta?.color ?? STATUS_COLOR_MAP[k] ?? 'default';
  return (
    <Tag ref={ref} color={tagColor as never} className={className} {...rest}>
      {label}
    </Tag>
  );
});

export default StatusTag;

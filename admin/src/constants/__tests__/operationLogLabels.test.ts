import { describe, expect, it } from 'vitest';

import {
  OPERATION_LOG_ACTION_LABEL,
  OPERATION_LOG_RESOURCE_LABEL,
  operationLogActionLabel,
  operationLogResourceLabel,
} from '../operationLogs';

describe('operation log label maps', () => {
  it('maps order automation actions to Chinese (R129 UX v8 P2)', () => {
    expect(operationLogActionLabel('order_automation.execute')).toBe('订单自动化执行');
    expect(operationLogActionLabel('order_automation_rule.create')).toBe('创建订单自动化规则');
    expect(operationLogActionLabel('order_automation_log.retry')).toBe('重试订单自动化执行');
    expect(operationLogResourceLabel('order_automation')).toBe('订单自动化');
  });

  it('maps backend-logged resources that previously fell back to raw keys', () => {
    for (const key of ['order_review', 'admin_user', 'auth_session', 'waybill', 'procurement']) {
      const label = operationLogResourceLabel(key);
      expect(label).not.toBe(key);
      expect(label).toMatch(/[\u4e00-\u9fff]/);
    }
  });

  it('keeps every mapped label Chinese-readable (no raw dotted keys as values)', () => {
    for (const [key, label] of [
      ...Object.entries(OPERATION_LOG_ACTION_LABEL),
      ...Object.entries(OPERATION_LOG_RESOURCE_LABEL),
    ]) {
      expect(label.trim(), `label for ${key}`).not.toBe('');
      expect(label, `label for ${key}`).not.toBe(key);
    }
  });

  it('falls back to a humanized segment label for unknown keys', () => {
    expect(operationLogActionLabel('')).toBe('—');
    expect(operationLogActionLabel('order.create')).toBe('创建订单');
    expect(operationLogActionLabel('unknown_thing.frobnicate')).toContain('·');
  });
});

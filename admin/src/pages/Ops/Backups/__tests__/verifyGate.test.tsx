import { describe, expect, it, vi } from 'vitest';

vi.mock('@/services/opsP6', () => ({
  fetchBackups: vi.fn(),
  createBackup: vi.fn(),
  verifyBackup: vi.fn(),
  downloadBackup: vi.fn(),
}));

import { verifyDisabledReason } from '../index';

describe('备份校验按钮状态门（R58 ①）', () => {
  it('completed 状态可校验（无禁用原因）', () => {
    expect(verifyDisabledReason('completed')).toBeUndefined();
  });

  it('manual_review 状态禁用并给出 BACKUP_ENABLED 中文口径', () => {
    const reason = verifyDisabledReason('manual_review');
    expect(reason).toContain('BACKUP_ENABLED');
    expect(reason).toContain('人工审查');
  });

  it('其他非 completed 状态也禁用校验', () => {
    expect(verifyDisabledReason('failed')).toContain('仅已完成');
    expect(verifyDisabledReason('pending')).toContain('仅已完成');
    expect(verifyDisabledReason(undefined)).toContain('仅已完成');
  });
});

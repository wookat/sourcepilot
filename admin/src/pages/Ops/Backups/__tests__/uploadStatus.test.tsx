import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('@/services/opsP6', () => ({
  fetchBackups: vi.fn(),
  createBackup: vi.fn(),
  verifyBackup: vi.fn(),
  downloadBackup: vi.fn(),
  holdBackup: vi.fn(),
  retryBackupUpload: vi.fn(),
}));

import { uploadStatusTag } from '../index';

describe('备份对象存储上传状态展示（R138 线1）', () => {
  it('uploaded 显示已上传', () => {
    render(uploadStatusTag({ uploadStatus: 'uploaded' }));
    expect(screen.getByText('已上传')).toBeInTheDocument();
  });

  it('failed 显示上传失败', () => {
    render(uploadStatusTag({ uploadStatus: 'failed', uploadError: 'injected' }));
    expect(screen.getByText('上传失败')).toBeInTheDocument();
  });

  it('skipped 显示仅本地（降级模式）', () => {
    render(uploadStatusTag({ uploadStatus: 'skipped' }));
    expect(screen.getByText('仅本地')).toBeInTheDocument();
  });

  it('历史记录无上传状态时显示占位符', () => {
    render(uploadStatusTag({}));
    expect(screen.getByText('-')).toBeInTheDocument();
  });
});

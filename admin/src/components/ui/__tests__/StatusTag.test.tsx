import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import StatusTag from '../StatusTag';

describe('StatusTag', () => {
  it('renders shared collect task labels for known statuses', () => {
    const { rerender } = render(<StatusTag status="running" />);
    expect(screen.getByText('处理中')).toBeInTheDocument();

    rerender(<StatusTag status="retrying" />);
    expect(screen.getByText('等待重试').closest('.ant-tag-warning')).not.toBeNull();
  });

  it('maps extended shared statuses to unified labels and colors', () => {
    const { rerender } = render(<StatusTag status="matched" />);
    expect(screen.getByText('已匹配').closest('.ant-tag-success')).not.toBeNull();

    rerender(<StatusTag status="not_ready" />);
    expect(screen.getByText('未就绪').closest('.ant-tag-error')).not.toBeNull();

    rerender(<StatusTag status="manual_review" />);
    expect(screen.getByText('待人工复核').closest('.ant-tag-warning')).not.toBeNull();

    rerender(<StatusTag status="partial" />);
    expect(screen.getByText('部分完成').closest('.ant-tag-warning')).not.toBeNull();
  });

  it('keeps unknown status raw with default color', () => {
    render(<StatusTag status="some_unknown_status" />);
    expect(screen.getByText('some_unknown_status')).toBeInTheDocument();
  });

  it('forwards ref and DOM props so Tooltip can attach', () => {
    const ref = { current: null as HTMLElement | null };
    render(<StatusTag ref={ref} status="failed" data-testid="status-tag" onMouseEnter={() => {}} />);

    const tag = screen.getByTestId('status-tag');
    expect(tag).toBeInTheDocument();
    expect(ref.current).toBe(tag);
  });

  it('prefers explicit text and className overrides', () => {
    render(<StatusTag status="failed" text="平台返回失败" className="custom-status" />);

    const tag = screen.getByText('平台返回失败');
    expect(tag).toBeInTheDocument();
    expect(tag.closest('.custom-status')).not.toBeNull();
  });
});

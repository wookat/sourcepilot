import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import PlatformTag from '../PlatformTag';

describe('PlatformTag', () => {
  it('maps internal enum to Chinese label with brand color', () => {
    render(<PlatformTag platform="douyin_shop" />);

    const tag = screen.getByText('抖店');
    expect(tag).toBeInTheDocument();
    expect(tag.closest('.ant-tag-volcano')).not.toBeNull();
    expect(screen.queryByText('douyin_shop')).toBeNull();
  });

  it('maps migration platform to Chinese label', () => {
    render(<PlatformTag platform="migration" />);

    const tag = screen.getByText('迁移导入');
    expect(tag).toBeInTheDocument();
    expect(tag.closest('.ant-tag-purple')).not.toBeNull();
    expect(screen.queryByText('migration')).toBeNull();
  });

  it('keeps unknown platform value visible with default color', () => {
    render(<PlatformTag platform="unknown_platform" />);

    const tag = screen.getByText('unknown_platform');
    expect(tag).toBeInTheDocument();
  });

  it('renders placeholder for empty platform', () => {
    const { container } = render(<PlatformTag platform="" />);

    expect(container.textContent).toBe('—');
    expect(container.querySelector('.ant-tag')).toBeNull();
  });

  it('avoids word-break wrapping via nowrap', () => {
    render(<PlatformTag platform="tiktok" />);

    const tag = screen.getByText('TikTok Shop').closest('.ant-tag') as HTMLElement;
    expect(tag.style.whiteSpace).toBe('nowrap');
  });
});

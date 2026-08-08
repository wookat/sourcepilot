import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import DashboardScreenPage from '../index';

const queryDashboardScreen = vi.fn();
const queryDashboardScreenConfig = vi.fn();
const updateDashboardScreenConfig = vi.fn();

vi.mock('@/services/dashboard', () => ({
  queryDashboardScreen: (...args: unknown[]) => queryDashboardScreen(...args),
  queryDashboardScreenConfig: (...args: unknown[]) => queryDashboardScreenConfig(...args),
  updateDashboardScreenConfig: (...args: unknown[]) => updateDashboardScreenConfig(...args),
}));

vi.mock('@/hooks/usePermission', () => ({
  usePermission: () => ({ canManageSettings: true }),
}));

vi.mock('@ant-design/plots', () => ({
  Line: () => <div data-testid="mock-line" />,
  Column: () => <div data-testid="mock-column" />,
}));

// R189（收口 R188 线2 P2-4）：浏览器全屏 (Fullscreen API) 只渲染全屏元素的
// 子树，挂在 body 上的弹窗在全屏态不可见。配置弹窗必须挂载到大屏根节点内。
describe('大屏卡片配置弹窗容器', () => {
  // 全局 setup 的 matchMedia 为 vi.fn()，restoreMocks 会在用例前清空其实现，
  // antd useBreakpoint 需要真实返回值，这里用普通函数重新桩上。
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }),
    });
  });

  it('配置弹窗挂载在大屏根节点内（全屏态可见），不挂 body', async () => {
    queryDashboardScreen.mockRejectedValue(new Error('offline'));
    queryDashboardScreenConfig.mockResolvedValue({ cards: [] });
    const user = userEvent.setup();
    render(<DashboardScreenPage />);

    await user.click(await screen.findByTestId('screen-config-button'));

    const root = screen.getByTestId('dashboard-screen-root');
    const dialog = await within(root).findByRole('dialog');
    expect(within(dialog).getByText('自定义大屏指标')).toBeInTheDocument();
  });
});

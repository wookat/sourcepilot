import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useModel } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { McpAuditLogRow, McpTokenRow } from '@/services/mcpTokens';
import McpTokensPage from '../McpTokens';

const listMcpTokens = vi.fn();
const listMcpAuditLogs = vi.fn();
const fetchSettingsList = vi.fn();
const saveSettingsItems = vi.fn();

vi.mock('@/services/mcpTokens', () => ({
  listMcpTokens: (...args: unknown[]) => listMcpTokens(...args),
  listMcpAuditLogs: (...args: unknown[]) => listMcpAuditLogs(...args),
  createMcpToken: vi.fn(),
  createMcpWriteToken: vi.fn(),
  revokeMcpToken: vi.fn(),
}));

vi.mock('@/services/settings', () => ({
  fetchSettingsList: (...args: unknown[]) => fetchSettingsList(...args),
  saveSettingsItems: (...args: unknown[]) => saveSettingsItems(...args),
}));

function auditRow(id: string, tool: string): McpAuditLogRow {
  return {
    id,
    tokenId: 'e2e-token-1',
    tokenName: 'e2e-claude',
    tokenMasked: 'tmmcp_****abcd',
    tool,
    status: 'success',
    durationMs: 12,
    createdAt: '2026-08-07T00:00:00Z',
  };
}

function tokenRow(id: string, name: string, scope: string): McpTokenRow {
  return {
    id,
    name,
    maskedToken: 'tmmcp_****' + id.slice(-4),
    scope,
    purpose: 'mcp',
    revoked: false,
    expired: false,
    createdAt: '2026-08-07T00:00:00Z',
  };
}

type Deferred<T> = { promise: Promise<T>; resolve: (v: T) => void };

function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

describe('McpTokensPage 审计卡片轻刷新时序（R150 v24 P2-1 回归）', () => {
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

  it('迟到的旧审计响应不得覆盖后发请求的结果（不闪回「暂无数据」）', async () => {
    fetchSettingsList.mockResolvedValue({ items: [] });
    listMcpTokens.mockResolvedValue([]);
    const first = deferred<{ total: number; items: McpAuditLogRow[] }>();
    const second = deferred<{ total: number; items: McpAuditLogRow[] }>();
    listMcpAuditLogs.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

    const user = userEvent.setup();
    render(<McpTokensPage />);

    // 首次挂载请求仍在途时点「刷新」，形成两笔并发的审计加载
    await user.click(screen.getByRole('button', { name: '刷 新' }));
    expect(listMcpAuditLogs).toHaveBeenCalledTimes(2);

    // 后发请求先返回：工具调用已入库
    second.resolve({ total: 1, items: [auditRow('log-1', 'orders_query')] });
    expect(await screen.findByText('tmmcp_****abcd')).toBeInTheDocument();

    // 先发请求迟到且为空：不得把已渲染的数据闪回成「暂无数据」
    first.resolve({ total: 0, items: [] });
    // 等待迟到响应的 setState 全部落盘后再断言
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    const auditCard = screen
      .getByText('MCP / 开放 API 调用审计日志')
      .closest('.ant-card') as HTMLElement;
    expect(within(auditCard).getByText('tmmcp_****abcd')).toBeInTheDocument();
    expect(within(auditCard).queryByText('暂无数据')).toBeNull();
  });

  it('文档入口为可点击链接，指向站内自托管文档（UX v10 P2-3）', async () => {
    fetchSettingsList.mockResolvedValue({ items: [] });
    listMcpTokens.mockResolvedValue([]);
    listMcpAuditLogs.mockResolvedValue({ total: 0, items: [] });
    render(<McpTokensPage />);

    const mcpLink = await screen.findByRole('link', { name: 'docs/mcp.md' });
    expect(mcpLink).toHaveAttribute('href', '/docs/mcp.md');
    expect(mcpLink).toHaveAttribute('target', '_blank');
    const openApiLink = screen.getByRole('link', { name: 'docs/open-api.md' });
    expect(openApiLink).toHaveAttribute('href', '/docs/open-api.md');
    expect(openApiLink).toHaveAttribute('target', '_blank');
  });
});

describe('McpTokensPage MCP 写白名单管理（R180 W2）', () => {
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
    fetchSettingsList.mockResolvedValue({ items: [] });
    listMcpAuditLogs.mockResolvedValue({ total: 0, items: [] });
  });

  it('管理员可见写管理卡片：租户开关默认关、写 token 单独列表、风险提示', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'admin' } },
    });
    listMcpTokens.mockResolvedValue([
      tokenRow('token-r1', 'reader', 'readonly'),
      tokenRow('token-w1', 'writer', 'readonly,write:ops'),
    ]);
    render(<McpTokensPage />);

    const writeCard = (await screen.findByText('MCP 写白名单（write:ops，仅管理员）')).closest(
      '.ant-card',
    ) as HTMLElement;
    expect(within(writeCard).getByText('高风险能力：三层闸门默认全关')).toBeInTheDocument();
    // 租户开关默认关闭
    expect(within(writeCard).getByRole('switch')).not.toBeChecked();
    // 写 token 只出现在写卡片，只读表格不展示
    expect(await within(writeCard).findByText('writer')).toBeInTheDocument();
    const readCard = screen
      .getByText('创建只读 token', { selector: 'span' })
      .closest('.ant-card') as HTMLElement;
    expect(within(readCard).getByText('reader')).toBeInTheDocument();
    expect(within(readCard).queryByText('writer')).toBeNull();
  });

  it('开启租户开关前弹出风险确认，确认后才保存设置', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'admin' } },
    });
    listMcpTokens.mockResolvedValue([]);
    saveSettingsItems.mockResolvedValue({ items: [] });
    const user = userEvent.setup();
    render(<McpTokensPage />);

    const writeCard = (await screen.findByText('MCP 写白名单（write:ops，仅管理员）')).closest(
      '.ant-card',
    ) as HTMLElement;
    await user.click(within(writeCard).getByRole('switch'));
    expect(saveSettingsItems).not.toHaveBeenCalled();
    expect(await screen.findByText('确认开启本租户的 MCP 写白名单？')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '确认开启' }));
    expect(saveSettingsItems).toHaveBeenCalledWith([
      expect.objectContaining({ groupKey: 'mcp', itemKey: 'write_enabled', itemValue: 'true' }),
    ]);
  });

  it('operator 不可见写管理卡片，且不拉取写开关设置', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'operator' } },
    });
    listMcpTokens.mockResolvedValue([tokenRow('token-r1', 'reader', 'readonly')]);
    render(<McpTokensPage />);

    expect(await screen.findByText('reader')).toBeInTheDocument();
    expect(screen.queryByText('MCP 写白名单（write:ops，仅管理员）')).toBeNull();
    expect(fetchSettingsList).not.toHaveBeenCalled();
  });

  it('审计列表展示 mode / paramsSummary / confirmHash', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'admin' } },
    });
    listMcpTokens.mockResolvedValue([]);
    listMcpAuditLogs.mockResolvedValue({
      total: 1,
      items: [
        {
          ...auditRow('log-w1', 'procurement_mark_placed'),
          mode: 'execute',
          paramsSummary: 'purchaseOrderId=po-1 externalOrderId=1688-X1',
          confirmHash: 'abcdef0123456789abcdef0123456789',
        },
      ],
    });
    render(<McpTokensPage />);

    expect(await screen.findByText('execute 执行')).toBeInTheDocument();
    expect(
      screen.getByText('purchaseOrderId=po-1 externalOrderId=1688-X1'),
    ).toBeInTheDocument();
    expect(screen.getByText('abcdef012345…')).toBeInTheDocument();
  });

  it('审计列表金额列：金额型动作显示金额，其余显示 —', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'admin' } },
    });
    listMcpTokens.mockResolvedValue([]);
    listMcpAuditLogs.mockResolvedValue({
      total: 2,
      items: [
        {
          ...auditRow('log-paid', 'procurement_mark_paid'),
          mode: 'execute',
          amount: 88.5,
        },
        {
          ...auditRow('log-tag', 'orders_add_tag'),
          mode: 'execute',
        },
      ],
    });
    render(<McpTokensPage />);

    expect(await screen.findByText('88.50')).toBeInTheDocument();
    expect(screen.getByText('金额（仅支付登记）')).toBeInTheDocument();
  });

  it('管理员可配置 mark-paid 金额限额：回显租户设置并保存两个限额键（R185 v13）', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'admin' } },
    });
    listMcpTokens.mockResolvedValue([]);
    fetchSettingsList.mockResolvedValue({
      items: [
        { groupKey: 'mcp', itemKey: 'mark_paid_single_limit', itemValue: '500' },
        { groupKey: 'mcp', itemKey: 'mark_paid_daily_limit', itemValue: '2000' },
      ],
    });
    saveSettingsItems.mockResolvedValue({ items: [] });
    const user = userEvent.setup();
    render(<McpTokensPage />);

    const writeCard = (await screen.findByText('MCP 写白名单（write:ops，仅管理员）')).closest(
      '.ant-card',
    ) as HTMLElement;
    // 已配置的限额需要回显
    const single = await within(writeCard).findByLabelText('单笔上限');
    const daily = within(writeCard).getByLabelText('日累计上限');
    expect(single).toHaveValue('500.00');
    expect(daily).toHaveValue('2000.00');

    await user.clear(single);
    await user.type(single, '800');
    await user.click(within(writeCard).getByRole('button', { name: '保存限额' }));
    expect(saveSettingsItems).toHaveBeenCalledWith([
      expect.objectContaining({
        groupKey: 'mcp',
        itemKey: 'mark_paid_single_limit',
        itemValue: '800',
      }),
      expect.objectContaining({
        groupKey: 'mcp',
        itemKey: 'mark_paid_daily_limit',
        itemValue: '2000',
      }),
    ]);
  });

  it('非管理员不展示写审计列与调用模式筛选（R184 最小暴露）', async () => {
    vi.mocked(useModel).mockReturnValue({
      initialState: { currentUser: { role: 'operator' } },
    });
    listMcpTokens.mockResolvedValue([]);
    listMcpAuditLogs.mockResolvedValue({
      total: 1,
      items: [auditRow('log-r1', 'orders_query')],
    });
    render(<McpTokensPage />);

    expect(await screen.findByText('orders_query')).toBeInTheDocument();
    expect(screen.queryByText('模式')).toBeNull();
    expect(screen.queryByText('参数摘要')).toBeNull();
    expect(screen.queryByText('确认哈希')).toBeNull();
    expect(screen.queryByText('金额（仅支付登记）')).toBeNull();
    expect(screen.queryByText('调用模式')).toBeNull();
  });
});

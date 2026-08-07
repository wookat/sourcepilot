import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { McpAuditLogRow } from '@/services/mcpTokens';
import McpTokensPage from '../McpTokens';

const listMcpTokens = vi.fn();
const listMcpAuditLogs = vi.fn();

vi.mock('@/services/mcpTokens', () => ({
  listMcpTokens: (...args: unknown[]) => listMcpTokens(...args),
  listMcpAuditLogs: (...args: unknown[]) => listMcpAuditLogs(...args),
  createMcpToken: vi.fn(),
  revokeMcpToken: vi.fn(),
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

type Deferred<T> = { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void };

function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((r, j) => {
    resolve = r;
    reject = j;
  });
  return { promise, resolve, reject };
}

function auditCardEl(): HTMLElement {
  return screen.getByText('工具调用审计日志').closest('.ant-card') as HTMLElement;
}

async function flush() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
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
    const auditCard = auditCardEl();
    expect(within(auditCard).getByText('tmmcp_****abcd')).toBeInTheDocument();
    expect(within(auditCard).queryByText('暂无数据')).toBeNull();
  });

  it('新审计行入库后点「刷新」：新行出现，刷新期间旧行保持可见、不闪回「暂无数据」（v25 P2 时序补验）', async () => {
    listMcpTokens.mockResolvedValue([]);
    const initial = deferred<{ total: number; items: McpAuditLogRow[] }>();
    const refresh = deferred<{ total: number; items: McpAuditLogRow[] }>();
    listMcpAuditLogs.mockReturnValueOnce(initial.promise).mockReturnValueOnce(refresh.promise);

    const user = userEvent.setup();
    render(<McpTokensPage />);

    // 首屏：已有 1 条审计行（对比数据）
    initial.resolve({ total: 1, items: [auditRow('log-1', 'orders_query')] });
    expect(await screen.findByText('orders_query')).toBeInTheDocument();

    // 新审计行入库后用户点「刷新」；响应在途期间旧行必须保持可见
    await user.click(screen.getByRole('button', { name: '刷 新' }));
    let auditCard = auditCardEl();
    expect(within(auditCard).getByText('orders_query')).toBeInTheDocument();
    expect(within(auditCard).queryByText('暂无数据')).toBeNull();

    // 响应返回：新行出现且旧行仍在
    refresh.resolve({
      total: 2,
      items: [auditRow('log-2', 'inventory_query'), auditRow('log-1', 'orders_query')],
    });
    await flush();
    auditCard = auditCardEl();
    expect(within(auditCard).getByText('inventory_query')).toBeInTheDocument();
    expect(within(auditCard).getByText('orders_query')).toBeInTheDocument();
    expect(within(auditCard).queryByText('暂无数据')).toBeNull();
  });

  it('迟到的旧请求报错不得覆盖后发请求的成功结果（不显示错误 Alert）', async () => {
    listMcpTokens.mockResolvedValue([]);
    const first = deferred<{ total: number; items: McpAuditLogRow[] }>();
    const second = deferred<{ total: number; items: McpAuditLogRow[] }>();
    listMcpAuditLogs.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

    const user = userEvent.setup();
    render(<McpTokensPage />);

    await user.click(screen.getByRole('button', { name: '刷 新' }));
    expect(listMcpAuditLogs).toHaveBeenCalledTimes(2);

    second.resolve({ total: 1, items: [auditRow('log-1', 'orders_query')] });
    expect(await screen.findByText('tmmcp_****abcd')).toBeInTheDocument();

    // 先发请求迟到且失败：不得把已渲染的数据覆盖成错误态
    first.reject(new Error('审计日志加载超时'));
    await flush();
    const auditCard = auditCardEl();
    expect(within(auditCard).getByText('tmmcp_****abcd')).toBeInTheDocument();
    expect(within(auditCard).queryByText('审计日志加载超时')).toBeNull();
  });
});

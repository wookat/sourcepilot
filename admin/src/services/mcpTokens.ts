import { getWithParams, postJSON } from '@/services/request';

export type McpTokenRow = {
  id: string;
  name: string;
  maskedToken: string;
  scope: string;
  /** token 用途：mcp（MCP 只读）/ openapi（开放 API）/ both（两者）。 */
  purpose: string;
  revoked: boolean;
  expired: boolean;
  createdAt: string;
  expiresAt?: string;
  lastUsedAt?: string;
  revokedAt?: string;
};

export async function listMcpTokens(): Promise<McpTokenRow[]> {
  const data = await getWithParams<{ items: McpTokenRow[] }>('/api/v1/mcp/tokens', {});
  return data.items || [];
}

export type CreateMcpTokenResult = {
  token: McpTokenRow;
  /** 明文 token 仅在创建时返回一次，页面展示后不再可见。 */
  plaintext: string;
};

export async function createMcpToken(
  name: string,
  expiresInDays?: number,
  purpose?: string,
): Promise<CreateMcpTokenResult> {
  return postJSON('/api/v1/mcp/tokens', {
    name,
    ...(purpose ? { purpose } : {}),
    ...(expiresInDays && expiresInDays > 0 ? { expiresInDays } : {}),
  });
}

/**
 * 创建带 write:ops 作用域的写 token（仅管理员；后端同步校验）。
 * expiresInDays 为 0 时后端默认 30 天，最长 90 天，不支持不过期。
 */
export async function createMcpWriteToken(
  name: string,
  expiresInDays?: number,
): Promise<CreateMcpTokenResult> {
  return postJSON('/api/v1/mcp/tokens', {
    name,
    scopes: ['readonly', 'write:ops'],
    ...(expiresInDays && expiresInDays > 0 ? { expiresInDays } : {}),
  });
}

export async function revokeMcpToken(id: string): Promise<{ token: McpTokenRow }> {
  return postJSON(`/api/v1/mcp/tokens/${id}/revoke`, {});
}

export type McpAuditLogRow = {
  id: string;
  tokenId: string;
  tokenName: string;
  tokenMasked: string;
  tool: string;
  status: 'success' | 'error' | 'auth_failed' | 'rate_limited';
  durationMs: number;
  createdAt: string;
  /** 写管道字段（R180 W2）：读工具为空。mode 为 dry_run / execute。 */
  mode?: string;
  /** 白名单参数摘要（仅标识符，不含敏感值与确认 token）。 */
  paramsSummary?: string;
  /** 确认 token 绑定哈希（dry_run 签发 / execute 核销对账用）。 */
  confirmHash?: string;
  /** 金额，仅金额型写动作（procurement_mark_paid）有意义，其余为 0/缺省。 */
  amount?: number;
};

export type McpAuditLogQuery = {
  page?: number;
  pageSize?: number;
  tool?: string;
  status?: string;
  mode?: string;
};

export async function listMcpAuditLogs(
  query: McpAuditLogQuery,
): Promise<{ total: number; items: McpAuditLogRow[] }> {
  return getWithParams('/api/v1/mcp/audit-logs', query);
}

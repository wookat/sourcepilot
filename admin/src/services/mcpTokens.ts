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

export async function revokeMcpToken(id: string): Promise<{ token: McpTokenRow }> {
  return postJSON(`/api/v1/mcp/tokens/${id}/revoke`, {});
}

export type McpAuditLogRow = {
  id: string;
  tokenId: string;
  tokenName: string;
  tokenMasked: string;
  tool: string;
  status: 'success' | 'error';
  durationMs: number;
  createdAt: string;
};

export type McpAuditLogQuery = {
  page?: number;
  pageSize?: number;
  tool?: string;
  status?: string;
};

export async function listMcpAuditLogs(
  query: McpAuditLogQuery,
): Promise<{ total: number; items: McpAuditLogRow[] }> {
  return getWithParams('/api/v1/mcp/audit-logs', query);
}

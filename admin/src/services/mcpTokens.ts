import { getWithParams, postJSON } from '@/services/request';

export type McpTokenRow = {
  id: string;
  name: string;
  maskedToken: string;
  scope: string;
  revoked: boolean;
  createdAt: string;
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

export async function createMcpToken(name: string): Promise<CreateMcpTokenResult> {
  return postJSON('/api/v1/mcp/tokens', { name });
}

export async function revokeMcpToken(id: string): Promise<{ token: McpTokenRow }> {
  return postJSON(`/api/v1/mcp/tokens/${id}/revoke`, {});
}

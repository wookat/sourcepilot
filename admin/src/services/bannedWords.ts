import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type BannedWordLevel = 'forbidden' | 'warning';

export type BannedWordRow = {
  id: string;
  tenantId: number;
  word: string;
  category: string;
  level: BannedWordLevel | string;
  isPreset: boolean;
  enabled: boolean;
  suggestion?: string;
  createdAt: string;
  updatedAt: string;
};

export type BannedWordCategory = {
  category: string;
  categoryLabel: string;
  enabled: boolean;
  wordCount: number;
};

export type BannedWordHitPosition = { start: number; end: number };

export type BannedWordHit = {
  word: string;
  field: string;
  fieldLabel: string;
  category: string;
  categoryLabel: string;
  level: BannedWordLevel | string;
  levelLabel: string;
  suggestion?: string;
  positions: BannedWordHitPosition[];
};

export type BannedWordScanField = {
  field: string;
  label: string;
  text: string;
};

export type BannedWordScanResult = {
  productId: string;
  status: 'passed' | 'warning' | 'blocked' | string;
  statusLabel?: string;
  forbiddenCount: number;
  warningCount: number;
  hits: BannedWordHit[];
  fields?: BannedWordScanField[];
};

export const bannedWordLevelLabel = (level: string): string =>
  level === 'forbidden' ? '禁止' : level === 'warning' ? '警告' : level;

export type BannedWordListQuery = {
  category?: string;
  level?: string;
  keyword?: string;
  enabled?: boolean;
};

export async function listBannedWords(query: BannedWordListQuery = {}): Promise<BannedWordRow[]> {
  const params: Record<string, string> = {};
  if (query.category) params.category = query.category;
  if (query.level) params.level = query.level;
  if (query.keyword) params.keyword = query.keyword;
  if (query.enabled) params.enabled = '1';
  const data = await getWithParams<{ items: BannedWordRow[] }>('/api/v1/banned-words', params);
  return data.items || [];
}

export type CreateBannedWordBody = {
  word: string;
  category?: string;
  level?: BannedWordLevel;
  suggestion?: string;
};

export async function createBannedWord(body: CreateBannedWordBody): Promise<BannedWordRow> {
  return postJSON('/api/v1/banned-words', body);
}

export type UpdateBannedWordBody = {
  enabled?: boolean;
  level?: BannedWordLevel;
  category?: string;
  suggestion?: string;
};

export async function updateBannedWord(id: string, body: UpdateBannedWordBody): Promise<BannedWordRow> {
  return putJSON(`/api/v1/banned-words/${id}`, body);
}

export async function deleteBannedWord(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/banned-words/${id}`);
}

export async function listBannedWordCategories(): Promise<BannedWordCategory[]> {
  const data = await getWithParams<{ items: BannedWordCategory[] }>(
    '/api/v1/banned-words/categories',
    {},
  );
  return data.items || [];
}

export async function toggleBannedWordCategory(
  category: string,
  enabled: boolean,
): Promise<BannedWordCategory> {
  return putJSON(`/api/v1/banned-words/categories/${encodeURIComponent(category)}`, { enabled });
}

export async function checkProductBannedWords(productId: string): Promise<BannedWordScanResult> {
  return getWithParams<BannedWordScanResult>(
    `/api/v1/products/${encodeURIComponent(productId)}/banned-words/check`,
    {},
  );
}

export async function batchCheckBannedWords(
  productIds: string[],
): Promise<{ list: BannedWordScanResult[] }> {
  return postJSON<{ list: BannedWordScanResult[] }>('/api/v1/products/banned-words/check-batch', {
    productIds,
  });
}

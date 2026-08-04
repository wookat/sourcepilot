import { ok } from './envelope';
import { E2E_PRODUCT_ID } from './product.fixture';

export const E2E_BANNED_WORD_PRESET_ID = 'e2e-banned-word-preset';
export const E2E_BANNED_WORD_CUSTOM_ID = 'e2e-banned-word-custom';

export const e2eBannedWords = [
  {
    id: E2E_BANNED_WORD_PRESET_ID,
    tenantId: 1,
    word: '最佳',
    category: 'ad_extreme',
    level: 'forbidden',
    isPreset: true,
    enabled: true,
    suggestion: '可改为「优选」「精选」等非绝对化表述。',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: E2E_BANNED_WORD_CUSTOM_ID,
    tenantId: 1,
    word: '祖传',
    category: 'custom',
    level: 'warning',
    isPreset: false,
    enabled: true,
    suggestion: '建议改为可证明的工艺描述。',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eBannedWordCategories = [
  { category: 'ad_extreme', categoryLabel: '广告法极限词', enabled: true, wordCount: 22 },
  { category: 'general', categoryLabel: '通用违禁词', enabled: true, wordCount: 7 },
  { category: 'medical', categoryLabel: '医疗功效词', enabled: true, wordCount: 10 },
  { category: 'infringement', categoryLabel: '品牌侵权词', enabled: false, wordCount: 5 },
  { category: 'custom', categoryLabel: '自定义', enabled: true, wordCount: 1 },
];

export const e2eBannedWordScanBlocked = {
  productId: E2E_PRODUCT_ID,
  status: 'blocked',
  statusLabel: '存在禁止级违禁词',
  forbiddenCount: 1,
  warningCount: 1,
  hits: [
    {
      word: '最佳',
      field: 'title',
      fieldLabel: '商品标题',
      category: 'ad_extreme',
      categoryLabel: '广告法极限词',
      level: 'forbidden',
      levelLabel: '禁止',
      suggestion: '可改为「优选」「精选」等非绝对化表述。',
      positions: [{ start: 3, end: 5 }],
    },
    {
      word: '祖传',
      field: 'description',
      fieldLabel: '商品详情/卖点',
      category: 'custom',
      categoryLabel: '自定义',
      level: 'warning',
      levelLabel: '警告',
      suggestion: '建议改为可证明的工艺描述。',
      positions: [{ start: 0, end: 2 }],
    },
  ],
  fields: [
    { field: 'title', label: '商品标题', text: 'E2E最佳商品标题' },
    { field: 'aiTitle', label: 'AI 标题', text: '' },
    { field: 'description', label: '商品详情/卖点', text: '祖传工艺，品质可靠。' },
    { field: 'aiDescription', label: 'AI 描述', text: '' },
  ],
};

export function bannedWordsResponse(path: string) {
  if (path === '/api/v1/banned-words') return ok({ items: e2eBannedWords });
  if (path === '/api/v1/banned-words/categories') return ok({ items: e2eBannedWordCategories });
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/banned-words/check`) {
    return ok(e2eBannedWordScanBlocked);
  }
  return null;
}

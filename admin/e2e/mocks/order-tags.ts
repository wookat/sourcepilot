import { ok } from './envelope';

export const E2E_TAG_URGENT_ID = 'e2e-order-tag-urgent';
export const E2E_TAG_VIP_ID = 'e2e-order-tag-vip';

export const e2eOrderTags = [
  {
    id: E2E_TAG_URGENT_ID,
    tenantId: 1,
    name: '加急',
    color: 'red',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: E2E_TAG_VIP_ID,
    tenantId: 1,
    name: '大客户',
    color: 'gold',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export function orderTagsResponse(path: string) {
  if (path === '/api/v1/order-tags') return ok({ items: e2eOrderTags });
  return null;
}

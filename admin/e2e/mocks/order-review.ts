import { ok } from './envelope';

export const E2E_REVIEW_RULE_ID = 'e2e-review-rule-1';
export const E2E_REVIEW_ORDER_HELD_ID = 'e2e-review-order-held';
export const E2E_REVIEW_ORDER_PENDING_ID = 'e2e-review-order-pending';

export const e2eOrderReviewRules = [
  {
    id: E2E_REVIEW_RULE_ID,
    tenantId: 1,
    name: '大额订单人工审核',
    priority: 1,
    enabled: true,
    action: 'review',
    minAmount: 500,
    remarkKeywords: ['加急'],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'e2e-review-rule-2',
    tenantId: 1,
    name: '黑名单地区挂起',
    priority: 2,
    enabled: false,
    action: 'hold',
    addressKeywords: ['某某区'],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eReviewWorkbenchRows = [
  {
    id: E2E_REVIEW_ORDER_HELD_ID,
    orderNo: 'SO-E2E-HELD-1',
    platform: 'manual',
    customerName: 'E2E 买家甲',
    status: 'paid',
    reviewStatus: 'held',
    currency: 'CNY',
    totalAmount: 880,
    itemCount: 2,
    createdAt: '2026-01-02T00:00:00Z',
    hits: [
      {
        id: 'e2e-review-hit-1',
        orderId: E2E_REVIEW_ORDER_HELD_ID,
        ruleId: 'e2e-review-rule-2',
        ruleName: '黑名单地区挂起',
        action: 'hold',
        reason: '收货地址含关键词「某某区」',
        decisive: true,
      },
    ],
  },
  {
    id: E2E_REVIEW_ORDER_PENDING_ID,
    orderNo: 'SO-E2E-PEND-1',
    platform: 'manual',
    customerName: 'E2E 买家乙',
    status: 'paid',
    reviewStatus: 'pending_review',
    currency: 'CNY',
    totalAmount: 620,
    itemCount: 1,
    createdAt: '2026-01-02T01:00:00Z',
    hits: [
      {
        id: 'e2e-review-hit-2',
        orderId: E2E_REVIEW_ORDER_PENDING_ID,
        ruleId: E2E_REVIEW_RULE_ID,
        ruleName: '大额订单人工审核',
        action: 'review',
        reason: '订单金额 620.00 落入阈值区间',
        decisive: true,
      },
    ],
  },
];

export function orderReviewResponse(path: string) {
  if (path === '/api/v1/order-review-rules') return ok({ items: e2eOrderReviewRules });
  if (path === '/api/v1/order-review')
    return ok({
      items: e2eReviewWorkbenchRows,
      total: e2eReviewWorkbenchRows.length,
      page: 1,
      pageSize: 20,
      totalPages: 1,
      pendingTotal: e2eReviewWorkbenchRows.length,
    });
  return null;
}

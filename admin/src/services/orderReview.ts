import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

// 审单规则（订单进入时按优先级命中第一条决定动作）
export type OrderReviewAction = 'pass' | 'review' | 'hold';

export type OrderReviewRuleRow = {
  id: string;
  tenantId: number;
  name: string;
  priority: number;
  enabled: boolean;
  action: OrderReviewAction;
  minAmount?: number;
  maxAmount?: number;
  addressKeywords?: string[];
  remarkKeywords?: string[];
  platforms?: string[];
  shopIds?: string[];
  maxTotalQuantity?: number;
  maxSkuQuantity?: number;
  repeatReceiverMinOrders?: number;
  repeatReceiverWindowDays?: number;
  createdAt: string;
  updatedAt: string;
};

export const REVIEW_ACTION_LABELS: Record<OrderReviewAction, string> = {
  pass: '自动通过',
  review: '待人工审核',
  hold: '挂起拦截',
};

export const REVIEW_STATUS_LABELS: Record<string, string> = {
  pending_review: '待人工审核',
  held: '已挂起',
  approved: '已放行',
  rejected: '已拒绝',
  auto_passed: '自动通过',
};

export const REVIEW_STATUS_COLORS: Record<string, string> = {
  pending_review: 'gold',
  held: 'red',
  approved: 'green',
  rejected: 'default',
  auto_passed: 'blue',
};

export type OrderReviewRuleBody = {
  name?: string;
  priority?: number;
  enabled?: boolean;
  action?: OrderReviewAction;
  minAmount?: number;
  maxAmount?: number;
  addressKeywords?: string[];
  remarkKeywords?: string[];
  platforms?: string[];
  shopIds?: string[];
  maxTotalQuantity?: number;
  maxSkuQuantity?: number;
  repeatReceiverMinOrders?: number;
  repeatReceiverWindowDays?: number;
  clearMinAmount?: boolean;
  clearMaxAmount?: boolean;
  clearMaxTotalQuantity?: boolean;
  clearMaxSkuQuantity?: boolean;
  clearRepeatReceiver?: boolean;
};

export async function listOrderReviewRules(): Promise<OrderReviewRuleRow[]> {
  const data = await getWithParams<{ items: OrderReviewRuleRow[] }>(
    '/api/v1/order-review-rules',
    {},
  );
  return data.items || [];
}

export async function createOrderReviewRule(
  body: OrderReviewRuleBody,
): Promise<OrderReviewRuleRow> {
  return postJSON('/api/v1/order-review-rules', body);
}

export async function updateOrderReviewRule(
  id: string,
  body: OrderReviewRuleBody,
): Promise<OrderReviewRuleRow> {
  return putJSON(`/api/v1/order-review-rules/${id}`, body);
}

export async function deleteOrderReviewRule(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/order-review-rules/${id}`);
}

export type OrderReviewDryRunResult = {
  scanned: number;
  matched: number;
  samples: { orderId: string; orderNo: string; amount: number; reason: string }[];
};

export async function dryRunOrderReviewRule(
  body: OrderReviewRuleBody,
): Promise<OrderReviewDryRunResult> {
  return postJSON('/api/v1/order-review-rules/dry-run', body);
}

// 审单工作台
export type OrderReviewHit = {
  id: string;
  orderId: string;
  ruleId: string;
  ruleName: string;
  action: OrderReviewAction;
  reason: string;
  decisive: boolean;
};

export type ReviewOrderRow = {
  id: string;
  orderNo: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  customerName: string;
  status: string;
  reviewStatus: string;
  currency: string;
  totalAmount: number;
  itemCount: number;
  createdAt: string;
  hits: OrderReviewHit[];
};

export type ReviewWorkbenchResult = {
  items: ReviewOrderRow[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
  pendingTotal: number;
};

export async function listOrderReviewWorkbench(params: {
  page?: number;
  pageSize?: number;
  reviewStatus?: string;
  keyword?: string;
}): Promise<ReviewWorkbenchResult> {
  return getWithParams('/api/v1/order-review', params);
}

export type ReviewDecisionResult = {
  total: number;
  done: number;
  failed: number;
  results: { orderId: string; orderNo?: string; ok: boolean; error?: string }[];
};

export async function approveReviewOrders(orderIds: string[]): Promise<ReviewDecisionResult> {
  return postJSON('/api/v1/order-review/approve', { orderIds });
}

export async function rejectReviewOrders(orderIds: string[]): Promise<ReviewDecisionResult> {
  return postJSON('/api/v1/order-review/reject', { orderIds });
}

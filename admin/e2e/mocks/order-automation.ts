import { ok } from './envelope';

export const E2E_AUTOMATION_RULE_ID = 'e2e-automation-rule-1';
export const E2E_AUTOMATION_LOG_FAILED_ID = 'e2e-automation-log-failed';

export const e2eOrderAutomationRules = [
  {
    id: E2E_AUTOMATION_RULE_ID,
    tenantId: 1,
    name: '低额订单自动确认付款',
    priority: 1,
    enabled: true,
    triggerEvent: 'order_created',
    action: 'confirm_payment',
    maxAmount: 200,
    requireReviewPassed: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'e2e-automation-rule-2',
    tenantId: 1,
    name: '付款后自动生成采购单',
    priority: 2,
    enabled: false,
    triggerEvent: 'order_paid',
    action: 'generate_procurement',
    platforms: ['manual'],
    requireReviewPassed: true,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eOrderAutomationLogs = [
  {
    id: 'e2e-automation-log-success',
    tenantId: 1,
    ruleId: E2E_AUTOMATION_RULE_ID,
    ruleName: '低额订单自动确认付款',
    orderId: 'e2e-automation-order-1',
    orderNo: 'SO-E2E-AT-1',
    triggerEvent: 'order_created',
    action: 'confirm_payment',
    status: 'success',
    reason: '已自动确认付款（金额 120.00 低于上限 200.00）',
    attempts: 1,
    createdAt: '2026-01-02T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
  },
  {
    id: E2E_AUTOMATION_LOG_FAILED_ID,
    tenantId: 1,
    ruleId: 'e2e-automation-rule-2',
    ruleName: '付款后自动生成采购单',
    orderId: 'e2e-automation-order-2',
    orderNo: 'SO-E2E-AT-2',
    triggerEvent: 'order_paid',
    action: 'generate_procurement',
    status: 'failed',
    reason: '生成采购单被阻断：SKU 未匹配货源',
    attempts: 3,
    createdAt: '2026-01-02T01:00:00Z',
    updatedAt: '2026-01-02T01:00:00Z',
  },
  {
    id: 'e2e-automation-log-skipped',
    tenantId: 1,
    ruleId: E2E_AUTOMATION_RULE_ID,
    ruleName: '低额订单自动确认付款',
    orderId: 'e2e-automation-order-3',
    orderNo: 'SO-E2E-AT-3',
    triggerEvent: 'order_created',
    action: 'confirm_payment',
    status: 'skipped',
    reason: '订单审单状态为待审核/挂起，安全边界禁止自动化',
    attempts: 1,
    createdAt: '2026-01-02T02:00:00Z',
    updatedAt: '2026-01-02T02:00:00Z',
  },
];

export function orderAutomationResponse(path: string) {
  if (path === '/api/v1/order-automation-rules') return ok({ items: e2eOrderAutomationRules });
  if (path === '/api/v1/order-automation-logs')
    return ok({
      items: e2eOrderAutomationLogs,
      total: e2eOrderAutomationLogs.length,
      page: 1,
      pageSize: 20,
      totalPages: 1,
    });
  return null;
}

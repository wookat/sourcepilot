import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

// 自动化订单规则（R119）：状态事件触发 + 条件 → 站内自动动作
export type AutomationTriggerEvent =
  | 'order_created'
  | 'order_paid'
  | 'procurement_delivered'
  | 'logistics_collected';

export type AutomationAction =
  | 'confirm_payment'
  | 'generate_procurement'
  | 'mark_printed'
  | 'notify_shipping'
  | 'apply_shipping_rule'
  | 'assign_warehouse';

// 自动应用发货规则：仅推荐 / 直接应用（发货时均可人工改选）
export type ShippingApplyMode = 'recommend' | 'apply';

// 自动分仓策略：默认仓 / 库存充足优先
export type WarehouseStrategy = 'default_warehouse' | 'stock_first';

export const SHIPPING_APPLY_MODE_LABELS: Record<ShippingApplyMode, string> = {
  recommend: '仅推荐物流商',
  apply: '直接应用物流商',
};

export const WAREHOUSE_STRATEGY_LABELS: Record<WarehouseStrategy, string> = {
  default_warehouse: '默认仓',
  stock_first: '库存充足优先',
};

export type AutomationLogStatus = 'success' | 'failed' | 'skipped';

export const AUTOMATION_EVENT_LABELS: Record<AutomationTriggerEvent, string> = {
  order_created: '订单创建',
  order_paid: '进入待采购（已付款）',
  procurement_delivered: '采购签收入库',
  logistics_collected: '采购物流揽收',
};

export const AUTOMATION_ACTION_LABELS: Record<AutomationAction, string> = {
  confirm_payment: '自动确认付款',
  generate_procurement: '自动生成采购单',
  mark_printed: '自动标记打单',
  notify_shipping: '自动通知发货工作台',
  apply_shipping_rule: '自动应用发货规则',
  assign_warehouse: '自动分仓',
};

export const AUTOMATION_LOG_STATUS_LABELS: Record<AutomationLogStatus, string> = {
  success: '成功',
  failed: '失败',
  skipped: '跳过',
};

export const AUTOMATION_LOG_STATUS_COLORS: Record<AutomationLogStatus, string> = {
  success: 'green',
  failed: 'red',
  skipped: 'gold',
};

// 每个触发时机允许的动作（与后端 AutomationActionAllowed 对齐）
export const AUTOMATION_EVENT_ACTIONS: Record<AutomationTriggerEvent, AutomationAction[]> = {
  order_created: ['confirm_payment', 'mark_printed', 'apply_shipping_rule'],
  order_paid: ['generate_procurement', 'mark_printed', 'apply_shipping_rule', 'assign_warehouse'],
  procurement_delivered: [
    'notify_shipping',
    'mark_printed',
    'apply_shipping_rule',
    'assign_warehouse',
  ],
  logistics_collected: ['notify_shipping', 'apply_shipping_rule'],
};

export type OrderAutomationRuleRow = {
  id: string;
  tenantId: number;
  name: string;
  priority: number;
  enabled: boolean;
  triggerEvent: AutomationTriggerEvent;
  action: AutomationAction;
  minAmount?: number;
  maxAmount?: number;
  platforms?: string[];
  shopIds?: string[];
  requireReviewPassed: boolean;
  shippingApplyMode?: ShippingApplyMode;
  warehouseStrategy?: WarehouseStrategy;
  createdAt: string;
  updatedAt: string;
};

export type OrderAutomationRuleBody = {
  name?: string;
  priority?: number;
  enabled?: boolean;
  triggerEvent?: AutomationTriggerEvent;
  action?: AutomationAction;
  minAmount?: number;
  maxAmount?: number;
  platforms?: string[];
  shopIds?: string[];
  requireReviewPassed?: boolean;
  shippingApplyMode?: ShippingApplyMode;
  warehouseStrategy?: WarehouseStrategy;
  clearMinAmount?: boolean;
  clearMaxAmount?: boolean;
};

export async function listOrderAutomationRules(): Promise<OrderAutomationRuleRow[]> {
  const data = await getWithParams<{ items: OrderAutomationRuleRow[] }>(
    '/api/v1/order-automation-rules',
    {},
  );
  return data.items || [];
}

export async function createOrderAutomationRule(
  body: OrderAutomationRuleBody,
): Promise<OrderAutomationRuleRow> {
  return postJSON('/api/v1/order-automation-rules', body);
}

export async function updateOrderAutomationRule(
  id: string,
  body: OrderAutomationRuleBody,
): Promise<OrderAutomationRuleRow> {
  return putJSON(`/api/v1/order-automation-rules/${id}`, body);
}

export async function deleteOrderAutomationRule(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/order-automation-rules/${id}`);
}

export type OrderAutomationDryRunResult = {
  scanned: number;
  matched: number;
  blocked: number;
  samples: {
    orderId: string;
    orderNo: string;
    amount: number;
    reason: string;
    blocked: boolean;
  }[];
};

export async function dryRunOrderAutomationRule(
  body: OrderAutomationRuleBody,
): Promise<OrderAutomationDryRunResult> {
  return postJSON('/api/v1/order-automation-rules/dry-run', body);
}

export type OrderAutomationLogRow = {
  id: string;
  tenantId: number;
  ruleId: string;
  ruleName: string;
  orderId: string;
  orderNo: string;
  triggerEvent: AutomationTriggerEvent;
  action: AutomationAction;
  status: AutomationLogStatus;
  reason: string;
  attempts: number;
  createdAt: string;
  updatedAt: string;
};

export type OrderAutomationLogResult = {
  items: OrderAutomationLogRow[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
};

export async function listOrderAutomationLogs(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  triggerEvent?: string;
  action?: string;
  ruleId?: string;
  keyword?: string;
}): Promise<OrderAutomationLogResult> {
  return getWithParams('/api/v1/order-automation-logs', params);
}

export async function retryOrderAutomationLog(id: string): Promise<OrderAutomationLogRow> {
  return postJSON(`/api/v1/order-automation-logs/${id}/retry`, {});
}

export async function listOrderAutomationTrail(
  orderId: string,
): Promise<OrderAutomationLogRow[]> {
  const data = await getWithParams<{ items: OrderAutomationLogRow[] }>(
    `/api/v1/orders/${orderId}/automation-logs`,
    {},
  );
  return data.items || [];
}

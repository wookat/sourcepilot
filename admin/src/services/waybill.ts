import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

// 面单打印模板（自定义打印模板，非电子面单）
export type WaybillTemplateRow = {
  id: string;
  tenantId: number;
  name: string;
  sizeCode: 'a4_list' | '100x180' | '100x150';
  showRecipient: boolean;
  showSender: boolean;
  showItems: boolean;
  showRemark: boolean;
  showCarrierLogo: boolean;
  headerText?: string;
  footerText?: string;
  isDefault: boolean;
  isPreset: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export const WAYBILL_SIZE_LABELS: Record<WaybillTemplateRow['sizeCode'], string> = {
  '100x180': '100×180mm 标准面单',
  '100x150': '100×150mm 小号面单',
  a4_list: 'A4 一联单',
};

export async function listWaybillTemplates(): Promise<WaybillTemplateRow[]> {
  const data = await getWithParams<{ items: WaybillTemplateRow[] }>('/api/v1/waybill-templates', {});
  return data.items || [];
}

export type WaybillTemplateBody = {
  name?: string;
  sizeCode?: string;
  showRecipient?: boolean;
  showSender?: boolean;
  showItems?: boolean;
  showRemark?: boolean;
  showCarrierLogo?: boolean;
  headerText?: string;
  footerText?: string;
  isDefault?: boolean;
  sortOrder?: number;
};

export async function createWaybillTemplate(body: WaybillTemplateBody): Promise<WaybillTemplateRow> {
  return postJSON('/api/v1/waybill-templates', body);
}

export async function updateWaybillTemplate(id: string, body: WaybillTemplateBody): Promise<WaybillTemplateRow> {
  return putJSON(`/api/v1/waybill-templates/${id}`, body);
}

export async function deleteWaybillTemplate(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/waybill-templates/${id}`);
}

// 发货规则（按条件推荐物流商，可手动覆盖，不强制）
export type ShippingRuleRow = {
  id: string;
  tenantId: number;
  name: string;
  priority: number;
  enabled: boolean;
  provinces?: string[];
  platforms?: string[];
  minWeightKg?: number;
  maxWeightKg?: number;
  minAmount?: number;
  maxAmount?: number;
  carrierCode: string;
  createdAt: string;
  updatedAt: string;
};

export async function listShippingRules(): Promise<ShippingRuleRow[]> {
  const data = await getWithParams<{ items: ShippingRuleRow[] }>('/api/v1/shipping-rules', {});
  return data.items || [];
}

export type ShippingRuleBody = {
  name?: string;
  priority?: number;
  enabled?: boolean;
  provinces?: string[];
  platforms?: string[];
  minWeightKg?: number | null;
  maxWeightKg?: number | null;
  minAmount?: number | null;
  maxAmount?: number | null;
  carrierCode?: string;
};

export async function createShippingRule(body: ShippingRuleBody): Promise<ShippingRuleRow> {
  return postJSON('/api/v1/shipping-rules', body);
}

export async function updateShippingRule(id: string, body: ShippingRuleBody): Promise<ShippingRuleRow> {
  return putJSON(`/api/v1/shipping-rules/${id}`, body);
}

export async function deleteShippingRule(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/shipping-rules/${id}`);
}

export type ShippingRuleRecommendation = {
  matched: boolean;
  ruleId?: string;
  ruleName?: string;
  carrierCode?: string;
  carrierName?: string;
};

export async function recommendByAttrs(body: {
  province?: string;
  platform?: string;
  weightKg?: number;
  amount?: number;
}): Promise<ShippingRuleRecommendation> {
  return postJSON('/api/v1/shipping-rules/recommend', body);
}

// 订单动线：按订单批量取推荐（省份/重量可选补充）
export type OrderShippingRecommendation = ShippingRuleRecommendation & {
  key: string;
  orderId?: string;
  message?: string;
};

export async function recommendForOrders(
  items: { key?: string; orderId?: string; orderNo?: string; province?: string; weightKg?: number }[],
): Promise<OrderShippingRecommendation[]> {
  const data = await postJSON<{ items: OrderShippingRecommendation[] }>(
    '/api/v1/orders/shipping-recommendations',
    { items },
  );
  return data.items || [];
}

export async function markOrdersPrinted(ids: string[]): Promise<{ marked: number }> {
  return postJSON('/api/v1/orders/print/mark', { ids });
}

import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';
import { responseErrorMessage } from '@/utils/httpErrorCopy';
import { fetchWithSessionGuard } from '@/utils/sessionGuard';
import type { OrderInventoryEffectRow, PaginatedInventory } from '@/services/inventory';

export type OrderShipmentRow = {
  id: string;
  orderId: string;
  carrier: string;
  carrierId?: string;
  carrierCode?: string;
  trackingNo: string;
  trackingUrl?: string;
  status: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type OrderItemRow = {
  id: string;
  orderId: string;
  productId?: string;
  productSkuId?: string;
  externalItemId?: string;
  externalSkuId?: string;
  sellerSku?: string;
  productTitle: string;
  skuName?: string;
  skuCode?: string;
  quantity: number;
  unitPrice: number;
  totalPrice: number;
  imageUrl?: string;
  attrs?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type OrderShopSummary = {
  id: string;
  platform: string;
  shopName: string;
  shopCode?: string;
  status: string;
  authStatus: string;
};

/** Order inventory flags from backend `inventory_summary` projection. */
export type OrderInventorySummary = {
  hasDeductionSuccess: boolean;
  hasRestoreSuccess: boolean;
  fullyRestored: boolean;
};

/** GET /orders/:id response (flattened header + nested children) */
export type OrderDetailDTO = {
  id: string;
  tenantId: number;
  platform: string;
  shopId?: string;
  shopSummary?: OrderShopSummary | null;
  externalOrderId?: string;
  orderNo: string;
  customerName: string;
  customerEmail?: string;
  customerPhone?: string;
  status: string;
  reviewStatus?: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  paidAt?: string;
  orderedAt?: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  items: OrderItemRow[];
  shipments: OrderShipmentRow[];
  inventorySummary?: OrderInventorySummary | null;
  plannedCarrierCode?: string;
  plannedCarrierName?: string;
  plannedCarrierMode?: string;
  plannedCarrierRule?: string;
  plannedCarrierAt?: string;
  assignedWarehouseId?: string;
  assignedWarehouseName?: string;
  assignedWarehouseStrategy?: string;
  warehouseAssignedAt?: string;
};

export type OrderListRow = {
  id: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  shopPlatform?: string;
  orderNo: string;
  customerName: string;
  status: string;
  reviewStatus?: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  itemCount?: number;
  skuMatchStatus?: string;
  skuMatchedCount?: number;
  skuTotalCount?: number;
  inventoryDeductStatus?: string;
  syncStatus?: string;
  openExceptionCount?: number;
  detailUrl?: string;
  orderedAt?: string;
  createdAt: string;
  updatedAt?: string;
  latestShipmentStatus?: string;
  waybillPrintedAt?: string;
  externalOrderId?: string;
};

export async function queryOrders(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  shopId?: string;
  orderNo?: string;
  customerName?: string;
  keyword?: string;
  status?: string;
  paymentStatus?: string;
  fulfillmentStatus?: string;
  skuMatchStatus?: string;
  inventoryDeductStatus?: string;
  syncStatus?: string;
  hasException?: boolean;
  hasPurchase?: '0' | '1';
  start?: string;
  end?: string;
}): Promise<{
  list: OrderListRow[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/orders', params);
}

export async function createOrder(payload: Record<string, unknown>): Promise<OrderDetailDTO> {
  return postJSON('/api/v1/orders', payload);
}

export type OrderImportRowResult = {
  orderNo: string;
  status: 'created' | 'skipped_duplicate' | 'failed' | string;
  orderId?: string;
  error?: string;
  itemsTotal: number;
  itemsMatched: number;
};

export type OrderImportSummary = {
  total: number;
  created: number;
  duplicate: number;
  failed: number;
  results: OrderImportRowResult[];
};

export async function importOrders(payload: {
  orders: Record<string, unknown>[];
  matchSkus?: boolean;
}): Promise<OrderImportSummary> {
  return postJSON('/api/v1/orders/import', payload);
}

export async function downloadOrdersShippingCsv(ids: string[]) {
  const resp = await fetchWithSessionGuard(
    `/api/v1/orders/shipping-list/export.csv?ids=${encodeURIComponent(ids.join(','))}`,
  );
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `shipping-list-${ids.length}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export type SalesAmount = {
  currency: string;
  amount: number;
  orders: number;
  /** 折算为本位币的金额；未配置汇率时缺省（未折算） */
  baseAmount?: number;
};

export type SalesWindowStats = {
  key: string;
  orderCount: number;
  paidCount: number;
  shippedCount: number;
  paidAmounts: SalesAmount[];
  /** 已折算入本位币的已付款销售额合计 */
  paidAmountBase: number;
  /** 缺少汇率、未折算入合计的币种 */
  unconvertedCurrencies?: string[];
};

export type SalesStatsDTO = {
  generatedAt: string;
  baseCurrency: string;
  windows: SalesWindowStats[];
};

export async function fetchOrderSalesStats(): Promise<SalesStatsDTO> {
  return getJSON('/api/v1/orders/stats/sales');
}

export type DailyStat = {
  date: string;
  orderCount: number;
  paidCount: number;
  shippedCount: number;
  paidAmounts: SalesAmount[];
  /** 已折算入本位币的已付款销售额合计 */
  paidAmountBase: number;
  /** 缺少汇率、未折算入合计的币种 */
  unconvertedCurrencies?: string[];
};

export type DailyStatsDTO = {
  generatedAt: string;
  days: number;
  baseCurrency: string;
  items: DailyStat[];
};

export async function fetchOrderDailyStats(days = 30): Promise<DailyStatsDTO> {
  return getJSON(`/api/v1/orders/stats/daily?days=${days}`);
}

export async function downloadDailyReportCsv(days: number) {
  const resp = await fetchWithSessionGuard(`/api/v1/orders/stats/daily/export.csv?days=${days}`);
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `daily-report-${days}d.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export async function getOrder(id: string): Promise<OrderDetailDTO> {
  return getJSON(`/api/v1/orders/${id}`);
}

export async function updateOrder(id: string, payload: Record<string, unknown>): Promise<OrderDetailDTO> {
  return putJSON(`/api/v1/orders/${id}`, payload);
}

export async function deleteOrder(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${id}`);
}

export async function createOrderItem(orderId: string, payload: Record<string, unknown>): Promise<OrderItemRow> {
  return postJSON(`/api/v1/orders/${orderId}/items`, payload);
}

export async function updateOrderItem(
  orderId: string,
  itemId: string,
  payload: Record<string, unknown>,
): Promise<OrderItemRow> {
  return putJSON(`/api/v1/orders/${orderId}/items/${itemId}`, payload);
}

export async function deleteOrderItem(orderId: string, itemId: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/items/${itemId}`);
}

export async function createOrderShipment(orderId: string, payload: Record<string, unknown>): Promise<OrderShipmentRow> {
  return postJSON(`/api/v1/orders/${orderId}/shipments`, payload);
}

export async function updateOrderShipment(
  orderId: string,
  shipmentId: string,
  payload: Record<string, unknown>,
): Promise<OrderShipmentRow> {
  return putJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`, payload);
}

export async function deleteOrderShipment(orderId: string, shipmentId: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`);
}

export type BatchShipmentLineResult = {
  key: string;
  orderId?: string;
  ok: boolean;
  status?: string;
  message?: string;
  /** 仅成功行返回：该订单是否已有成功的库存扣减（发货本身不扣库存） */
  inventoryDeducted?: boolean;
};

export type BatchShipmentsResult = {
  succeeded: number;
  failed: number;
  results: BatchShipmentLineResult[];
};

export async function batchCreateOrderShipments(
  items: { orderNo: string; trackingNo: string; carrier?: string; carrierCode?: string }[],
  defaultCarrierCode?: string,
): Promise<BatchShipmentsResult> {
  return postJSON('/api/v1/orders/shipments/batch', { items, defaultCarrierCode });
}

export type PrintSheetItem = {
  productTitle: string;
  skuName?: string;
  skuCode?: string;
  sellerSku?: string;
  quantity: number;
};

export type PrintSheetShipment = {
  carrier: string;
  carrierCode?: string;
  trackingNo?: string;
  status: string;
};

export type PrintSheet = {
  orderId: string;
  orderNo: string;
  platform: string;
  shopName?: string;
  customerName: string;
  customerPhone?: string;
  customerEmail?: string;
  remark?: string;
  orderedAt?: string;
  items: PrintSheetItem[];
  shipments: PrintSheetShipment[];
};

export async function getOrderPrintSheets(ids: string[]): Promise<PrintSheet[]> {
  const data = await getWithParams<{ items: PrintSheet[] }>('/api/v1/orders/print/sheets', {
    ids: ids.join(','),
  });
  return data.items || [];
}

export async function getOrderPrintSheetsWithTemplate(
  ids: string[],
  templateId?: string,
): Promise<{ items: PrintSheet[]; template?: import('@/services/waybill').WaybillTemplateRow | null }> {
  const params: Record<string, string> = { ids: ids.join(',') };
  if (templateId) params.templateId = templateId;
  const data = await getWithParams<{
    items: PrintSheet[];
    template?: import('@/services/waybill').WaybillTemplateRow | null;
  }>('/api/v1/orders/print/sheets', params);
  return { items: data.items || [], template: data.template };
}

export async function refreshShipmentTracking(
  orderId: string,
  shipmentId: string,
): Promise<{ provider: string; supported: boolean; message: string; shipment: OrderShipmentRow }> {
  return postJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}/refresh-tracking`, {});
}

export async function deductOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean; warehouseId?: string },
): Promise<{ order: OrderDetailDTO; inventoryDeduction: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/deduct-inventory`, body ?? {});
}

export async function restoreOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean; reason?: string },
): Promise<{ order: OrderDetailDTO; inventoryRestoration: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/restore-inventory`, body ?? {});
}

export async function getOrderInventoryEffects(
  orderId: string,
  params?: { page?: number; pageSize?: number },
): Promise<{ list: OrderInventoryEffectRow[]; pagination: PaginatedInventory<OrderInventoryEffectRow>['pagination'] }> {
  return getWithParams(`/api/v1/orders/${orderId}/inventory-effects`, params ?? {});
}

export type OrderSkuMatchRow = {
  id?: string;
  orderId?: string;
  orderItemId?: string;
  platform?: string;
  externalSkuId?: string;
  sellerSku?: string;
  skuCode?: string;
  matchStatus?: string;
  matchType?: string;
  confidence?: number;
  reason?: string;
  productId?: string;
  productSkuId?: string;
  productTitle?: string;
  localSkuCode?: string;
  externalOrderId?: string;
  candidateSkus?: Array<{
    productSkuId: string;
    productId: string;
    skuCode: string;
    skuName?: string;
    productTitle?: string;
  }>;
};

export async function getOrderSKUMatches(orderId: string): Promise<{ items: OrderSkuMatchRow[] }> {
  return getJSON(`/api/v1/orders/${orderId}/sku-matches`);
}

export async function matchOrderSKUs(
  orderId: string,
  body?: { overwrite?: boolean; force?: boolean },
): Promise<{ summary: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/match-skus`, body ?? {});
}

export async function bindOrderItemSku(
  itemId: string,
  body: {
    productSkuId: string;
    deductInventory?: boolean;
    syncInventory?: boolean;
    candidateConfidence?: number | null;
    candidateSource?: string;
  },
): Promise<{ item: OrderItemRow; inventoryDeduction?: Record<string, unknown> }> {
  return postJSON(`/api/v1/order-items/${itemId}/bind-sku`, body);
}

export type OrderSkuMatchListRow = OrderSkuMatchRow & {
  shopName?: string;
  orderNo?: string;
  productTitle?: string;
  localSkuCode?: string;
  createdAt?: string;
  updatedAt?: string;
};

export async function queryOrderSkuMatches(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  shopId?: string;
  matchStatus?: string;
  matchType?: string;
  orderId?: string;
  productSkuId?: string;
  start?: string;
  end?: string;
}): Promise<{
  list: OrderSkuMatchListRow[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/order-item-sku-matches', params);
}

import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { getJSON, getWithParams, postJSON } from './request';

export type PurchaseOrderItem = {
  id: string;
  purchaseOrderId: string;
  salesOrderId?: string;
  localSkuId: string;
  sourceSkuId: string;
  externalOfferId?: string;
  externalSkuId?: string;
  sourceUrl?: string;
  productTitle?: string;
  skuName?: string;
  quantity: number;
  expectedPrice?: number;
  actualPrice?: number;
};

export type PurchaseOrderEvent = {
  id: number;
  fromStatus?: string;
  toStatus: string;
  source: string;
  createdAt: string;
};

export type PurchaseLogistics = {
  id: string;
  trackingNo?: string;
  carrier?: string;
  status?: string;
  inboundAt?: string;
};

export type PurchaseOrder = {
  id: string;
  supplierId: string;
  supplierName: string;
  sourcePlatform: string;
  externalOrderId?: string;
  status: string;
  totalAmount: number;
  currency: string;
  payStatus: string;
  payChannel?: string;
  paidAt?: string;
  idempotencyKey: string;
  errorMessage?: string;
  retryCount: number;
  confirmedAt?: string;
  createdAt: string;
  items?: PurchaseOrderItem[];
  events?: PurchaseOrderEvent[];
  logistics?: PurchaseLogistics[];
};

export type GenerateResult = {
  orders: PurchaseOrder[];
  blockers?: {
    orderId: string;
    localSkuId?: string;
    skuName?: string;
    code: string;
    message: string;
  }[];
};

export async function generatePurchaseOrders(body: {
  orderIds: string[];
  idempotencyKey?: string;
}) {
  return postJSON<GenerateResult>('/api/v1/procurement/orders/generate', body);
}

export async function fetchPurchaseOrders(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  keyword?: string;
}) {
  return getWithParams<{ items: PurchaseOrder[]; total: number; page: number; pageSize: number }>(
    '/api/v1/procurement/orders',
    { page: params.page, pageSize: params.pageSize, status: params.status, keyword: params.keyword },
  );
}

export async function fetchPurchaseOrder(id: string) {
  return getJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}`);
}

export async function submitPurchaseOrder(id: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/submit`);
}

export async function confirmPurchaseOrder(id: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/confirm`);
}

export async function retryPurchaseOrder(id: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/retry`);
}

export async function cancelPurchaseOrder(id: string, reason?: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/cancel`, { reason });
}

export async function markPurchaseOrderPlaced(id: string, externalOrderId: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/mark-placed`, {
    externalOrderId,
  });
}

export async function markPurchaseOrderPaid(id: string, payChannel?: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/mark-paid`, { payChannel });
}

export async function fillPurchaseLogistics(id: string, trackingNo: string, carrier?: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/logistics`, {
    trackingNo,
    carrier,
  });
}

export async function markPurchaseOrderDelivered(id: string) {
  return postJSON<PurchaseOrder>(`/api/v1/procurement/orders/${id}/mark-delivered`);
}

export async function downloadPurchaseOrderCsv(id: string) {
  const token = localStorage.getItem(AUTH_TOKEN_KEY);
  const resp = await fetch(`/api/v1/procurement/orders/${id}/export.csv`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!resp.ok) {
    throw new Error(`export failed: ${resp.status}`);
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `purchase-list-${id.slice(0, 8)}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

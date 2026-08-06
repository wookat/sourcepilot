import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

// 订单标签（R135）：租户级标签管理 + 订单打标/去标/批量打标
export type OrderTagRow = {
  id: string;
  tenantId: number;
  name: string;
  color: string;
  createdAt: string;
  updatedAt: string;
};

export type OrderTagBrief = {
  id: string;
  name: string;
  color: string;
};

export type OrderTagBody = {
  name?: string;
  color?: string;
};

// 与后端 validOrderTagColors 对齐（Ant Design 预设色）
export const ORDER_TAG_COLORS = [
  'blue',
  'green',
  'red',
  'orange',
  'gold',
  'purple',
  'cyan',
  'magenta',
  'geekblue',
  'volcano',
  'lime',
  'default',
] as const;

export async function listOrderTags(): Promise<OrderTagRow[]> {
  const data = await getWithParams<{ items: OrderTagRow[] }>('/api/v1/order-tags', {});
  return data.items || [];
}

export async function createOrderTag(body: OrderTagBody): Promise<OrderTagRow> {
  return postJSON('/api/v1/order-tags', body);
}

export async function updateOrderTag(id: string, body: OrderTagBody): Promise<OrderTagRow> {
  return putJSON(`/api/v1/order-tags/${id}`, body);
}

export async function deleteOrderTag(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/order-tags/${id}`);
}

export async function addOrderTags(orderId: string, tagIds: string[]): Promise<OrderTagBrief[]> {
  const data = await postJSON<{ tags: OrderTagBrief[] }>(`/api/v1/orders/${orderId}/tags`, {
    tagIds,
  });
  return data.tags || [];
}

export async function removeOrderTag(orderId: string, tagId: string): Promise<OrderTagBrief[]> {
  const data = await deleteJSON<{ tags: OrderTagBrief[] }>(
    `/api/v1/orders/${orderId}/tags/${tagId}`,
  );
  return data.tags || [];
}

export type BatchOrderTagResult = {
  orders: number;
  tags: number;
  applied: number;
  removed: number;
};

export async function batchTagOrders(body: {
  orderIds: string[];
  tagIds: string[];
  action?: 'add' | 'remove';
}): Promise<BatchOrderTagResult> {
  return postJSON('/api/v1/orders/batch-tags', body);
}

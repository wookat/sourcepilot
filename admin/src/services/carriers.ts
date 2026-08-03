import { deleteJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type CarrierRow = {
  id: string;
  tenantId: number;
  code: string;
  name: string;
  enabled: boolean;
  isPreset: boolean;
  trackingUrlTemplate?: string;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export type CarrierListQuery = {
  enabled?: boolean;
  keyword?: string;
};

export async function listCarriers(query: CarrierListQuery = {}): Promise<CarrierRow[]> {
  const params: Record<string, string> = {};
  if (query.enabled) params.enabled = '1';
  if (query.keyword) params.keyword = query.keyword;
  const data = await getWithParams<{ items: CarrierRow[] }>('/api/v1/carriers', params);
  return data.items || [];
}

export type CreateCarrierBody = {
  code: string;
  name: string;
  trackingUrlTemplate?: string;
  sortOrder?: number;
};

export async function createCarrier(body: CreateCarrierBody): Promise<CarrierRow> {
  return postJSON('/api/v1/carriers', body);
}

export type UpdateCarrierBody = {
  name?: string;
  enabled?: boolean;
  trackingUrlTemplate?: string;
  sortOrder?: number;
};

export async function updateCarrier(id: string, body: UpdateCarrierBody): Promise<CarrierRow> {
  return putJSON(`/api/v1/carriers/${id}`, body);
}

export async function deleteCarrier(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/carriers/${id}`);
}

import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from './request';

export type Supplier = {
  id: string;
  platform: string;
  externalId?: string;
  name: string;
  rating?: number;
  contact?: Record<string, unknown>;
  remark?: string;
  status: string;
  createdAt: string;
  updatedAt?: string;
};

export type ProductSourceSKU = {
  id: string;
  productSourceId: string;
  localSkuId: string;
  externalSkuId?: string;
  externalSpec?: Record<string, unknown>;
  currentPrice?: number;
  currency: string;
  currentStock?: number;
  status: string;
};

export type ProductSource = {
  id: string;
  productId: string;
  supplierId: string;
  sourceUrl?: string;
  sourceOfferId?: string;
  priority: number;
  isPrimary: boolean;
  locked: boolean;
  status: string;
  moq?: number;
  leadTimeDays?: number;
  lastCheckedAt?: string;
  supplier?: Supplier;
  skus?: ProductSourceSKU[];
  createdAt: string;
};

export type SourcePriceHistoryRow = {
  id: number;
  sourceSkuId: string;
  price: number;
  stock?: number;
  capturedAt: string;
  captureSource: string;
};

export type SourceSwitchEvent = {
  id: string;
  productId: string;
  fromSourceId?: string;
  toSourceId: string;
  reason: string;
  mode: string;
  operator?: string;
  createdAt: string;
};

export type RefreshResult = {
  productId: string;
  refreshed: number;
  alerts?: string[];
  switched?: ProductSource;
  sources: ProductSource[];
};

export async function fetchSuppliers(params: {
  page?: number;
  pageSize?: number;
  keyword?: string;
  status?: string;
}) {
  return getWithParams<{ items: Supplier[]; total: number; page: number; pageSize: number }>(
    '/api/v1/suppliers',
    { page: params.page, pageSize: params.pageSize, keyword: params.keyword, status: params.status },
  );
}

export async function createSupplier(body: Partial<Supplier>) {
  return postJSON<Supplier>('/api/v1/suppliers', body);
}

export async function updateSupplier(id: string, body: Partial<Supplier>) {
  return putJSON<Supplier, Partial<Supplier>>(`/api/v1/suppliers/${id}`, body);
}

export async function deleteSupplier(id: string) {
  return deleteJSON<{ deleted: boolean }>(`/api/v1/suppliers/${id}`);
}

export async function fetchProductSources(productId: string) {
  return getJSON<{ items: ProductSource[] }>(`/api/v1/products/${productId}/sources`);
}

export async function bindProductSource(
  productId: string,
  body: {
    supplierId?: string;
    supplierName?: string;
    sourceUrl?: string;
    sourceOfferId?: string;
    priority?: number;
    moq?: number;
    leadTimeDays?: number;
    setPrimary?: boolean;
  },
) {
  return postJSON<ProductSource>(`/api/v1/products/${productId}/sources`, body);
}

export async function updateProductSource(
  id: string,
  body: {
    priority?: number;
    locked?: boolean;
    status?: string;
    moq?: number;
    leadTimeDays?: number;
    sourceUrl?: string;
  },
) {
  return putJSON<ProductSource, typeof body>(`/api/v1/product-sources/${id}`, body);
}

export async function setPrimarySource(id: string) {
  return postJSON<ProductSource>(`/api/v1/product-sources/${id}/set-primary`);
}

export async function saveSkuMappings(
  sourceId: string,
  mappings: {
    localSkuId: string;
    externalSkuId?: string;
    currentPrice?: number;
    currentStock?: number;
  }[],
) {
  return postJSON<{ items: ProductSourceSKU[] }>(
    `/api/v1/product-sources/${sourceId}/sku-mappings`,
    { mappings },
  );
}

export async function deleteSkuMapping(sourceSkuId: string) {
  return deleteJSON<{ deleted: boolean }>(`/api/v1/product-source-skus/${sourceSkuId}`);
}

export async function fetchPriceHistory(sourceSkuId: string, days = 90) {
  return getWithParams<{ items: SourcePriceHistoryRow[] }>(
    `/api/v1/product-source-skus/${sourceSkuId}/price-history`,
    { days },
  );
}

export async function refreshProductSources(productId: string) {
  return postJSON<RefreshResult>(`/api/v1/products/${productId}/sources/refresh`);
}

export async function fetchSwitchEvents(params: {
  productId?: string;
  page?: number;
  pageSize?: number;
}) {
  return getWithParams<{ items: SourceSwitchEvent[]; total: number }>(
    '/api/v1/source-switch-events',
    { productId: params.productId, page: params.page, pageSize: params.pageSize },
  );
}

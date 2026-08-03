import { getJSON, postJSON, putJSON } from '@/services/request';

export type PlatformTenantStatus = 'active' | 'disabled';

export type PlatformTenantRow = {
  id: number;
  name: string;
  status?: PlatformTenantStatus;
  adminCount: number;
  createdAt?: string;
};

export type CreateTenantBody = {
  name: string;
  adminEmail: string;
  adminPassword: string;
};

export type CreateTenantResult = {
  tenant: PlatformTenantRow;
  adminId: string;
  adminEmail: string;
};

export async function fetchPlatformTenants() {
  return getJSON<{ list: PlatformTenantRow[] }>('/api/v1/platform/tenants');
}

export async function createPlatformTenant(body: CreateTenantBody) {
  return postJSON<CreateTenantResult, CreateTenantBody>('/api/v1/platform/tenants', body);
}

export async function renamePlatformTenant(id: number, name: string) {
  return putJSON<PlatformTenantRow, { name: string }>(`/api/v1/platform/tenants/${id}`, { name });
}

export type PurgeTaskStatus = 'pending' | 'running' | 'succeeded' | 'failed';

export type PurgeReport = {
  tables: Record<string, number>;
  total: number;
  verifiedAt: string;
};

export type PurgeTask = {
  id: string;
  tenantId: number;
  tenantName: string;
  status: PurgeTaskStatus;
  error?: string;
  report?: PurgeReport;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
};

export async function purgePlatformTenant(id: number, confirmName: string) {
  return postJSON<PurgeTask, { confirmName: string }>(`/api/v1/platform/tenants/${id}/purge`, {
    confirmName,
  });
}

export async function fetchPlatformTenantPurge(id: number) {
  return getJSON<PurgeTask>(`/api/v1/platform/tenants/${id}/purge`);
}

export async function disablePlatformTenant(id: number) {
  return postJSON<PlatformTenantRow>(`/api/v1/platform/tenants/${id}/disable`);
}

export async function enablePlatformTenant(id: number) {
  return postJSON<PlatformTenantRow>(`/api/v1/platform/tenants/${id}/enable`);
}

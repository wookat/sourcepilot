import { getJSON, postJSON } from '@/services/request';

export type PlatformTenantRow = {
  id: number;
  name: string;
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

import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { request } from '@umijs/max';

export type BackupJob = {
  id: string;
  backupId: string;
  backupType: string;
  environment: string;
  status: string;
  verificationStatus: string;
  storageProvider: string;
  encrypted: boolean;
  artifactSize?: number;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type RestoreJob = {
  id: string;
  restoreId: string;
  backupId: string;
  targetEnvironment: string;
  status: string;
  safetyGateStatus: string;
  validationStatus?: string;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type ReleaseRun = {
  id: string;
  releaseId: string;
  version: string;
  gitCommit?: string;
  environment: string;
  strategy: string;
  state: string;
  preBackupId?: string;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type DRStatus = {
  status: string;
  rpoTarget: string;
  rtoTarget: string;
  realProductionDRVerification: string;
  realProductionBackupVerification: string;
  realPITRDrill: string;
  lastDrill?: Record<string, unknown>;
};

export type OpsCheck = {
  key: string;
  status: 'passed' | 'failed' | 'skipped' | 'not_implemented';
  message?: string;
};

export type BackupVerification = {
  backupId: string;
  status: string;
  checksumPassed: boolean;
  manifestPassed: boolean;
  encryptionPassed: boolean;
  pgRestoreListed: boolean;
  details?: { checks?: OpsCheck[] };
  errorSummary?: string;
  verifiedAt: string;
};

export type RestoreValidation = {
  restoreId: string;
  status: string;
  details?: { checks?: OpsCheck[] };
  errorSummary?: string;
  validatedAt: string;
};

export type DRDrill = {
  drillId: string;
  status: string;
  backupId?: string;
  reportJson?: { checks?: OpsCheck[] };
  errorSummary?: string;
};

type ListResult<T> = {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
};

export async function fetchBackups(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<BackupJob> }>('/api/v1/ops/backups', { method: 'GET', params });
}

export async function createBackup(data?: { dryRun?: boolean; reason?: string }) {
  return request<{ data: BackupJob }>('/api/v1/ops/backups', { method: 'POST', data });
}

export async function verifyBackup(id: string) {
  return request<{ data: BackupVerification }>(`/api/v1/ops/backups/${id}/verify`, {
    method: 'POST',
  });
}

/** 下载已通过校验的 completed 备份文件（流式；需要 backup.download 权限）。 */
export async function downloadBackup(id: string, fallbackName?: string) {
  const token = localStorage.getItem(AUTH_TOKEN_KEY);
  const res = await fetch(`/api/v1/ops/backups/${id}/download`, {
    method: 'GET',
    credentials: 'include',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) {
    let msg = `下载失败（HTTP ${res.status}）`;
    try {
      const body = (await res.json()) as { message?: string };
      if (body?.message) msg = body.message;
    } catch {
      // 非 JSON 响应时使用默认提示
    }
    throw new Error(msg);
  }
  const disposition = res.headers.get('Content-Disposition') || '';
  const match = /filename="?([^";]+)"?/.exec(disposition);
  const filename = match?.[1] || fallbackName || `${id}.dump`;
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export async function holdBackup(id: string, reason: string) {
  return request(`/api/v1/ops/backups/${id}/hold`, {
    method: 'POST',
    data: { holdType: 'manual_hold', reason },
  });
}

export async function fetchRestores(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<RestoreJob> }>('/api/v1/ops/restores', { method: 'GET', params });
}

export async function createRestore(data: {
  backupId: string;
  targetEnvironment: string;
  targetDatabaseName: string;
  targetIsIsolated: boolean;
  operatorReauthenticated: boolean;
  highRiskConfirmed: boolean;
}) {
  return request<{ data: RestoreJob }>('/api/v1/ops/restores', { method: 'POST', data });
}

export async function verifyRestore(id: string) {
  return request<{ data: RestoreValidation }>(`/api/v1/ops/restores/${id}/verify`, {
    method: 'POST',
  });
}

export async function fetchReleases(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<ReleaseRun> }>('/api/v1/ops/releases', { method: 'GET', params });
}

export async function createRelease(data: { version: string; gitCommit?: string }) {
  return request<{ data: ReleaseRun }>('/api/v1/ops/releases', { method: 'POST', data });
}

export async function executeRelease(id: string) {
  return request(`/api/v1/ops/releases/${id}/execute`, { method: 'POST' });
}

export async function rollbackRelease(id: string, reason: string) {
  return request(`/api/v1/ops/releases/${id}/rollback`, { method: 'POST', data: { reason } });
}

export async function fetchDRStatus() {
  return request<{ data: DRStatus }>('/api/v1/ops/dr/status', { method: 'GET' });
}

export async function createDRDrill(data: {
  drillType: string;
  backupId?: string;
  restoreId?: string;
  releaseId?: string;
  confirmedIsolated: boolean;
}) {
  return request<{ data: DRDrill }>('/api/v1/ops/dr/drills', { method: 'POST', data });
}

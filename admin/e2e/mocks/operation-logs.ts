import { ok, paged, type ApiEnvelope } from './envelope';

export type OperationLogMockRow = {
  id: string;
  username: string;
  action: string;
  resource: string;
  resourceId?: string;
  method: string;
  path: string;
  requestId: string;
  status: string;
  message?: string;
  createdAt: string;
};

const rows: OperationLogMockRow[] = [
  {
    id: 'e2e-oplog-1',
    username: 'e2e-admin',
    action: 'login',
    resource: 'auth',
    method: 'POST',
    path: '/api/v1/auth/login',
    requestId: 'e2e-req-1',
    status: 'success',
    createdAt: '2026-08-01T10:00:00Z',
  },
  {
    id: 'e2e-oplog-2',
    username: 'e2e-operator',
    action: 'settings_update',
    resource: 'settings',
    resourceId: 'ai',
    method: 'PUT',
    path: '/api/v1/settings',
    requestId: 'e2e-req-2',
    status: 'success',
    message: '更新 AI 设置',
    createdAt: '2026-08-01T11:00:00Z',
  },
  {
    id: 'e2e-oplog-4',
    username: 'e2e-admin',
    action: 'order_automation.execute',
    resource: 'order_automation',
    resourceId: 'e2e-auto-rule-1',
    method: 'POST',
    path: '/api/v1/orders/automation/execute',
    requestId: 'e2e-req-4',
    status: 'success',
    message: '订单自动化规则执行',
    createdAt: '2026-08-01T12:30:00Z',
  },
  {
    id: 'e2e-oplog-3',
    username: 'e2e-operator',
    action: 'procurement.status.update',
    resource: 'procurement',
    resourceId: 'e2e-po-1',
    method: 'POST',
    path: '/api/v1/procurements/:id/status',
    requestId: 'e2e-req-3',
    status: 'failed',
    message: '状态流转失败',
    createdAt: '2026-08-01T12:00:00Z',
  },
];

export function operationLogsResponse(path: string, search: URLSearchParams): ApiEnvelope<unknown> | null {
  if (path !== '/api/v1/operation-logs') return null;
  let items = rows;
  const action = search.get('action');
  const username = search.get('username');
  const resource = search.get('resource');
  if (action) items = items.filter((r) => r.action === action);
  if (username) items = items.filter((r) => r.username.toLowerCase().includes(username.toLowerCase()));
  if (resource) items = items.filter((r) => r.resource === resource);
  const page = Number(search.get('page') || '1') || 1;
  const pageSize = Number(search.get('pageSize') || '20') || 20;
  const startIdx = (page - 1) * pageSize;
  return ok(paged(items.slice(startIdx, startIdx + pageSize), items.length, page, pageSize));
}

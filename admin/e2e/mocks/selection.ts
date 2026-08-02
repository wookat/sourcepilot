import { ok } from './envelope';

export const E2E_SELECTION_TASK_ID = '11111111-2222-3333-4444-555555555555';
export const E2E_SELECTION_FAILED_TASK_ID = '22222222-3333-4444-5555-666666666666';
export const E2E_SELECTION_TASK_ERROR = '采集服务不可用：collector timeout';
export const E2E_SELECTION_CANDIDATE_ERROR = '1688 同款匹配失败：source match provider unavailable';

const task = {
  id: E2E_SELECTION_TASK_ID,
  name: 'E2E 选品任务',
  targetPlatform: 'tiktok',
  targetCountry: 'US',
  status: 'success',
  candidateCount: 1,
  scoredCount: 1,
  failedCount: 0,
  createdAt: '2026-01-01T00:00:00Z',
};

const failedTask = {
  id: E2E_SELECTION_FAILED_TASK_ID,
  name: 'E2E 失败任务',
  targetPlatform: 'tiktok',
  targetCountry: 'US',
  status: 'failed',
  errorMessage: E2E_SELECTION_TASK_ERROR,
  candidateCount: 1,
  scoredCount: 0,
  failedCount: 1,
  createdAt: '2026-01-02T00:00:00Z',
};

const failedCandidate = {
  candidate: {
    id: 'bbbbbbbb-cccc-dddd-eeee-ffffffffffff',
    taskId: E2E_SELECTION_FAILED_TASK_ID,
    title: 'E2E 失败候选',
    status: 'failed',
    errorMessage: E2E_SELECTION_CANDIDATE_ERROR,
  },
};

const fallbackCandidate = {
  candidate: {
    id: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
    taskId: E2E_SELECTION_TASK_ID,
    title: 'E2E 规则兜底候选',
    status: 'scored',
    marketPrice: 12.99,
    marketCurrency: 'USD',
  },
  evaluation: {
    id: 'ffffffff-1111-2222-3333-444444444444',
    candidateId: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
    estProfit: 5.2,
    estMarginPercent: 40.1,
    aiScore: 62,
    aiReasons: {
      summary: '规则兜底评分：预估利润率 40.10%，月销 0',
      risks: [],
      fallback: true,
    },
    decision: 'pending',
  },
};

export const integrationsOverviewAIUnconfigured = {
  ai: { configured: false },
  storage: { configured: false },
  mail: { configured: false },
};

export function selectionResponse(path: string, statusFilter?: string | null) {
  if (path === '/api/v1/selection/tasks') {
    const items = [task, failedTask].filter((t) => !statusFilter || t.status === statusFilter);
    return ok({ items, total: items.length, page: 1, pageSize: 20 });
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_TASK_ID}`) {
    return ok(task);
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_TASK_ID}/candidates`) {
    return ok([fallbackCandidate]);
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}`) {
    return ok(failedTask);
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}/candidates`) {
    return ok([failedCandidate]);
  }
  return null;
}

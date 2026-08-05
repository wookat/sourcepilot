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

export const E2E_CANDIDATE_A_ID = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
export const E2E_CANDIDATE_B_ID = 'cccccccc-dddd-eeee-ffff-000000000000';

const insightsCandidateB = {
  candidate: {
    id: E2E_CANDIDATE_B_ID,
    taskId: E2E_SELECTION_TASK_ID,
    title: 'E2E 已采集候选',
    category: 'e2e-家居',
    status: 'scored',
    marketPrice: 19.9,
    marketCurrency: 'USD',
    marketSales30d: 320,
    sourceUrl: 'https://market.e2e.local/item/e2e-1',
  },
  evaluation: {
    id: 'ffffffff-5555-6666-7777-888888888888',
    candidateId: E2E_CANDIDATE_B_ID,
    purchaseCost: 22.5,
    shippingCost: 6,
    commissionFee: 1.6,
    landedCost: 7.1,
    estProfit: 12.8,
    estMarginPercent: 64.3,
    aiScore: 81,
    aiModel: 'e2e-mock',
    aiReasons: { summary: 'E2E 数据面板评分摘要', sellingPoints: ['轻小件'], risks: ['同款竞争'] },
    decision: 'pending',
  },
  bestMatch: {
    id: 'eeeeeeee-1111-2222-3333-444444444444',
    candidateId: E2E_CANDIDATE_B_ID,
    sourcePlatform: '1688',
    minPrice: 20.5,
    maxPrice: 24.0,
    currency: 'CNY',
    supplierName: 'e2e-义乌供应商',
  },
};

export function selectionInsightsResponse(path: string, searchParams: URLSearchParams) {
  if (path === `/api/v1/selection/candidates/${E2E_CANDIDATE_B_ID}/insights`) {
    return ok({
      candidate: insightsCandidateB.candidate,
      evaluation: insightsCandidateB.evaluation,
      bestMatch: insightsCandidateB.bestMatch,
      collected: {
        marketPrice: 19.9,
        marketCurrency: 'USD',
        marketSales30d: 320,
        sourcePrice: 21.2,
        sourceCurrency: 'CNY',
        sourceSales: 1350,
        sourceReviewCount: 214,
        sourceCapturedAt: '2026-01-03T08:00:00Z',
        collectCount: 4,
      },
      benchmark: {
        category: 'e2e-家居',
        productCount: 3,
        avgDraftMarginPercent: 41.5,
        windowDays: 90,
        orderCount: 6,
        soldQty: 12,
        revenue: 1520.4,
        grossProfit: 634.2,
        grossMarginPercent: 41.7,
      },
      external: [
        {
          name: 'tiktok_hotlist',
          displayName: 'TikTok 热销榜',
          configured: false,
          message: '未配置平台开放接口凭证',
        },
        {
          name: 'shopee_hotlist',
          displayName: 'Shopee 热销榜',
          configured: false,
          message: '未配置平台开放接口凭证',
        },
      ],
    });
  }
  if (path === `/api/v1/selection/candidates/${E2E_CANDIDATE_A_ID}/insights`) {
    return ok({
      candidate: fallbackCandidate.candidate,
      evaluation: fallbackCandidate.evaluation,
      collected: { collectCount: 0, marketPrice: 12.99, marketCurrency: 'USD' },
      external: [
        {
          name: 'tiktok_hotlist',
          displayName: 'TikTok 热销榜',
          configured: false,
          message: '未配置平台开放接口凭证',
        },
      ],
    });
  }
  if (path === `/api/v1/selection/candidates/${E2E_CANDIDATE_B_ID}/price-trend`) {
    return ok({
      sourceUrl: 'https://market.e2e.local/item/e2e-1',
      currency: 'CNY',
      points: [
        { capturedAt: '2026-01-01T08:00:00Z', price: 24.8, taskId: E2E_SELECTION_TASK_ID },
        { capturedAt: '2026-01-02T08:00:00Z', price: 23.9, taskId: E2E_SELECTION_TASK_ID },
        { capturedAt: '2026-01-03T08:00:00Z', price: 21.2, taskId: E2E_SELECTION_TASK_ID },
      ],
    });
  }
  if (path === `/api/v1/selection/candidates/${E2E_CANDIDATE_A_ID}/price-trend`) {
    return ok({ points: [] });
  }
  if (path === '/api/v1/selection/compare') {
    const ids = (searchParams.get('ids') || '').split(',').filter(Boolean);
    if (ids.length < 2) return null;
    return ok([
      {
        candidate: insightsCandidateB.candidate,
        evaluation: insightsCandidateB.evaluation,
        bestMatch: insightsCandidateB.bestMatch,
        supply: { ready: true, supplierName: 'e2e-义乌供应商', sourceStatus: 'active' },
        banned: { forbiddenCount: 0, warningCount: 0 },
      },
      {
        candidate: fallbackCandidate.candidate,
        evaluation: fallbackCandidate.evaluation,
        supply: { ready: false },
        banned: { forbiddenCount: 1, warningCount: 0, words: ['最强'] },
      },
    ]);
  }
  if (path === '/api/v1/selection/market-sources') {
    return ok([
      {
        name: 'tiktok_hotlist',
        displayName: 'TikTok 热销榜',
        configured: false,
        message: '未配置平台开放接口凭证',
      },
    ]);
  }
  return null;
}

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
    return ok([fallbackCandidate, insightsCandidateB]);
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}`) {
    return ok(failedTask);
  }
  if (path === `/api/v1/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}/candidates`) {
    return ok([failedCandidate]);
  }
  return null;
}

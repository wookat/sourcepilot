import { getJSON, getWithParams, postJSON } from './request';

export type SelectionTaskRow = {
  id: string;
  name: string;
  targetPlatform: string;
  targetCountry: string;
  status: string;
  params?: unknown;
  errorMessage?: string;
  candidateCount: number;
  scoredCount: number;
  failedCount: number;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
};

export type SelectionTaskListResult = {
  items: SelectionTaskRow[];
  total: number;
  page: number;
  pageSize: number;
};

export type SelectionCandidateRow = {
  id: string;
  taskId: string;
  productId?: string;
  title: string;
  imageUrl?: string;
  category?: string;
  sourceUrl?: string;
  marketPlatform?: string;
  marketPrice?: number;
  marketCurrency?: string;
  marketSales30d?: number;
  status: string;
  errorMessage?: string;
};

export type SelectionSourceMatchRow = {
  id: string;
  candidateId: string;
  sourcePlatform: string;
  sourceUrl?: string;
  sourceOfferId?: string;
  matchMethod?: string;
  similarity?: number;
  minPrice?: number;
  maxPrice?: number;
  currency: string;
  moq?: number;
  supplierName?: string;
  supplierRating?: number;
};

export type SelectionEvaluationRow = {
  id: string;
  candidateId: string;
  bestMatchId?: string;
  purchaseCost?: number;
  shippingCost?: number;
  commissionFee?: number;
  exchangeRate?: number;
  landedCost?: number;
  estProfit?: number;
  estMarginPercent?: number;
  aiScore?: number;
  aiReasons?: unknown;
  aiModel?: string;
  decision: string;
  decidedAt?: string;
  draftProductId?: string;
};

export type SelectionCandidateItem = {
  candidate: SelectionCandidateRow;
  evaluation?: SelectionEvaluationRow;
  bestMatch?: SelectionSourceMatchRow;
  matches?: SelectionSourceMatchRow[];
};

export type SelectionTaskParams = {
  exchangeRate?: number;
  commissionPercent?: number;
  logisticsBaseFee?: number;
  logisticsPerKgFee?: number;
  lastMileFee?: number;
  returnRatePercent?: number;
  minMarginPercent?: number;
  sourceMatchProvider?: string;
  targetCurrency?: string;
};

export type CreateSelectionTaskBody = {
  name?: string;
  targetPlatform: string;
  targetCountry?: string;
  items?: {
    title: string;
    imageUrl?: string;
    category?: string;
    sourceUrl?: string;
    marketPrice?: number;
    marketCurrency?: string;
    marketSales30d?: number;
    weightKg?: number;
  }[];
  productIds?: string[];
  keywords?: string[];
  params?: SelectionTaskParams;
};

export async function fetchSelectionTasks(params: {
  page?: number;
  pageSize?: number;
  status?: string;
}) {
  return getWithParams<SelectionTaskListResult>('/api/v1/selection/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
  });
}

export async function fetchSelectionTask(id: string) {
  return getJSON<SelectionTaskRow>(`/api/v1/selection/tasks/${id}`);
}

export async function fetchSelectionCandidates(taskId: string) {
  return getJSON<SelectionCandidateItem[]>(`/api/v1/selection/tasks/${taskId}/candidates`);
}

export async function createSelectionTask(body: CreateSelectionTaskBody) {
  return postJSON<SelectionTaskRow>('/api/v1/selection/tasks', body);
}

export async function retrySelectionTask(id: string) {
  return postJSON<SelectionTaskRow>(`/api/v1/selection/tasks/${id}/retry`, {});
}

export async function decideSelectionCandidate(id: string, decision: 'approved' | 'rejected') {
  return postJSON<SelectionEvaluationRow>(`/api/v1/selection/candidates/${id}/decision`, {
    decision,
  });
}

export type DraftProduct = {
  id: string;
  title: string;
  status: string;
};

export async function selectionCandidateToDraft(id: string) {
  return postJSON<DraftProduct>(`/api/v1/selection/candidates/${id}/to-draft`, {});
}

export type CollectedFacts = {
  marketPrice?: number;
  marketCurrency?: string;
  marketSales30d?: number;
  marketReviewCount?: number;
  sourcePrice?: number;
  sourceCurrency?: string;
  sourceSales?: number;
  sourceReviewCount?: number;
  sourceCapturedAt?: string;
  collectCount: number;
};

export type CategoryBenchmark = {
  category: string;
  productCount: number;
  avgDraftMarginPercent?: number;
  windowDays: number;
  orderCount: number;
  soldQty: number;
  revenue?: number;
  grossProfit?: number;
  grossMarginPercent?: number;
};

export type MarketSourceStatus = {
  name: string;
  displayName: string;
  configured: boolean;
  message?: string;
};

export type CandidateInsights = {
  candidate: SelectionCandidateRow;
  evaluation?: SelectionEvaluationRow;
  bestMatch?: SelectionSourceMatchRow;
  collected: CollectedFacts;
  benchmark?: CategoryBenchmark;
  external: MarketSourceStatus[];
};

export type PriceTrendPoint = {
  capturedAt: string;
  price: number;
  taskId: string;
};

export type PriceTrend = {
  sourceUrl?: string;
  currency?: string;
  points: PriceTrendPoint[];
};

export type SupplyReadiness = {
  ready: boolean;
  supplierName?: string;
  sourceStatus?: string;
};

export type BannedRisk = {
  forbiddenCount: number;
  warningCount: number;
  words?: string[];
};

export type CompareRow = {
  candidate: SelectionCandidateRow;
  evaluation?: SelectionEvaluationRow;
  bestMatch?: SelectionSourceMatchRow;
  supply: SupplyReadiness;
  banned: BannedRisk;
};

export async function fetchCandidateInsights(id: string) {
  return getJSON<CandidateInsights>(`/api/v1/selection/candidates/${id}/insights`);
}

export async function fetchCandidatePriceTrend(id: string) {
  return getJSON<PriceTrend>(`/api/v1/selection/candidates/${id}/price-trend`);
}

export async function fetchSelectionCompare(ids: string[]) {
  return getWithParams<CompareRow[]>('/api/v1/selection/compare', { ids: ids.join(',') });
}

export async function fetchMarketSources() {
  return getJSON<MarketSourceStatus[]>('/api/v1/selection/market-sources');
}

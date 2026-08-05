import { getJSON } from '@/services/request';
import { responseErrorMessage } from '@/utils/httpErrorCopy';
import { fetchWithSessionGuard } from '@/utils/sessionGuard';

// ---- 利润报表 ----

export type ProfitDimension = 'order' | 'product' | 'shop';

export type MoneyByCurrency = {
  currency: string;
  amount: number;
  /** 折算到本位币金额；未配置汇率时缺省（显式「未折算」，不伪造） */
  baseAmount?: number;
};

export type ProfitFeeItem = {
  name: string;
  mode: 'percent' | 'fixed_per_order';
  value: number;
};

export type ProfitRow = {
  key: string;
  label: string;
  platform?: string;
  orderCount: number;
  quantity?: number;
  revenue: MoneyByCurrency[];
  revenueBase: number;
  unconvertedCurrencies?: string[];
  costCny: number;
  costBase?: number;
  missingCostLines: number;
  feeBase: number;
  grossProfitBase?: number;
  marginPercent?: number;
};

export type ProfitSummary = Omit<ProfitRow, 'key' | 'label' | 'platform' | 'quantity'>;

export type ProfitReportDTO = {
  generatedAt: string;
  dimension: ProfitDimension;
  startDate: string;
  endDate: string;
  baseCurrency: string;
  feeItems: ProfitFeeItem[];
  summary: ProfitSummary;
  rows: ProfitRow[];
  truncated?: boolean;
};

export type ReportRangeParams = {
  days?: number;
  start?: string;
  end?: string;
};

function rangeQuery(params: ReportRangeParams): string {
  const q = new URLSearchParams();
  if (params.start && params.end) {
    q.set('start', params.start);
    q.set('end', params.end);
  } else if (params.days) {
    q.set('days', String(params.days));
  }
  return q.toString();
}

export async function fetchProfitReport(
  dimension: ProfitDimension,
  params: ReportRangeParams,
): Promise<ProfitReportDTO> {
  const q = rangeQuery(params);
  return getJSON(`/api/v1/reports/profit?dimension=${dimension}${q ? `&${q}` : ''}`);
}

export async function downloadProfitReportCsv(dimension: ProfitDimension, params: ReportRangeParams) {
  const q = rangeQuery(params);
  const resp = await fetchWithSessionGuard(
    `/api/v1/reports/profit/export.csv?dimension=${dimension}${q ? `&${q}` : ''}`,
  );
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `profit-report-${dimension}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// ---- 采购报表 ----

export type LeadTimeBucket = { label: string; count: number };

export type SupplierAgg = {
  supplierId: string;
  supplierName: string;
  poCount: number;
  amount: number;
  deliveredCount: number;
  avgLeadTimeDays?: number;
};

export type ProcurementDaily = { date: string; poCount: number; amount: number };

export type ProcurementReportDTO = {
  generatedAt: string;
  startDate: string;
  endDate: string;
  currency: string;
  summary: {
    poCount: number;
    totalAmount: number;
    inTransitCount: number;
    deliveredCount: number;
    cancelledCount: number;
    avgLeadTimeDays?: number;
    leadTimeSamples: number;
  };
  daily: ProcurementDaily[];
  leadTime: LeadTimeBucket[];
  suppliers: SupplierAgg[];
};

export async function fetchProcurementReport(params: ReportRangeParams): Promise<ProcurementReportDTO> {
  const q = rangeQuery(params);
  return getJSON(`/api/v1/reports/procurement${q ? `?${q}` : ''}`);
}

// ---- 库存报表 ----

export type InventorySKURow = {
  productId: string;
  skuId: string;
  title: string;
  skuName?: string;
  skuCode?: string;
  stock: number;
  warningStock: number;
  safetyStock: number;
  unitCostCny?: number;
  stockValueCny?: number;
  lastOutboundAt?: string;
};

export type InventoryReportDTO = {
  generatedAt: string;
  slowDays: number;
  currency: string;
  warehouseId?: string;
  warehouseName?: string;
  summary: {
    skuCount: number;
    totalStock: number;
    stockValueCny: number;
    valuedSkuCount: number;
    unvaluedSkuCount: number;
    lowStockCount: number;
    outOfStockCount: number;
    slowMovingCount: number;
    avgDailyOutbound?: number;
    turnoverDays?: number;
  };
  slowMoving: InventorySKURow[];
  lowStock: InventorySKURow[];
};

export async function fetchInventoryReport(slowDays?: number, warehouseId?: string): Promise<InventoryReportDTO> {
  const parts: string[] = [];
  if (slowDays) parts.push(`slowDays=${slowDays}`);
  if (warehouseId) parts.push(`warehouseId=${encodeURIComponent(warehouseId)}`);
  const q = parts.length ? `?${parts.join('&')}` : '';
  return getJSON(`/api/v1/reports/inventory${q}`);
}

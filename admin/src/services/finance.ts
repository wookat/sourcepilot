import { deleteJSON, getWithParams, postJSON } from '@/services/request';
import { responseErrorMessage } from '@/utils/httpErrorCopy';
import { fetchWithSessionGuard } from '@/utils/sessionGuard';

// ---- 财务对账（回款 / 费用 / 实算毛利 / 对账报表） ----

/** 对账状态：未回款 / 少款 / 多款 / 已结清 */
export type SettlementStatus = 'unpaid' | 'short' | 'over' | 'settled';

export const SETTLEMENT_LABEL: Record<SettlementStatus, { text: string; color: string }> = {
  unpaid: { text: '未回款', color: 'default' },
  short: { text: '少款', color: 'red' },
  over: { text: '多款', color: 'orange' },
  settled: { text: '已结清', color: 'green' },
};

export type ExpenseType = {
  code: string;
  label: string;
};

export type PaymentRecordRow = {
  id: string;
  orderId: string;
  shopId?: string;
  amount: number;
  currency: string;
  feeAmount: number;
  receivedAt: string;
  channel?: string;
  remark?: string;
  source: 'manual' | 'import';
  createdAt: string;
  orderNo: string;
  orderAmount: number;
  orderCurrency: string;
  shopName?: string;
  settlementStatus: SettlementStatus;
  diffAmount: number;
};

export type PaymentListParams = {
  page?: number;
  pageSize?: number;
  orderId?: string;
  shopId?: string;
  status?: SettlementStatus | '';
};

export type CreatePaymentBody = {
  orderId: string;
  amount: number;
  currency?: string;
  feeAmount?: number;
  receivedAt: string;
  channel?: string;
  remark?: string;
};

export type OrderExpenseRow = {
  id: string;
  orderId: string;
  shopId?: string;
  typeCode: string;
  typeLabel?: string;
  amount: number;
  currency: string;
  incurredAt?: string;
  remark?: string;
  createdAt: string;
};

export type CreateOrderExpenseBody = {
  orderId: string;
  typeCode: string;
  amount: number;
  currency?: string;
  incurredAt?: string;
  remark?: string;
};

export type ShopExpenseRow = {
  id: string;
  shopId: string;
  shopName?: string;
  month: string;
  typeCode: string;
  typeLabel?: string;
  amount: number;
  currency: string;
  remark?: string;
  createdAt: string;
};

export type CreateShopExpenseBody = {
  shopId: string;
  month: string;
  typeCode: string;
  amount: number;
  currency?: string;
  remark?: string;
};

/** 订单财务视图（*Base 为本位币金额；未配置汇率时缺省，不伪造） */
export type OrderFinance = {
  orderId: string;
  orderNo: string;
  platform?: string;
  shopId?: string;
  shopName?: string;
  currency: string;
  receivable: number;
  received: number;
  feeTotal: number;
  diffAmount: number;
  settlementStatus: SettlementStatus;
  receivedBase?: number;
  actualCostBase?: number;
  expenseBase?: number;
  actualProfitBase?: number;
  estimatedProfitBase?: number;
  profitDiffBase?: number;
  largeDiff: boolean;
  missingActualLines: number;
  paymentCount: number;
  expenseCount: number;
};

export type OrderFinanceSummary = {
  baseCurrency: string;
  finance: OrderFinance;
  payments: PaymentRecordRow[];
  expenses: OrderExpenseRow[];
  expenseTypes: ExpenseType[];
};

export type ReconSummary = {
  orderCount: number;
  unpaidCount: number;
  shortCount: number;
  overCount: number;
  settledCount: number;
  largeDiffs: number;
  flaggedCount: number;
};

export type ReconciliationDTO = {
  generatedAt: string;
  startDate: string;
  endDate: string;
  baseCurrency: string;
  summary: ReconSummary;
  rows: OrderFinance[];
  truncated?: boolean;
};

export type ReconStatusFilter = '' | 'flagged' | 'large_diff' | SettlementStatus;

export type FeePart = {
  typeCode: string;
  typeLabel: string;
  base: number;
};

export type FinanceReportRow = {
  shopId?: string;
  shopName: string;
  month: string;
  orderCount: number;
  receivableBase?: number;
  receivedBase?: number;
  returnRatePercent?: number;
  feesByType: FeePart[];
  expenseBase?: number;
  shopExpenseBase?: number;
  actualCostBase?: number;
  actualProfitBase?: number;
  estimatedProfitBase?: number;
  profitDiffBase?: number;
  unpaidCount: number;
  shortCount: number;
  overCount: number;
  settledCount: number;
  largeDiffCount: number;
  missingActualLines: number;
};

export type FinanceReportDTO = {
  generatedAt: string;
  startDate: string;
  endDate: string;
  baseCurrency: string;
  rows: FinanceReportRow[];
};

export type FinanceRangeParams = {
  days?: number;
  start?: string;
  end?: string;
};

function rangeQuery(params: FinanceRangeParams): URLSearchParams {
  const q = new URLSearchParams();
  if (params.start && params.end) {
    q.set('start', params.start);
    q.set('end', params.end);
  } else if (params.days) {
    q.set('days', String(params.days));
  }
  return q;
}

export async function queryExpenseTypes(): Promise<{ items: ExpenseType[] }> {
  return getWithParams('/api/v1/finance/expense-types', {});
}

export async function queryPayments(
  params: PaymentListParams,
): Promise<{ items: PaymentRecordRow[]; total: number }> {
  return getWithParams('/api/v1/finance/payments', params);
}

export async function createPayment(body: CreatePaymentBody): Promise<PaymentRecordRow> {
  return postJSON('/api/v1/finance/payments', body);
}

export async function deletePayment(id: string): Promise<{ deleted: boolean }> {
  return deleteJSON(`/api/v1/finance/payments/${id}`);
}

export async function createOrderExpense(body: CreateOrderExpenseBody): Promise<OrderExpenseRow> {
  return postJSON('/api/v1/finance/order-expenses', body);
}

export async function deleteOrderExpense(id: string): Promise<{ deleted: boolean }> {
  return deleteJSON(`/api/v1/finance/order-expenses/${id}`);
}

export async function queryShopExpenses(params: {
  shopId?: string;
  month?: string;
  page?: number;
  pageSize?: number;
}): Promise<{ items: ShopExpenseRow[]; total: number }> {
  return getWithParams('/api/v1/finance/shop-expenses', params);
}

export async function createShopExpense(body: CreateShopExpenseBody): Promise<ShopExpenseRow> {
  return postJSON('/api/v1/finance/shop-expenses', body);
}

export async function deleteShopExpense(id: string): Promise<{ deleted: boolean }> {
  return deleteJSON(`/api/v1/finance/shop-expenses/${id}`);
}

export async function fetchOrderFinanceSummary(orderId: string): Promise<OrderFinanceSummary> {
  return getWithParams(`/api/v1/finance/orders/${orderId}/summary`, {});
}

export async function fetchReconciliation(
  params: FinanceRangeParams,
  status: ReconStatusFilter,
): Promise<ReconciliationDTO> {
  return getWithParams('/api/v1/finance/reconciliation', {
    ...params,
    status: status || undefined,
  });
}

export async function fetchFinanceReport(params: FinanceRangeParams): Promise<FinanceReportDTO> {
  return getWithParams('/api/v1/finance/report', { ...params });
}

async function downloadCsv(url: string, fileName: string) {
  const resp = await fetchWithSessionGuard(url);
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(objectUrl);
}

export async function downloadReconciliationCsv(
  params: FinanceRangeParams,
  status: ReconStatusFilter,
) {
  const q = rangeQuery(params);
  if (status) {
    q.set('status', status);
  }
  const s = q.toString();
  return downloadCsv(
    `/api/v1/finance/reconciliation/export.csv${s ? `?${s}` : ''}`,
    'finance-reconciliation.csv',
  );
}

export async function downloadFinanceReportCsv(params: FinanceRangeParams) {
  const q = rangeQuery(params).toString();
  return downloadCsv(`/api/v1/finance/report/export.csv${q ? `?${q}` : ''}`, 'finance-report.csv');
}

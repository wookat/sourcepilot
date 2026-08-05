import { ok } from './envelope';

export const E2E_FINANCE_ORDER_ID = 'e2e-finance-order-1';
export const E2E_FINANCE_PAYMENT_ID = 'e2e-finance-payment-1';
export const E2E_FINANCE_SHOP_ID = 'e2e-finance-shop-1';

export const e2eExpenseTypes = [
  { code: 'platform_commission', label: '平台佣金' },
  { code: 'promotion', label: '推广费' },
  { code: 'shipping', label: '运费' },
  { code: 'other', label: '其他' },
];

export const e2eFinancePayments = [
  {
    id: E2E_FINANCE_PAYMENT_ID,
    orderId: E2E_FINANCE_ORDER_ID,
    shopId: E2E_FINANCE_SHOP_ID,
    amount: 199.5,
    currency: 'CNY',
    feeAmount: 3.99,
    receivedAt: '2026-08-01T00:00:00Z',
    channel: '平台结算',
    source: 'manual',
    createdAt: '2026-08-01T00:00:00Z',
    orderNo: 'SO-E2E-FIN-1',
    orderAmount: 199.5,
    orderCurrency: 'CNY',
    shopName: 'e2e 店铺A',
    settlementStatus: 'settled',
    diffAmount: 0,
  },
  {
    id: 'e2e-finance-payment-2',
    orderId: 'e2e-finance-order-2',
    shopId: E2E_FINANCE_SHOP_ID,
    amount: 60,
    currency: 'CNY',
    feeAmount: 0,
    receivedAt: '2026-08-02T00:00:00Z',
    channel: '银行转账',
    source: 'import',
    createdAt: '2026-08-02T00:00:00Z',
    orderNo: 'SO-E2E-FIN-2',
    orderAmount: 100,
    orderCurrency: 'CNY',
    shopName: 'e2e 店铺A',
    settlementStatus: 'short',
    diffAmount: -40,
  },
];

export const e2eShopExpenses = [
  {
    id: 'e2e-finance-shop-expense-1',
    shopId: E2E_FINANCE_SHOP_ID,
    shopName: 'e2e 店铺A',
    month: '2026-08',
    typeCode: 'promotion',
    typeLabel: '推广费',
    amount: 88,
    currency: 'CNY',
    remark: 'e2e 月度推广费',
    createdAt: '2026-08-01T00:00:00Z',
  },
];

export const e2eReconciliation = {
  generatedAt: '2026-08-05T00:00:00Z',
  startDate: '2026-07-06',
  endDate: '2026-08-05',
  baseCurrency: 'CNY',
  summary: {
    orderCount: 3,
    unpaidCount: 1,
    shortCount: 1,
    overCount: 0,
    settledCount: 1,
    largeDiffs: 1,
    flaggedCount: 2,
  },
  rows: [
    {
      orderId: E2E_FINANCE_ORDER_ID,
      orderNo: 'SO-E2E-FIN-1',
      platform: 'manual',
      shopId: E2E_FINANCE_SHOP_ID,
      shopName: 'e2e 店铺A',
      currency: 'CNY',
      receivable: 199.5,
      received: 199.5,
      feeTotal: 3.99,
      diffAmount: 0,
      settlementStatus: 'settled',
      receivedBase: 195.51,
      actualCostBase: 120,
      expenseBase: 16.58,
      actualProfitBase: 58.93,
      estimatedProfitBase: 89.5,
      profitDiffBase: -30.57,
      largeDiff: true,
      missingActualLines: 0,
      paymentCount: 1,
      expenseCount: 2,
    },
    {
      orderId: 'e2e-finance-order-2',
      orderNo: 'SO-E2E-FIN-2',
      platform: 'manual',
      shopId: E2E_FINANCE_SHOP_ID,
      shopName: 'e2e 店铺A',
      currency: 'CNY',
      receivable: 100,
      received: 60,
      feeTotal: 0,
      diffAmount: -40,
      settlementStatus: 'short',
      largeDiff: false,
      missingActualLines: 1,
      paymentCount: 1,
      expenseCount: 0,
    },
  ],
};

export const e2eFinanceReport = {
  generatedAt: '2026-08-05T00:00:00Z',
  startDate: '2026-07-06',
  endDate: '2026-08-05',
  baseCurrency: 'CNY',
  rows: [
    {
      shopId: E2E_FINANCE_SHOP_ID,
      shopName: 'e2e 店铺A',
      month: '2026-08',
      orderCount: 3,
      receivableBase: 399.5,
      receivedBase: 255.51,
      returnRatePercent: 63.96,
      feesByType: [
        { typeCode: 'platform_commission', typeLabel: '平台佣金', base: 9.98 },
        { typeCode: 'promotion', typeLabel: '推广费', base: 6.6 },
      ],
      expenseBase: 16.58,
      shopExpenseBase: 118,
      actualCostBase: 120,
      actualProfitBase: -2.06,
      estimatedProfitBase: 150.2,
      profitDiffBase: -152.26,
      unpaidCount: 1,
      shortCount: 1,
      overCount: 0,
      settledCount: 1,
      largeDiffCount: 1,
      missingActualLines: 1,
    },
  ],
};

export const e2eFinanceShops = {
  list: [
    {
      id: E2E_FINANCE_SHOP_ID,
      shopName: 'e2e 店铺A',
      platform: 'manual',
      status: 'active',
    },
  ],
  pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 },
};

export const e2eFinanceOrdersLookup = {
  list: [
    {
      id: E2E_FINANCE_ORDER_ID,
      orderNo: 'SO-E2E-FIN-1',
      platform: 'manual',
      currency: 'CNY',
      totalAmount: 199.5,
      status: 'paid',
    },
  ],
  pagination: { page: 1, pageSize: 1, total: 1, totalPages: 1 },
};

export function financeResponse(path: string): ReturnType<typeof ok> | null {
  if (path === '/api/v1/finance/expense-types') {
    return ok({ items: e2eExpenseTypes });
  }
  if (path === '/api/v1/finance/payments') {
    return ok({ items: e2eFinancePayments, total: e2eFinancePayments.length });
  }
  if (path === '/api/v1/finance/shop-expenses') {
    return ok({ items: e2eShopExpenses, total: e2eShopExpenses.length });
  }
  if (path === '/api/v1/finance/reconciliation') {
    return ok(e2eReconciliation);
  }
  if (path === '/api/v1/finance/report') {
    return ok(e2eFinanceReport);
  }
  return null;
}

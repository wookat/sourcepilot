import { ok } from './envelope';

/** 移动首页（R113）相关 GET Mock：经营指标 / 利润摘要 / 工作台待办 */

export function salesStatsBody() {
  return ok({
    generatedAt: '2026-08-05T01:00:00Z',
    baseCurrency: 'CNY',
    windows: [
      {
        key: 'today',
        orderCount: 12,
        paidCount: 9,
        shippedCount: 5,
        paidAmounts: [{ currency: 'CNY', amount: 3200, orders: 9, baseAmount: 3200 }],
        paidAmountBase: 3200,
      },
      {
        key: '7d',
        orderCount: 86,
        paidCount: 70,
        shippedCount: 61,
        paidAmounts: [{ currency: 'CNY', amount: 25800, orders: 70, baseAmount: 25800 }],
        paidAmountBase: 25800,
      },
      {
        key: '30d',
        orderCount: 300,
        paidCount: 260,
        shippedCount: 240,
        paidAmounts: [{ currency: 'CNY', amount: 99000, orders: 260, baseAmount: 99000 }],
        paidAmountBase: 99000,
      },
    ],
  });
}

export function profitReportBody(days: number) {
  const grossProfitBase = days === 1 ? 860 : 6400;
  return ok({
    generatedAt: '2026-08-05T01:00:00Z',
    dimension: 'order',
    startDate: '2026-08-05',
    endDate: '2026-08-05',
    baseCurrency: 'CNY',
    feeItems: [],
    summary: {
      orderCount: days === 1 ? 9 : 70,
      revenue: [{ currency: 'CNY', amount: days === 1 ? 3200 : 25800, baseAmount: days === 1 ? 3200 : 25800 }],
      revenueBase: days === 1 ? 3200 : 25800,
      costCny: days === 1 ? 2100 : 17800,
      costBase: days === 1 ? 2100 : 17800,
      missingCostLines: 0,
      feeBase: days === 1 ? 240 : 1600,
      grossProfitBase,
      marginPercent: 25,
    },
    rows: [],
  });
}

function todo(key: string, title: string, count: number, link: string) {
  return { id: `e2e-todo-${key}`, key, title, count, severity: 'high', level: 'high', description: title, link };
}

export function productOperationDashboardBody() {
  return ok({
    summary: {
      criticalAlertCount: 2,
      openAlertCount: 1,
    },
    todos: [
      todo('order_await_payment', '订单待收款确认', 3, '/orders/list?payStatus=unpaid'),
      todo('order_await_procurement', '订单待采购', 2, '/orders/list?payStatus=paid&hasPurchase=0'),
      todo('procurement_await_receipt', '采购单待签收', 4, '/procurement/orders?status=shipped'),
      todo('order_await_shipment', '订单待发货', 5, '/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled'),
      todo('order_exceptions', '订单异常', 1, '/orders/exceptions'),
      todo('missing_ai_title', '待补 AI 标题', 8, '/product/drafts?missingAiTitle=1'),
    ],
    funnel: [],
    exceptions: [],
    charts: {},
    quickLinks: [],
    recent: {},
  });
}

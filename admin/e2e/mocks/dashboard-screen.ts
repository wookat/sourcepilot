import { ok } from './envelope';

function hourIso(offset: number): string {
  const d = new Date('2026-08-06T15:00:00+08:00');
  d.setHours(d.getHours() - offset);
  return d.toISOString();
}

export function dashboardScreenResponse() {
  const trend = Array.from({ length: 24 }, (_, i) => ({
    hour: hourIso(23 - i),
    orderCount: (i * 7) % 13,
    paidCount: (i * 5) % 9,
  }));
  return ok({
    generatedAt: '2026-08-06T07:00:00Z',
    funnelDays: 7,
    trendHours: 24,
    today: {
      orderCount: 128,
      paidOrderCount: 96,
      salesBase: 45230.5,
      baseCurrency: 'USD',
      unconvertedCurrencies: ['EUR'],
      grossProfitBase: 12890.25,
      marginPercent: 28.5,
    },
    todos: [
      { key: 'await_payment', title: '待收款确认', count: 12, priority: 'P1', link: '/orders/list?payStatus=unpaid' },
      { key: 'await_procurement', title: '待采购', count: 8, priority: 'P1', link: '/orders/list?payStatus=paid&hasPurchase=0' },
      { key: 'await_shipment', title: '待发货', count: 15, priority: 'P1', link: '/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled' },
      { key: 'in_transit', title: '在途待送达', count: 33, priority: 'P2', link: '/orders/list?status=shipped' },
      { key: 'order_exceptions', title: '订单异常', count: 4, priority: 'P0', link: '/orders/exceptions' },
    ],
    funnel: [
      { key: 'created', title: '新建订单', count: 320 },
      { key: 'paid', title: '已付款', count: 260 },
      { key: 'procured', title: '已生成采购', count: 210 },
      { key: 'shipped', title: '已发货', count: 180 },
      { key: 'delivered', title: '已送达', count: 120 },
    ],
    trend,
    alerts: [
      {
        type: 'task_alert',
        severity: 'critical',
        title: '订单同步失败：DEMO 抖店 A',
        detail: '连续 3 次拉单失败，请检查店铺授权',
        link: '/ops/task-center/alerts',
        occurredAt: '2026-08-06T06:40:00Z',
      },
      {
        type: 'out_of_stock',
        severity: 'high',
        title: '断货：DEMO 商品 A',
        detail: 'SKU DEMO-A-1 库存 0（预警线 10）',
        link: '/inventory/alerts',
        occurredAt: '2026-08-06T05:20:00Z',
      },
      {
        type: 'low_stock',
        severity: 'medium',
        title: '低库存：DEMO 商品 B',
        detail: 'SKU DEMO-B-2 库存 3（预警线 10）',
        link: '/inventory/alerts',
        occurredAt: '2026-08-06T04:10:00Z',
      },
    ],
  });
}

export function dashboardScreenEmptyResponse() {
  return ok({
    generatedAt: '2026-08-06T07:00:00Z',
    funnelDays: 7,
    trendHours: 24,
    today: { orderCount: 0, paidOrderCount: 0, salesBase: 0, baseCurrency: 'USD' },
    todos: [
      { key: 'await_payment', title: '待收款确认', count: 0, priority: 'P1', link: '/orders/list?payStatus=unpaid' },
      { key: 'await_procurement', title: '待采购', count: 0, priority: 'P1', link: '/orders/list?payStatus=paid&hasPurchase=0' },
      { key: 'await_shipment', title: '待发货', count: 0, priority: 'P1', link: '/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled' },
      { key: 'in_transit', title: '在途待送达', count: 0, priority: 'P2', link: '/orders/list?status=shipped' },
      { key: 'order_exceptions', title: '订单异常', count: 0, priority: 'P0', link: '/orders/exceptions' },
    ],
    funnel: [
      { key: 'created', title: '新建订单', count: 0 },
      { key: 'paid', title: '已付款', count: 0 },
      { key: 'procured', title: '已生成采购', count: 0 },
      { key: 'shipped', title: '已发货', count: 0 },
      { key: 'delivered', title: '已送达', count: 0 },
    ],
    trend: Array.from({ length: 24 }, (_, i) => ({ hour: hourIso(23 - i), orderCount: 0, paidCount: 0 })),
    alerts: [],
  });
}

import { ok } from './envelope';

export type DailyStatItem = {
  date: string;
  orderCount: number;
  paidCount: number;
  shippedCount: number;
  paidAmounts: { currency: string; amount: number; orders: number }[];
};

function isoDaysAgo(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() - offset);
  return d.toISOString().slice(0, 10);
}

export function dailyStatsItems(days: number): DailyStatItem[] {
  const items: DailyStatItem[] = [];
  for (let i = days - 1; i >= 0; i--) {
    const date = isoDaysAgo(i);
    if (i === 0) {
      items.push({
        date,
        orderCount: 3,
        paidCount: 2,
        shippedCount: 1,
        paidAmounts: [{ currency: 'USD', amount: 25.5, orders: 2 }],
      });
    } else if (i === 1) {
      items.push({
        date,
        orderCount: 1,
        paidCount: 1,
        shippedCount: 0,
        paidAmounts: [{ currency: 'EUR', amount: 7.5, orders: 1 }],
      });
    } else {
      items.push({ date, orderCount: 0, paidCount: 0, shippedCount: 0, paidAmounts: [] });
    }
  }
  return items;
}

export function dailyStatsResponse(days: number) {
  return ok({
    generatedAt: new Date().toISOString(),
    days,
    items: dailyStatsItems(days),
  });
}

export const dailyStatsCsvBody =
  '\uFEFF日期,订单数,已付款数,已发货数,已付款销售额(EUR),已付款销售额(USD)\n' +
  `${isoDaysAgo(1)},1,1,0,7.50,0.00\n` +
  `${isoDaysAgo(0)},3,2,1,0.00,25.50\n`;

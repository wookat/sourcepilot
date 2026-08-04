import { ok } from './envelope';

export type DailyStatItem = {
  date: string;
  orderCount: number;
  paidCount: number;
  shippedCount: number;
  paidAmounts: { currency: string; amount: number; orders: number; baseAmount?: number }[];
  paidAmountBase: number;
  unconvertedCurrencies?: string[];
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
      // USD 已配置汇率（折算入本位币）
      items.push({
        date,
        orderCount: 3,
        paidCount: 2,
        shippedCount: 1,
        paidAmounts: [{ currency: 'USD', amount: 25.5, orders: 2, baseAmount: 181.82 }],
        paidAmountBase: 181.82,
      });
    } else if (i === 1) {
      // EUR 未配置汇率（未折算）
      items.push({
        date,
        orderCount: 1,
        paidCount: 1,
        shippedCount: 0,
        paidAmounts: [{ currency: 'EUR', amount: 7.5, orders: 1 }],
        paidAmountBase: 0,
        unconvertedCurrencies: ['EUR'],
      });
    } else {
      items.push({ date, orderCount: 0, paidCount: 0, shippedCount: 0, paidAmounts: [], paidAmountBase: 0 });
    }
  }
  return items;
}

export function dailyStatsResponse(days: number) {
  return ok({
    generatedAt: new Date().toISOString(),
    days,
    baseCurrency: 'CNY',
    items: dailyStatsItems(days),
  });
}

export const dailyStatsCsvBody =
  '\uFEFF日期,订单数,已付款数,已发货数,已付款销售额(EUR),折算金额(EUR→CNY),已付款销售额(USD),折算金额(USD→CNY),已付款销售额合计(CNY),未折算币种\n' +
  `${isoDaysAgo(1)},1,1,0,7.50,,0.00,0.00,0.00,EUR\n` +
  `${isoDaysAgo(0)},3,2,1,0.00,,25.50,181.82,181.82,EUR\n`;

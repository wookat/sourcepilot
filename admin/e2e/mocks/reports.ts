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

// ---- Round 110 深度报表 mock ----

export function profitReportResponse(dimension: string) {
  const cost = 40;
  const profit = 141.82 - cost;
  return ok({
    generatedAt: new Date().toISOString(),
    dimension,
    startDate: isoDaysAgo(29),
    endDate: isoDaysAgo(0),
    baseCurrency: 'CNY',
    feeItems: [{ name: '平台佣金', mode: 'percent', value: 5 }],
    summary: {
      orderCount: 3,
      revenue: [
        { currency: 'EUR', amount: 7.5 },
        { currency: 'USD', amount: 25.5, baseAmount: 181.82 },
      ],
      revenueBase: 181.82,
      unconvertedCurrencies: ['EUR'],
      costCny: cost,
      costBase: cost,
      missingCostLines: 1,
      feeBase: 9.09,
      grossProfitBase: 132.73,
      marginPercent: 73,
    },
    rows: [
      {
        key: 'row-1',
        label: dimension === 'product' ? 'DEMO 商品 A' : dimension === 'shop' ? 'DEMO 店铺' : 'SO-1001',
        platform: 'tiktok',
        orderCount: 2,
        quantity: 4,
        revenue: [{ currency: 'USD', amount: 25.5, baseAmount: 181.82 }],
        revenueBase: 181.82,
        costCny: cost,
        costBase: cost,
        missingCostLines: 0,
        feeBase: 9.09,
        grossProfitBase: profit,
        marginPercent: 56.01,
      },
      {
        key: 'row-2',
        label: dimension === 'product' ? 'DEMO 商品 B' : dimension === 'shop' ? '未绑定店铺' : 'SO-1002',
        orderCount: 1,
        quantity: 1,
        revenue: [{ currency: 'EUR', amount: 7.5 }],
        revenueBase: 0,
        unconvertedCurrencies: ['EUR'],
        costCny: 0,
        missingCostLines: 1,
        feeBase: 0,
      },
    ],
  });
}

export const profitCsvBody =
  '\uFEFF维度,订单数,原币收入,收入折算(CNY),采购成本(CNY),费用(CNY),毛利(CNY),毛利率(%),缺进价行数,未折算币种\n' +
  'SO-1001,2,USD 25.50,181.82,40.00,9.09,132.73,73.00,0,\n';

export function procurementReportResponse() {
  return ok({
    generatedAt: new Date().toISOString(),
    startDate: isoDaysAgo(29),
    endDate: isoDaysAgo(0),
    currency: 'CNY',
    summary: {
      poCount: 5,
      totalAmount: 1280.5,
      inTransitCount: 2,
      deliveredCount: 2,
      cancelledCount: 1,
      avgLeadTimeDays: 3.5,
      leadTimeSamples: 2,
    },
    daily: [
      { date: isoDaysAgo(1), poCount: 3, amount: 800.5 },
      { date: isoDaysAgo(0), poCount: 2, amount: 480 },
    ],
    leadTime: [
      { label: '0-3 天', count: 1 },
      { label: '4-7 天', count: 1 },
      { label: '8-15 天', count: 0 },
      { label: '16 天以上', count: 0 },
    ],
    suppliers: [
      { supplierId: 'sup-1', supplierName: 'DEMO 供应商甲', poCount: 3, amount: 800.5, deliveredCount: 2, avgLeadTimeDays: 3.5 },
      { supplierId: 'sup-2', supplierName: 'DEMO 供应商乙', poCount: 2, amount: 480, deliveredCount: 0 },
    ],
  });
}

export function inventoryReportResponse(slowDays: number) {
  return ok({
    generatedAt: new Date().toISOString(),
    slowDays,
    currency: 'CNY',
    summary: {
      skuCount: 3,
      totalStock: 130,
      stockValueCny: 2350,
      valuedSkuCount: 2,
      unvaluedSkuCount: 1,
      lowStockCount: 1,
      outOfStockCount: 0,
      slowMovingCount: 1,
      avgDailyOutbound: 1.2,
      turnoverDays: 108.3,
    },
    slowMoving: [
      {
        productId: 'p-1',
        skuId: 'sku-1',
        title: 'DEMO 滞销商品',
        skuName: '红色 M',
        skuCode: 'DEMO-A',
        stock: 100,
        warningStock: 5,
        safetyStock: 0,
        unitCostCny: 20,
        stockValueCny: 2000,
      },
    ],
    lowStock: [
      {
        productId: 'p-2',
        skuId: 'sku-2',
        title: 'DEMO 低库存商品',
        skuCode: 'DEMO-B',
        stock: 3,
        warningStock: 5,
        safetyStock: 2,
        unitCostCny: 116.67,
        stockValueCny: 350,
        lastOutboundAt: new Date().toISOString(),
      },
    ],
  });
}

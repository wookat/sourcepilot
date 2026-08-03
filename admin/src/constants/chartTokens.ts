import { themeTokens, tmSemanticTokens } from './layoutTokens';

/** 报表图表统一视觉 token：系列色取自 AntD 主题 seed，保证图表与全站配色一致 */
export const chartTokens = {
  seriesColors: [
    themeTokens.colorPrimary,
    themeTokens.colorSuccess,
    themeTokens.colorWarning,
    tmSemanticTokens.dataAccent,
    tmSemanticTokens.aiAccent,
    themeTokens.colorError,
  ],
  height: 300,
  heightCompact: 220,
  /** Column 单柱宽度上限（px）：数据点少时避免单柱铺满整卡 */
  barMaxWidth: 40,
  /** 涨/正向语义色（与订单列表预估毛利一致） */
  trendUp: '#3f8600',
  /** 跌/负向语义色 */
  trendDown: '#cf1322',
} as const;

/** x 轴标签公共配置：自动旋转 + 自动抽样隐藏，密集日期不重叠 */
export const chartAxisXLabel = { labelAutoRotate: true, labelAutoHide: true } as const;

/** x 轴日期刻度目标数量：宽屏/紧凑两档，密集日期抽稀不成标签墙 */
export const chartAxisXTickCount = { wide: 10, compact: 6 } as const;

/**
 * 分类轴标签抽稀过滤器：G2 分类轴的 tickCount 仅为建议值，密集日期轴需用
 * labelFilter 强制按步长抽稀，保证标签数不超过目标档位（末点标签始终保留）。
 */
export function makeCategoryLabelFilter(pointCount: number, targetCount: number) {
  const step = Math.max(1, Math.ceil(pointCount / Math.max(1, targetCount)));
  return (_datum: unknown, index: number) => index % step === 0 || index === pointCount - 1;
}

/** 日期刻度短标签：YYYY-MM-DD → MM-DD，其余原样返回 */
export function formatDateTickShort(value: unknown): string {
  const text = String(value ?? '');
  const m = /^\d{4}-(\d{2}-\d{2})/.exec(text);
  return m ? m[1] : text;
}

/** 数字排版：等宽数字，用于统计卡与金额列 */
export const tabularNumsStyle = { fontVariantNumeric: 'tabular-nums' } as const;

const countFormatter = new Intl.NumberFormat('zh-CN');

/** 整数计数：千分位分组（如 12,345） */
export function formatCount(value: number): string {
  return countFormatter.format(value);
}

/** 金额：千分位 + 固定两位小数，可带币种前缀（如 USD 1,234.50） */
export function formatAmount(value: number, currency?: string): string {
  const text = value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return currency ? `${currency} ${text}` : text;
}

export type ChartTokens = typeof chartTokens;

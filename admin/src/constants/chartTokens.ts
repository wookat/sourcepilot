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

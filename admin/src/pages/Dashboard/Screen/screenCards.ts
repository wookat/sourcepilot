import type { ScreenCard, ScreenCardKey } from '@/services/dashboard';

/** 默认卡片布局：与后端默认一致（全部启用、原始顺序），配置缺失时兜底。 */
export const DEFAULT_SCREEN_CARDS: ScreenCard[] = [
  { key: 'kpi_orders', title: '今日订单', enabled: true },
  { key: 'kpi_sales', title: '今日销售额', enabled: true },
  { key: 'kpi_profit', title: '今日毛利', enabled: true },
  { key: 'kpi_alerts', title: '当前告警', enabled: true },
  { key: 'todos', title: '待办事项', enabled: true },
  { key: 'funnel', title: '订单状态漏斗', enabled: true },
  { key: 'trend', title: '24 小时订单趋势', enabled: true },
  { key: 'alerts', title: '告警滚动列表', enabled: true },
];

const KPI_KEYS: ScreenCardKey[] = ['kpi_orders', 'kpi_sales', 'kpi_profit', 'kpi_alerts'];
const CHART_KEYS: ScreenCardKey[] = ['funnel', 'trend'];

export type ScreenCardSegment =
  | { type: 'kpi'; keys: ScreenCardKey[] }
  | { type: 'charts'; keys: ScreenCardKey[] }
  | { type: 'todos' }
  | { type: 'alerts' };

/**
 * 把启用的卡片按配置顺序分组成渲染段：相邻 KPI 卡合并为一行，相邻的
 * 漏斗/趋势合并为一行，待办与告警各自独立成段。
 */
export function groupScreenCards(cards: ScreenCard[] | undefined): ScreenCardSegment[] {
  const list = (cards && cards.length ? cards : DEFAULT_SCREEN_CARDS).filter((c) => c.enabled);
  const out: ScreenCardSegment[] = [];
  for (const card of list) {
    const last = out[out.length - 1];
    if (KPI_KEYS.includes(card.key)) {
      if (last && last.type === 'kpi') {
        last.keys.push(card.key);
      } else {
        out.push({ type: 'kpi', keys: [card.key] });
      }
    } else if (CHART_KEYS.includes(card.key)) {
      if (last && last.type === 'charts') {
        last.keys.push(card.key);
      } else {
        out.push({ type: 'charts', keys: [card.key] });
      }
    } else if (card.key === 'todos') {
      out.push({ type: 'todos' });
    } else if (card.key === 'alerts') {
      out.push({ type: 'alerts' });
    }
  }
  return out;
}

/** 上移/下移一张卡片，返回新数组（越界时原样返回）。 */
export function moveScreenCard(cards: ScreenCard[], index: number, delta: -1 | 1): ScreenCard[] {
  const target = index + delta;
  if (index < 0 || index >= cards.length || target < 0 || target >= cards.length) return cards;
  const next = [...cards];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

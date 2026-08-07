import { describe, expect, it } from 'vitest';
import type { ScreenCard } from '@/services/dashboard';
import { DEFAULT_SCREEN_CARDS, groupScreenCards, moveScreenCard } from '../screenCards';

describe('经营大屏 groupScreenCards', () => {
  it('缺省配置回退默认布局：KPI 一行 + 待办 + 图表一行 + 告警', () => {
    const segments = groupScreenCards(undefined);
    expect(segments).toEqual([
      { type: 'kpi', keys: ['kpi_orders', 'kpi_sales', 'kpi_profit', 'kpi_alerts'] },
      { type: 'todos' },
      { type: 'charts', keys: ['funnel', 'trend'] },
      { type: 'alerts' },
    ]);
  });

  it('禁用的卡片不出现在分段中', () => {
    const cards: ScreenCard[] = DEFAULT_SCREEN_CARDS.map((c) =>
      c.key === 'kpi_profit' || c.key === 'alerts' ? { ...c, enabled: false } : c,
    );
    const segments = groupScreenCards(cards);
    expect(segments).toEqual([
      { type: 'kpi', keys: ['kpi_orders', 'kpi_sales', 'kpi_alerts'] },
      { type: 'todos' },
      { type: 'charts', keys: ['funnel', 'trend'] },
    ]);
  });

  it('自定义顺序生效：不相邻的 KPI 卡各自成行', () => {
    const cards: ScreenCard[] = [
      { key: 'kpi_sales', title: '今日销售额', enabled: true },
      { key: 'trend', title: '趋势', enabled: true },
      { key: 'kpi_orders', title: '今日订单', enabled: true },
    ];
    const segments = groupScreenCards(cards);
    expect(segments).toEqual([
      { type: 'kpi', keys: ['kpi_sales'] },
      { type: 'charts', keys: ['trend'] },
      { type: 'kpi', keys: ['kpi_orders'] },
    ]);
  });

  it('空数组回退默认布局', () => {
    expect(groupScreenCards([])).toEqual(groupScreenCards(undefined));
  });
});

describe('经营大屏 moveScreenCard', () => {
  it('上移/下移交换相邻位置且不修改原数组', () => {
    const cards = DEFAULT_SCREEN_CARDS;
    const moved = moveScreenCard(cards, 1, -1);
    expect(moved[0].key).toBe('kpi_sales');
    expect(moved[1].key).toBe('kpi_orders');
    expect(cards[0].key).toBe('kpi_orders');
  });

  it('越界时原样返回', () => {
    expect(moveScreenCard(DEFAULT_SCREEN_CARDS, 0, -1)).toBe(DEFAULT_SCREEN_CARDS);
    const last = DEFAULT_SCREEN_CARDS.length - 1;
    expect(moveScreenCard(DEFAULT_SCREEN_CARDS, last, 1)).toBe(DEFAULT_SCREEN_CARDS);
  });
});

import { beforeEach, describe, expect, it, vi } from 'vitest';

const getWithParams = vi.fn();
const postJSON = vi.fn();
const putJSON = vi.fn();
const deleteJSON = vi.fn();

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: (...args: unknown[]) => getWithParams(...args),
  postJSON: (...args: unknown[]) => postJSON(...args),
  putJSON: (...args: unknown[]) => putJSON(...args),
  deleteJSON: (...args: unknown[]) => deleteJSON(...args),
}));

import {
  AUTOMATION_ACTION_LABELS,
  AUTOMATION_EVENT_ACTIONS,
  createOrderAutomationRule,
  listOrderAutomationRules,
  SHIPPING_APPLY_MODE_LABELS,
  updateOrderAutomationRule,
  WAREHOUSE_STRATEGY_LABELS,
} from '../orderAutomation';

beforeEach(() => {
  getWithParams.mockReset().mockResolvedValue({ items: [] });
  postJSON.mockReset().mockResolvedValue({ ok: true });
  putJSON.mockReset().mockResolvedValue({ ok: true });
  deleteJSON.mockReset().mockResolvedValue({ ok: true });
});

describe('order automation services', () => {
  it('lists rules and unwraps items', async () => {
    getWithParams.mockResolvedValue({ items: [{ id: 'r1' }] });
    const rows = await listOrderAutomationRules();
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/order-automation-rules', {});
    expect(rows).toEqual([{ id: 'r1' }]);
  });

  it('creates rule with new R126 action params', async () => {
    await createOrderAutomationRule({
      name: '付款后自动应用发货规则',
      triggerEvent: 'order_paid',
      action: 'apply_shipping_rule',
      shippingApplyMode: 'apply',
    });
    expect(postJSON).toHaveBeenCalledWith(
      '/api/v1/order-automation-rules',
      expect.objectContaining({ action: 'apply_shipping_rule', shippingApplyMode: 'apply' }),
    );
  });

  it('updates rule with warehouse strategy', async () => {
    await updateOrderAutomationRule('r1', {
      action: 'assign_warehouse',
      warehouseStrategy: 'stock_first',
    });
    expect(putJSON).toHaveBeenCalledWith(
      '/api/v1/order-automation-rules/r1',
      expect.objectContaining({ warehouseStrategy: 'stock_first' }),
    );
  });
});

describe('R126 automation action maps', () => {
  it('labels the new actions', () => {
    expect(AUTOMATION_ACTION_LABELS.apply_shipping_rule).toBe('自动应用发货规则');
    expect(AUTOMATION_ACTION_LABELS.assign_warehouse).toBe('自动分仓');
    expect(SHIPPING_APPLY_MODE_LABELS.recommend).toBe('仅推荐物流商');
    expect(SHIPPING_APPLY_MODE_LABELS.apply).toBe('直接应用物流商');
    expect(WAREHOUSE_STRATEGY_LABELS.default_warehouse).toBe('默认仓');
    expect(WAREHOUSE_STRATEGY_LABELS.stock_first).toBe('库存充足优先');
  });

  it('binds new actions to the allowed trigger events (与后端口径一致)', () => {
    expect(AUTOMATION_EVENT_ACTIONS.order_created).toContain('apply_shipping_rule');
    expect(AUTOMATION_EVENT_ACTIONS.order_created).not.toContain('assign_warehouse');
    expect(AUTOMATION_EVENT_ACTIONS.order_paid).toEqual(
      expect.arrayContaining(['apply_shipping_rule', 'assign_warehouse']),
    );
    expect(AUTOMATION_EVENT_ACTIONS.procurement_delivered).toEqual(
      expect.arrayContaining(['apply_shipping_rule', 'assign_warehouse']),
    );
    expect(AUTOMATION_EVENT_ACTIONS.logistics_collected).toContain('apply_shipping_rule');
    expect(AUTOMATION_EVENT_ACTIONS.logistics_collected).not.toContain('assign_warehouse');
  });
});

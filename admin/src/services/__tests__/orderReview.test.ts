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
  approveReviewOrders,
  createOrderReviewRule,
  deleteOrderReviewRule,
  dryRunOrderReviewRule,
  listOrderReviewRules,
  listOrderReviewWorkbench,
  rejectReviewOrders,
  updateOrderReviewRule,
} from '../orderReview';

beforeEach(() => {
  getWithParams.mockReset().mockResolvedValue({ items: [] });
  postJSON.mockReset().mockResolvedValue({ ok: true });
  putJSON.mockReset().mockResolvedValue({ ok: true });
  deleteJSON.mockReset().mockResolvedValue({ ok: true });
});

describe('order review services', () => {
  it('lists rules and unwraps items', async () => {
    getWithParams.mockResolvedValue({ items: [{ id: 'r1' }] });
    const rows = await listOrderReviewRules();
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/order-review-rules', {});
    expect(rows).toEqual([{ id: 'r1' }]);
  });

  it('creates a rule via POST with action and conditions', async () => {
    await createOrderReviewRule({ name: '大额审核', action: 'review', minAmount: 500 });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/order-review-rules', {
      name: '大额审核',
      action: 'review',
      minAmount: 500,
    });
  });

  it('updates via PUT and deletes via DELETE on the rule path', async () => {
    await updateOrderReviewRule('r1', { enabled: false });
    expect(putJSON).toHaveBeenCalledWith('/api/v1/order-review-rules/r1', { enabled: false });
    await deleteOrderReviewRule('r1');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/order-review-rules/r1');
  });

  it('dry-runs via POST on the dry-run path', async () => {
    await dryRunOrderReviewRule({ minAmount: 100 });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/order-review-rules/dry-run', {
      minAmount: 100,
    });
  });

  it('queries the workbench with pagination and filters', async () => {
    getWithParams.mockResolvedValue({ items: [], total: 0 });
    await listOrderReviewWorkbench({ page: 2, pageSize: 50, reviewStatus: 'held', keyword: 'SO' });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/order-review', {
      page: 2,
      pageSize: 50,
      reviewStatus: 'held',
      keyword: 'SO',
    });
  });

  it('approves and rejects batches via POST with orderIds', async () => {
    await approveReviewOrders(['o1', 'o2']);
    expect(postJSON).toHaveBeenCalledWith('/api/v1/order-review/approve', {
      orderIds: ['o1', 'o2'],
    });
    await rejectReviewOrders(['o3']);
    expect(postJSON).toHaveBeenCalledWith('/api/v1/order-review/reject', { orderIds: ['o3'] });
  });
});

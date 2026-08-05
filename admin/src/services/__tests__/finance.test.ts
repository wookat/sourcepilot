import { beforeEach, describe, expect, it, vi } from 'vitest';

const getWithParams = vi.fn();
const postJSON = vi.fn();
const deleteJSON = vi.fn();

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: (...args: unknown[]) => getWithParams(...args),
  postJSON: (...args: unknown[]) => postJSON(...args),
  putJSON: vi.fn(),
  deleteJSON: (...args: unknown[]) => deleteJSON(...args),
}));

vi.mock('@/utils/sessionGuard', () => ({
  fetchWithSessionGuard: vi.fn(),
}));

import {
  createOrderExpense,
  createPayment,
  createShopExpense,
  deleteOrderExpense,
  deletePayment,
  deleteShopExpense,
  fetchFinanceReport,
  fetchOrderFinanceSummary,
  fetchReconciliation,
  queryExpenseTypes,
  queryPayments,
  queryShopExpenses,
  SETTLEMENT_LABEL,
} from '../finance';

beforeEach(() => {
  getWithParams.mockReset().mockResolvedValue({ items: [], total: 0 });
  postJSON.mockReset().mockResolvedValue({ id: 'x' });
  deleteJSON.mockReset().mockResolvedValue({ deleted: true });
});

describe('finance services', () => {
  it('maps all settlement statuses to Chinese labels', () => {
    expect(SETTLEMENT_LABEL.unpaid.text).toBe('未回款');
    expect(SETTLEMENT_LABEL.short.text).toBe('少款');
    expect(SETTLEMENT_LABEL.over.text).toBe('多款');
    expect(SETTLEMENT_LABEL.settled.text).toBe('已结清');
  });

  it('loads expense types', async () => {
    await queryExpenseTypes();
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/expense-types', {});
  });

  it('lists payments with filters', async () => {
    await queryPayments({ page: 2, pageSize: 50, shopId: 's1', status: 'short' });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/payments', {
      page: 2,
      pageSize: 50,
      shopId: 's1',
      status: 'short',
    });
  });

  it('creates a payment via POST with the finance body', async () => {
    await createPayment({
      orderId: 'o1',
      amount: 199.5,
      currency: 'CNY',
      feeAmount: 3.99,
      receivedAt: '2026-08-01',
      channel: '平台结算',
    });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/finance/payments', {
      orderId: 'o1',
      amount: 199.5,
      currency: 'CNY',
      feeAmount: 3.99,
      receivedAt: '2026-08-01',
      channel: '平台结算',
    });
  });

  it('deletes payments / expenses via DELETE on the id path', async () => {
    await deletePayment('p1');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/finance/payments/p1');
    await deleteOrderExpense('e1');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/finance/order-expenses/e1');
    await deleteShopExpense('se1');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/finance/shop-expenses/se1');
  });

  it('creates order and shop expenses via POST', async () => {
    await createOrderExpense({ orderId: 'o1', typeCode: 'promotion', amount: 8.8 });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/finance/order-expenses', {
      orderId: 'o1',
      typeCode: 'promotion',
      amount: 8.8,
    });
    await createShopExpense({ shopId: 's1', month: '2026-08', typeCode: 'other', amount: 30 });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/finance/shop-expenses', {
      shopId: 's1',
      month: '2026-08',
      typeCode: 'other',
      amount: 30,
    });
  });

  it('lists shop expenses with month filter', async () => {
    await queryShopExpenses({ shopId: 's1', month: '2026-08', page: 1, pageSize: 20 });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/shop-expenses', {
      shopId: 's1',
      month: '2026-08',
      page: 1,
      pageSize: 20,
    });
  });

  it('fetches the order finance summary by order id', async () => {
    getWithParams.mockResolvedValue({ baseCurrency: 'CNY' });
    await fetchOrderFinanceSummary('o1');
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/orders/o1/summary', {});
  });

  it('fetches reconciliation with range and status, dropping the empty filter', async () => {
    getWithParams.mockResolvedValue({ rows: [] });
    await fetchReconciliation({ days: 30 }, 'large_diff');
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/reconciliation', {
      days: 30,
      status: 'large_diff',
    });
    await fetchReconciliation({ start: '2026-08-01', end: '2026-08-31' }, '');
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/reconciliation', {
      start: '2026-08-01',
      end: '2026-08-31',
      status: undefined,
    });
  });

  it('fetches the shop/month report with range params', async () => {
    getWithParams.mockResolvedValue({ rows: [] });
    await fetchFinanceReport({ days: 90 });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/finance/report', { days: 90 });
  });
});

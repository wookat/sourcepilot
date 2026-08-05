import { beforeEach, describe, expect, it, vi } from 'vitest';

const getJSON = vi.fn();
const getWithParams = vi.fn();
const postJSON = vi.fn();
const putJSON = vi.fn();
const deleteJSON = vi.fn();

vi.mock('@/services/request', () => ({
  getJSON: (...args: unknown[]) => getJSON(...args),
  getWithParams: (...args: unknown[]) => getWithParams(...args),
  postJSON: (...args: unknown[]) => postJSON(...args),
  putJSON: (...args: unknown[]) => putJSON(...args),
  deleteJSON: (...args: unknown[]) => deleteJSON(...args),
}));

import {
  batchMarkBuyerMsgDraftsSent,
  buyerMsgDraftStatusLabel,
  buyerMsgNodeLabel,
  createBuyerMsgRule,
  deleteBuyerMsgRule,
  generateBuyerMsgDrafts,
  ignoreBuyerMsgDraft,
  markBuyerMsgDraftSent,
  queryBuyerMsgDrafts,
  queryBuyerMsgRules,
  updateBuyerMsgDraft,
  updateBuyerMsgRule,
} from '../customer';

beforeEach(() => {
  getJSON.mockReset().mockResolvedValue({ list: [], canWrite: true });
  getWithParams.mockReset().mockResolvedValue({ list: [], total: 0, canWrite: true });
  postJSON.mockReset().mockResolvedValue({ ok: true });
  putJSON.mockReset().mockResolvedValue({ ok: true });
  deleteJSON.mockReset().mockResolvedValue({ ok: true });
});

describe('buyer message node/status labels', () => {
  it('maps every node to Chinese copy', () => {
    expect(buyerMsgNodeLabel('paid')).toBe('已付款');
    expect(buyerMsgNodeLabel('shipped')).toBe('已发货');
    expect(buyerMsgNodeLabel('delivered')).toBe('已签收');
    expect(buyerMsgNodeLabel('logistics_exception')).toBe('物流异常');
    expect(buyerMsgNodeLabel('refunded')).toBe('退款');
    expect(buyerMsgNodeLabel('unknown')).toBe('unknown');
  });

  it('maps every draft status to Chinese copy', () => {
    expect(buyerMsgDraftStatusLabel('pending')).toBe('待发送');
    expect(buyerMsgDraftStatusLabel('sent')).toBe('已发送');
    expect(buyerMsgDraftStatusLabel('ignored')).toBe('已忽略');
  });
});

describe('buyer message rule services', () => {
  it('lists rules via GET', async () => {
    await queryBuyerMsgRules();
    expect(getJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-message-rules');
  });

  it('creates via POST with node/template payload', async () => {
    await createBuyerMsgRule({ name: '发货通知', node: 'shipped', templateId: 'tpl-1' });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-message-rules', {
      name: '发货通知',
      node: 'shipped',
      templateId: 'tpl-1',
    });
  });

  it('updates via PUT on the id path', async () => {
    await updateBuyerMsgRule('rule-1', { enabled: false });
    expect(putJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-message-rules/rule-1', {
      enabled: false,
    });
  });

  it('deletes via DELETE on the id path', async () => {
    await deleteBuyerMsgRule('rule-2');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-message-rules/rule-2');
  });
});

describe('buyer message draft services', () => {
  it('queries drafts with node/status/shop/keyword filters', async () => {
    await queryBuyerMsgDrafts({
      page: 2,
      pageSize: 20,
      node: 'shipped',
      status: 'pending',
      shopId: 'shop-1',
      keyword: 'DEMO',
    });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/drafts', {
      page: 2,
      pageSize: 20,
      node: 'shipped',
      status: 'pending',
      platform: undefined,
      shopId: 'shop-1',
      keyword: 'DEMO',
    });
  });

  it('triggers generation via POST', async () => {
    await generateBuyerMsgDrafts();
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/generate', {});
  });

  it('edits draft content via PUT', async () => {
    await updateBuyerMsgDraft('d-1', '您好，您的订单已发货');
    expect(putJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/drafts/d-1', {
      content: '您好，您的订单已发货',
    });
  });

  it('marks a single draft sent via POST', async () => {
    await markBuyerMsgDraftSent('d-2');
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/drafts/d-2/mark-sent', {});
  });

  it('ignores a draft via POST', async () => {
    await ignoreBuyerMsgDraft('d-3');
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/drafts/d-3/ignore', {});
  });

  it('batch marks drafts sent via POST with ids', async () => {
    await batchMarkBuyerMsgDraftsSent(['a', 'b']);
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/buyer-messages/drafts/batch-mark-sent', {
      ids: ['a', 'b'],
    });
  });
});

import { describe, expect, it } from 'vitest';

import {
  CUSTOMER_MESSAGE_ROLE_LABEL,
  CUSTOMER_MESSAGE_SOURCE_LABEL,
  CUSTOMER_MESSAGE_TYPE_LABEL,
  ORDER_EXCEPTION_STATUS,
  ORDER_PAYMENT_STATUS,
  purchasePayChannelLabel,
} from '../status';

describe('status enum label maps (R147 raw enum cleanup)', () => {
  it('maps purchase order payment statuses to Chinese', () => {
    expect(ORDER_PAYMENT_STATUS.unpaid.text).toBe('未支付');
    expect(ORDER_PAYMENT_STATUS.paid.text).toBe('已支付');
  });

  it('maps purchase pay channels with raw fallback', () => {
    expect(purchasePayChannelLabel('manual')).toBe('手动标记');
    expect(purchasePayChannelLabel('alipay')).toBe('支付宝');
    expect(purchasePayChannelLabel('bank')).toBe('对公转账');
    expect(purchasePayChannelLabel('other')).toBe('其他');
    expect(purchasePayChannelLabel('wechat')).toBe('wechat');
    expect(purchasePayChannelLabel('')).toBe('-');
    expect(purchasePayChannelLabel(null)).toBe('-');
  });

  it('maps order exception statuses to Chinese', () => {
    expect(ORDER_EXCEPTION_STATUS.open.text).toBe('待处理');
    expect(ORDER_EXCEPTION_STATUS.handled.text).toBe('已处理');
    expect(ORDER_EXCEPTION_STATUS.ignored.text).toBe('已忽略');
  });

  it('maps customer message role / source / type to Chinese', () => {
    expect(CUSTOMER_MESSAGE_ROLE_LABEL.customer).toBe('买家');
    expect(CUSTOMER_MESSAGE_ROLE_LABEL.agent).toBe('客服');
    expect(CUSTOMER_MESSAGE_SOURCE_LABEL.platform).toBe('平台同步');
    expect(CUSTOMER_MESSAGE_SOURCE_LABEL.ai_suggestion).toBe('AI 建议');
    expect(CUSTOMER_MESSAGE_TYPE_LABEL.text).toBe('文本');
    expect(CUSTOMER_MESSAGE_TYPE_LABEL.system).toBe('系统');
  });
});

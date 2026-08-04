import { describe, expect, it } from 'vitest';
import {
  REPLY_TEMPLATE_GROUPS,
  fillReplyTemplate,
  replyTemplateGroupLabel,
} from '../replyTemplateVars';

describe('replyTemplateGroupLabel', () => {
  it('maps group keys to Chinese labels', () => {
    expect(replyTemplateGroupLabel('presale')).toBe('售前');
    expect(replyTemplateGroupLabel('aftersale')).toBe('售后');
    expect(replyTemplateGroupLabel('logistics')).toBe('物流');
    expect(replyTemplateGroupLabel('refund')).toBe('退款');
    expect(replyTemplateGroupLabel('other')).toBe('其他');
  });

  it('falls back to the raw key for unknown groups', () => {
    expect(replyTemplateGroupLabel('bogus')).toBe('bogus');
  });

  it('covers all five groups', () => {
    expect(REPLY_TEMPLATE_GROUPS.map((g) => g.key)).toEqual([
      'presale',
      'aftersale',
      'logistics',
      'refund',
      'other',
    ]);
  });
});

describe('fillReplyTemplate', () => {
  it('replaces variables from the conversation context', () => {
    const { text, missing } = fillReplyTemplate('您好{买家昵称}，订单 {订单号} 已发货，单号 {物流单号}。', {
      买家昵称: 'Alice',
      订单号: 'SO-1001',
      物流单号: 'SF12345',
    });
    expect(text).toBe('您好Alice，订单 SO-1001 已发货，单号 SF12345。');
    expect(missing).toEqual([]);
  });

  it('keeps unresolved placeholders and reports them once', () => {
    const { text, missing } = fillReplyTemplate('{买家昵称}您好，订单 {订单号}，再次确认 {订单号}。', {
      买家昵称: 'Bob',
    });
    expect(text).toBe('Bob您好，订单 {订单号}，再次确认 {订单号}。');
    expect(missing).toEqual(['订单号']);
  });

  it('treats empty-string context values as missing', () => {
    const { text, missing } = fillReplyTemplate('欢迎光临{店铺名}', { 店铺名: '  ' });
    expect(text).toBe('欢迎光临{店铺名}');
    expect(missing).toEqual(['店铺名']);
  });

  it('leaves text without placeholders unchanged', () => {
    const { text, missing } = fillReplyTemplate('感谢惠顾！', {});
    expect(text).toBe('感谢惠顾！');
    expect(missing).toEqual([]);
  });
});

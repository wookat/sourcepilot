/** 话术模板分组（与后端 customerchat 分组口径一致，services/customer 从此处复用） */
export type ReplyTemplateGroupKey = 'presale' | 'aftersale' | 'logistics' | 'refund' | 'other';

export const REPLY_TEMPLATE_GROUPS: { key: ReplyTemplateGroupKey; label: string }[] = [
  { key: 'presale', label: '售前' },
  { key: 'aftersale', label: '售后' },
  { key: 'logistics', label: '物流' },
  { key: 'refund', label: '退款' },
  { key: 'other', label: '其他' },
];

export function replyTemplateGroupLabel(key: string): string {
  return REPLY_TEMPLATE_GROUPS.find((g) => g.key === key)?.label || key;
}

export type ReplyTemplateVarContext = {
  买家昵称?: string;
  订单号?: string;
  物流单号?: string;
  商品名?: string;
  店铺名?: string;
};

export const REPLY_TEMPLATE_VAR_KEYS: (keyof ReplyTemplateVarContext)[] = [
  '买家昵称',
  '订单号',
  '物流单号',
  '商品名',
  '店铺名',
];

export type FillTemplateResult = {
  text: string;
  missing: string[];
};

// 用当前会话上下文替换 {变量} 占位；缺失的变量保留原样并返回提醒列表。
export function fillReplyTemplate(
  content: string,
  ctx: ReplyTemplateVarContext,
): FillTemplateResult {
  const missing: string[] = [];
  const text = content.replace(/\{([^{}]+)\}/g, (raw, name: string) => {
    const key = name.trim() as keyof ReplyTemplateVarContext;
    const value = ctx[key];
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      return String(value);
    }
    if (!missing.includes(name.trim())) {
      missing.push(name.trim());
    }
    return raw;
  });
  return { text, missing };
}

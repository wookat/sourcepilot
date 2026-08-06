import type { OrderItemRow, OrderSkuMatchRow } from '@/services/orders';

export type LocalSkuCodeDisplay =
  | { bound: true; text: string }
  | { bound: false };

/**
 * 订单行「本地规格编号」列展示口径：
 * 仅在明细行已绑定本地 SKU（行或匹配记录带 productSkuId）时展示编号；
 * 未绑定时不回显录入的 skuCode，避免把录入编码误读为已绑定的本地规格。
 */
export function localSkuCodeDisplay(
  row: Pick<OrderItemRow, 'productSkuId' | 'skuCode'>,
  match?: Pick<OrderSkuMatchRow, 'productSkuId' | 'localSkuCode'>,
): LocalSkuCodeDisplay {
  const bound = Boolean(row.productSkuId?.trim() || match?.productSkuId?.trim());
  if (!bound) return { bound: false };
  return { bound: true, text: match?.localSkuCode || row.skuCode || '—' };
}

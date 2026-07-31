export type ParsedImportLine = {
  line: number;
  raw: string;
  orderNo?: string;
  customerName?: string;
  productTitle?: string;
  skuCode?: string;
  quantity?: number;
  unitPrice?: number;
  currency?: string;
  error?: string;
};

export type ParsedImportOrder = {
  orderNo: string;
  customerName: string;
  currency: string;
  totalAmount: number;
  items: {
    productTitle: string;
    skuCode: string;
    quantity: number;
    unitPrice: number;
    totalPrice: number;
  }[];
};

export function splitImportLine(raw: string): string[] {
  return raw.split(/[\t,，;；|]/).map((s) => s.trim());
}

const CURRENCY_RE = /^[A-Za-z]{3}$/;

/**
 * Parse pasted text for batch order import.
 * Each line: 订单号,客户名,商品标题,SKU编码,数量,单价[,币种]
 * Lines sharing the same 订单号 become one order with multiple items.
 */
export function parseImportText(text: string): ParsedImportLine[] {
  const out: ParsedImportLine[] = [];
  const lines = text.split(/\r?\n/);
  lines.forEach((raw, i) => {
    const trimmed = raw.trim();
    if (!trimmed) return;
    const parts = splitImportLine(trimmed);
    const item: ParsedImportLine = { line: i + 1, raw: trimmed };
    if (parts.length < 6) {
      item.error = '格式：订单号,客户名,商品标题,SKU编码,数量,单价[,币种]（逗号/Tab 分隔）';
      out.push(item);
      return;
    }
    if (!parts[0]) {
      item.error = '订单号不能为空';
      out.push(item);
      return;
    }
    const qty = Number(parts[4]);
    if (!parts[4] || !Number.isInteger(qty) || qty < 1) {
      item.error = '数量需为正整数';
      out.push(item);
      return;
    }
    const price = Number(parts[5]);
    if (!parts[5] || !Number.isFinite(price) || price < 0) {
      item.error = '单价需为非负数字';
      out.push(item);
      return;
    }
    if (parts.length >= 7 && parts[6] && !CURRENCY_RE.test(parts[6])) {
      item.error = '币种需为 3 位字母代码（如 USD / CNY）';
      out.push(item);
      return;
    }
    item.orderNo = parts[0];
    item.customerName = parts[1];
    item.productTitle = parts[2];
    item.skuCode = parts[3];
    item.quantity = qty;
    item.unitPrice = price;
    item.currency = parts.length >= 7 && parts[6] ? parts[6].toUpperCase() : undefined;
    out.push(item);
  });
  return out;
}

/** Group valid parsed lines into orders (per orderNo, keeping first-seen order). */
export function groupImportOrders(lines: ParsedImportLine[]): ParsedImportOrder[] {
  const byNo = new Map<string, ParsedImportOrder>();
  const order: string[] = [];
  for (const l of lines) {
    if (l.error || !l.orderNo) continue;
    let o = byNo.get(l.orderNo);
    if (!o) {
      o = {
        orderNo: l.orderNo,
        customerName: l.customerName || '',
        currency: l.currency || 'USD',
        totalAmount: 0,
        items: [],
      };
      byNo.set(l.orderNo, o);
      order.push(l.orderNo);
    }
    const total = Math.round((l.quantity || 0) * (l.unitPrice || 0) * 100) / 100;
    o.items.push({
      productTitle: l.productTitle || '',
      skuCode: l.skuCode || '',
      quantity: l.quantity || 0,
      unitPrice: l.unitPrice || 0,
      totalPrice: total,
    });
    o.totalAmount = Math.round((o.totalAmount + total) * 100) / 100;
  }
  return order.map((no) => byNo.get(no) as ParsedImportOrder);
}

export type ParsedPlacedLine = {
  line: number;
  raw: string;
  purchaseOrderId?: string;
  externalOrderId?: string;
  error?: string;
};

export type ParsedLogisticsLine = {
  line: number;
  raw: string;
  externalOrderId?: string;
  trackingNo?: string;
  carrier?: string;
  error?: string;
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const PREFIX_RE = /^[0-9a-f]{4,32}$/i;

export function splitLine(raw: string): string[] {
  return raw
    .split(/[\t,，;；|]+|\s{2,}| /)
    .map((s) => s.trim())
    .filter(Boolean);
}

/** Resolve a full UUID or a unique hex prefix against a candidate id list. */
export function resolvePurchaseOrderId(
  token: string,
  candidateIds: string[],
): { id?: string; error?: string } {
  const t = token.trim().toLowerCase();
  if (UUID_RE.test(t)) return { id: t };
  if (!PREFIX_RE.test(t)) return { error: '采购单号格式不正确（需为完整 ID 或 ID 前缀）' };
  const matched = candidateIds.filter((id) => id.toLowerCase().startsWith(t));
  if (matched.length === 1) return { id: matched[0] };
  if (matched.length === 0) return { error: '未在待回填采购单中找到该前缀' };
  return { error: `前缀匹配到 ${matched.length} 张采购单，请使用更长前缀` };
}

/**
 * Parse pasted text for batch mark-placed.
 * Each line: <purchaseOrderId or prefix> <1688 externalOrderId>
 */
export function parsePlacedText(text: string, candidateIds: string[]): ParsedPlacedLine[] {
  const out: ParsedPlacedLine[] = [];
  const lines = text.split(/\r?\n/);
  lines.forEach((raw, i) => {
    const trimmed = raw.trim();
    if (!trimmed) return;
    const parts = splitLine(trimmed);
    const item: ParsedPlacedLine = { line: i + 1, raw: trimmed };
    if (parts.length < 2) {
      item.error = '格式：采购单号 1688订单号（空格/逗号/Tab 分隔）';
      out.push(item);
      return;
    }
    const resolved = resolvePurchaseOrderId(parts[0], candidateIds);
    if (resolved.error) {
      item.error = resolved.error;
      out.push(item);
      return;
    }
    item.purchaseOrderId = resolved.id;
    item.externalOrderId = parts[1];
    out.push(item);
  });
  return out;
}

/**
 * Parse pasted text for batch logistics backfill.
 * Each line: <1688 externalOrderId> <trackingNo> [carrier]
 */
export function parseLogisticsText(text: string): ParsedLogisticsLine[] {
  const out: ParsedLogisticsLine[] = [];
  const lines = text.split(/\r?\n/);
  lines.forEach((raw, i) => {
    const trimmed = raw.trim();
    if (!trimmed) return;
    const parts = splitLine(trimmed);
    const item: ParsedLogisticsLine = { line: i + 1, raw: trimmed };
    if (parts.length < 2) {
      item.error = '格式：1688订单号 运单号 [承运商]（空格/逗号/Tab 分隔）';
      out.push(item);
      return;
    }
    item.externalOrderId = parts[0];
    item.trackingNo = parts[1];
    if (parts.length >= 3) item.carrier = parts.slice(2).join(' ');
    out.push(item);
  });
  return out;
}

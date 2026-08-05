/** 极简 CSV 构建与下载（UTF-8 BOM，兼容 Excel 中文）。 */

function escapeCell(v: string | number | null | undefined): string {
  if (v === null || v === undefined) return '';
  const s = String(v);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function buildCSV(rows: (string | number | null | undefined)[][]): string {
  return rows.map((r) => r.map(escapeCell).join(',')).join('\r\n');
}

export function downloadCSV(filename: string, rows: (string | number | null | undefined)[][]) {
  const blob = new Blob([`\ufeff${buildCSV(rows)}`], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

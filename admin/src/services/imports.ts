import { getWithParams, postFormData, postJSON } from '@/services/request';
import { responseErrorMessage } from '@/utils/httpErrorCopy';
import { fetchWithSessionGuard } from '@/utils/sessionGuard';

/** 迁移导入字段定义（后端 FieldDef） */
export type ImportFieldDef = {
  key: string;
  label: string;
  required: boolean;
};

/** POST /api/v1/imports/parse 响应 */
export type ImportParseResult = {
  kind: 'product' | 'order';
  fileName: string;
  fileHash: string;
  sourceFormat: 'dianxiaomi' | 'mabang' | 'custom';
  columns: string[];
  rows: string[][];
  totalRows: number;
  mapping: Record<string, number>;
  fields: ImportFieldDef[];
};

/** validate / commit 公共请求体 */
export type ImportWizardBody = {
  kind: 'product' | 'order';
  shopId: string;
  columns: string[];
  rows: string[][];
  mapping: Record<string, number>;
  fileName?: string;
  fileHash?: string;
  sourceFormat?: string;
};

export type ImportRowError = {
  rowNumber: number;
  field?: string;
  message: string;
};

/** POST /api/v1/imports/validate 响应 */
export type ImportValidateResult = {
  totalRows: number;
  validRows: number;
  errorRows: number;
  groupCount: number;
  errors: ImportRowError[];
};

/** POST /api/v1/imports/commit 响应 */
export type ImportCommitResult = {
  jobId: string;
  status: 'success' | 'partial_success' | 'failed';
  totalRows: number;
  successRows: number;
  failedRows: number;
  duplicateRows: number;
  replayed: boolean;
};

/** GET /api/v1/imports 单条历史 */
export type ImportJobRow = {
  id: string;
  kind: 'product' | 'order';
  batchKey: string;
  shopId?: string;
  sourceFormat: string;
  fileName: string;
  status: 'success' | 'partial_success' | 'failed';
  totalRows: number;
  successRows: number;
  failedRows: number;
  duplicateRows: number;
  errorRowCount: number;
  createdAt: string;
  updatedAt: string;
};

/** GET /api/v1/imports/:id 错误行 */
export type ImportJobErrorRow = {
  id: string;
  jobId: string;
  rowNumber: number;
  status: 'failed' | 'duplicate';
  field?: string;
  message: string;
  rawValues?: Record<string, string>;
};

export async function parseImportFile(kind: 'product' | 'order', file: File): Promise<ImportParseResult> {
  const form = new FormData();
  form.append('kind', kind);
  form.append('file', file);
  return postFormData<ImportParseResult>('/api/v1/imports/parse', form);
}

export async function validateImport(body: ImportWizardBody): Promise<ImportValidateResult> {
  return postJSON<ImportValidateResult>('/api/v1/imports/validate', body);
}

export async function commitImport(body: ImportWizardBody): Promise<ImportCommitResult> {
  return postJSON<ImportCommitResult>('/api/v1/imports/commit', body);
}

export async function queryImportJobs(params: {
  page?: number;
  pageSize?: number;
  kind?: string;
}): Promise<{ list: ImportJobRow[]; total: number; page: number; pageSize: number }> {
  return getWithParams('/api/v1/imports', params);
}

export async function getImportJob(id: string): Promise<{ job: ImportJobRow; errorRows: ImportJobErrorRow[] }> {
  return getWithParams(`/api/v1/imports/${id}`, {});
}

export async function downloadImportErrorsCsv(id: string) {
  const resp = await fetchWithSessionGuard(`/api/v1/imports/${id}/errors.csv`);
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `import-errors-${id.slice(0, 8)}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

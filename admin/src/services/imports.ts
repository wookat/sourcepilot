import { deleteJSON, getWithParams, postFormData, postJSON } from '@/services/request';
import { responseErrorMessage } from '@/utils/httpErrorCopy';
import { fetchWithSessionGuard } from '@/utils/sessionGuard';

/** 导入类型：商品 / 订单 / 库存期初 / 货源档案 */
export type ImportKind = 'product' | 'order' | 'inventory' | 'source';

/** 迁移导入字段定义（后端 FieldDef） */
export type ImportFieldDef = {
  key: string;
  label: string;
  required: boolean;
};

/** POST /api/v1/imports/parse 响应 */
export type ImportParseResult = {
  kind: ImportKind;
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
  kind: ImportKind;
  /** 商品 / 订单导入必选；库存 / 货源导入为租户级可留空 */
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
  kind: ImportKind;
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

export async function parseImportFile(kind: ImportKind, file: File): Promise<ImportParseResult> {
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

async function downloadCsv(url: string, fileName: string) {
  const resp = await fetchWithSessionGuard(url);
  if (!resp.ok) {
    throw new Error(await responseErrorMessage(resp));
  }
  const blob = await resp.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(objectUrl);
}

export async function downloadImportErrorsCsv(id: string) {
  return downloadCsv(`/api/v1/imports/${id}/errors.csv`, `import-errors-${id.slice(0, 8)}.csv`);
}

/** GET /api/v1/imports/templates/:kind — 下载通用导入模板 */
export async function downloadImportTemplateCsv(kind: ImportKind) {
  return downloadCsv(`/api/v1/imports/templates/${kind}`, `trademind-import-template-${kind}.csv`);
}

/** GET /api/v1/imports/export/:kind — 全量数据 CSV 导出 */
export async function downloadExportCsv(kind: ImportKind) {
  return downloadCsv(`/api/v1/imports/export/${kind}`, `trademind-export-${kind}.csv`);
}

/** 租户级列映射方案 */
export type ImportMappingPreset = {
  id: string;
  kind: ImportKind;
  name: string;
  columns?: string[];
  mapping: Record<string, number>;
  createdAt: string;
  updatedAt: string;
};

export async function queryImportMappingPresets(kind: ImportKind): Promise<{ list: ImportMappingPreset[] }> {
  return getWithParams('/api/v1/imports/mappings', { kind });
}

export async function saveImportMappingPreset(body: {
  kind: ImportKind;
  name: string;
  columns: string[];
  mapping: Record<string, number>;
}): Promise<ImportMappingPreset> {
  return postJSON<ImportMappingPreset>('/api/v1/imports/mappings', body);
}

export async function deleteImportMappingPreset(id: string): Promise<{ deleted: boolean }> {
  return deleteJSON<{ deleted: boolean }>(`/api/v1/imports/mappings/${id}`);
}

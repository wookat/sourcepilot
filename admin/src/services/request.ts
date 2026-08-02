import { request } from '@umijs/max';

/** 后端统一返回结构（与 Gin Envelope 对齐） */
export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
};

export type RequestOptions = {
  headers?: Record<string, string>;
};

function withOptions(options?: RequestOptions) {
  return options?.headers ? { headers: options.headers } : {};
}

export class ApiRequestError extends Error {
  code: number;
  traceId?: string;
  data: unknown;

  constructor(res: ApiResponse<unknown>) {
    super(res.message || '请求失败，请稍后重试');
    this.name = 'ApiRequestError';
    this.code = res.code;
    this.traceId = res.traceId;
    this.data = res.data;
  }
}

function unwrap<T>(res: ApiResponse<T>): T {
  if (res.code !== 0) {
    throw new ApiRequestError(res as ApiResponse<unknown>);
  }
  return res.data;
}

/** 非 2xx 响应（axios 抛错）时提取后端 Envelope 的中文 message，避免透出英文 axios 原文 */
function normalizeRequestError(err: unknown): never {
  const body = (err as { response?: { data?: unknown } })?.response?.data;
  if (body && typeof body === 'object' && 'code' in body && 'message' in body) {
    const res = body as ApiResponse<unknown>;
    if (typeof res.message === 'string' && res.message) {
      throw new ApiRequestError(res);
    }
  }
  throw err;
}

async function requestEnvelope<T>(
  path: string,
  options: Record<string, unknown>,
): Promise<ApiResponse<T>> {
  try {
    return await request<ApiResponse<T>>(path, options);
  } catch (err: unknown) {
    normalizeRequestError(err);
  }
}

/** 通用 GET（后续各模块拆分到独立 service 文件） */
export async function getJSON<T>(path: string): Promise<T> {
  const res = await requestEnvelope<T>(path, { method: 'GET' });
  return unwrap(res);
}

/** 通用 PUT */
export async function putJSON<T, B extends object = object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'PUT',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 PATCH */
export async function patchJSON<T, B extends object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'PATCH',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 POST */
export async function postJSON<T, B extends object = object>(path: string, body?: B, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'POST',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** GET with query params */
export async function getWithParams<T>(
  path: string,
  params?: Record<string, string | number | boolean | undefined>,
): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'GET',
    params,
  });
  return unwrap(res);
}

/** DELETE */
export async function deleteJSON<T>(path: string): Promise<T> {
  const res = await requestEnvelope<T>(path, { method: 'DELETE' });
  return unwrap(res);
}

/** multipart/form-data（如上传）；由 request 识别 FormData，勿手动设 Content-Type */
export async function postFormData<T>(path: string, data: FormData): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'POST',
    data,
  });
  return unwrap(res);
}

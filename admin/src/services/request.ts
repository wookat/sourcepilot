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
    super(res.message || 'request_failed');
    this.name = 'ApiRequestError';
    this.code = res.code;
    this.traceId = res.traceId;
    this.data = res.data;
  }
}

/**
 * 提取用户可读的接口错误信息：优先取后端 envelope 的结构化 message
 * （业务错误 ApiRequestError，或非 2xx 时 axios error.response.data.message），
 * 避免向用户展示 "Request failed with status code 403" 这类英文技术文案。
 */
export function extractApiErrorMessage(e: unknown, fallback: string): string {
  if (e instanceof ApiRequestError) {
    return e.message || fallback;
  }
  const ax = e as {
    response?: { data?: { message?: string } };
    message?: string;
  } | null;
  const envelopeMessage = (ax?.response?.data?.message ?? '').trim();
  if (envelopeMessage) return envelopeMessage;
  const raw = (ax?.message ?? '').trim();
  if (raw && !/request failed|network error|timeout/i.test(raw)) return raw;
  return fallback;
}

function unwrap<T>(res: ApiResponse<T>): T {
  if (res.code !== 0) {
    throw new ApiRequestError(res as ApiResponse<unknown>);
  }
  return res.data;
}

/** 通用 GET（后续各模块拆分到独立 service 文件） */
export async function getJSON<T>(path: string): Promise<T> {
  const res = await request<ApiResponse<T>>(path, { method: 'GET' });
  return unwrap(res);
}

/** 通用 PUT */
export async function putJSON<T, B extends object = object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'PUT',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 PATCH */
export async function patchJSON<T, B extends object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'PATCH',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 POST */
export async function postJSON<T, B extends object = object>(path: string, body?: B, options?: RequestOptions): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
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
  const res = await request<ApiResponse<T>>(path, {
    method: 'GET',
    params,
  });
  return unwrap(res);
}

/** DELETE */
export async function deleteJSON<T>(path: string): Promise<T> {
  const res = await request<ApiResponse<T>>(path, { method: 'DELETE' });
  return unwrap(res);
}

/** multipart/form-data（如上传）；由 request 识别 FormData，勿手动设 Content-Type */
export async function postFormData<T>(path: string, data: FormData): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'POST',
    data,
  });
  return unwrap(res);
}

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

function unwrap<T>(res: ApiResponse<T>): T {
  if (res.code !== 0) {
    throw new ApiRequestError(res as ApiResponse<unknown>);
  }
  return res.data;
}

function isApiEnvelope(body: unknown): body is ApiResponse<unknown> {
  return (
    typeof body === 'object' &&
    body !== null &&
    typeof (body as { code?: unknown }).code === 'number' &&
    typeof (body as { message?: unknown }).message === 'string'
  );
}

/**
 * HTTP 非 2xx 时 axios 抛的是 "Request failed with status code xxx"，
 * 会丢掉后端 envelope 里的可行动 message（如「请配置 base_url」）。
 * 这里统一还原为 ApiRequestError，让页面 message.error(e.message) 展示后端文案。
 */
function normalizeRequestError(error: unknown): never {
  const body = (error as { response?: { data?: unknown } })?.response?.data;
  if (isApiEnvelope(body) && body.message) {
    throw new ApiRequestError(body);
  }
  throw error;
}

async function requestJSON<T>(path: string, options: Record<string, unknown>): Promise<T> {
  try {
    const res = await request<ApiResponse<T>>(path, options);
    return unwrap(res);
  } catch (error) {
    normalizeRequestError(error);
  }
}

/** 通用 GET（后续各模块拆分到独立 service 文件） */
export async function getJSON<T>(path: string): Promise<T> {
  return requestJSON<T>(path, { method: 'GET' });
}

/** 通用 PUT */
export async function putJSON<T, B extends object = object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  return requestJSON<T>(path, {
    method: 'PUT',
    data: body,
    ...withOptions(options),
  });
}

/** 通用 PATCH */
export async function patchJSON<T, B extends object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  return requestJSON<T>(path, {
    method: 'PATCH',
    data: body,
    ...withOptions(options),
  });
}

/** 通用 POST */
export async function postJSON<T, B extends object = object>(path: string, body?: B, options?: RequestOptions): Promise<T> {
  return requestJSON<T>(path, {
    method: 'POST',
    data: body,
    ...withOptions(options),
  });
}

/** GET with query params */
export async function getWithParams<T>(
  path: string,
  params?: Record<string, string | number | boolean | undefined>,
): Promise<T> {
  return requestJSON<T>(path, {
    method: 'GET',
    params,
  });
}

/** DELETE */
export async function deleteJSON<T>(path: string): Promise<T> {
  return requestJSON<T>(path, { method: 'DELETE' });
}

/** multipart/form-data（如上传）；由 request 识别 FormData，勿手动设 Content-Type */
export async function postFormData<T>(path: string, data: FormData): Promise<T> {
  return requestJSON<T>(path, {
    method: 'POST',
    data,
  });
}

import { history } from '@umijs/max';
import { Button, Tooltip, Typography, notification } from 'antd';

/** 后端 AI 调用失败时随 envelope data 下发的结构化错误码（见 docs/api.md） */
export type AIErrorCode =
  | 'AI_NOT_CONFIGURED'
  | 'AI_INVALID_KEY'
  | 'AI_FORBIDDEN'
  | 'AI_MODEL_NOT_FOUND'
  | 'AI_BAD_BASE_URL'
  | 'AI_QUOTA_EXCEEDED'
  | 'AI_UPSTREAM_ERROR'
  | 'AI_TIMEOUT'
  | 'AI_BAD_RESPONSE';

const AI_ERROR_COPY: Record<AIErrorCode, string> = {
  AI_NOT_CONFIGURED: '尚未配置 AI 服务（API Key / base_url），请先在 AI 设置中完成配置',
  AI_INVALID_KEY: 'API Key 无效或未授权，请在 AI 设置中检查密钥',
  AI_FORBIDDEN: '当前账号无权限访问该模型，请在 AI 设置中更换模型或检查账号权限',
  AI_MODEL_NOT_FOUND: '模型不存在或无权限，请在 AI 设置中检查模型名称',
  AI_BAD_BASE_URL: 'base_url 不可访问或接口路径错误，请在 AI 设置中检查服务地址',
  AI_QUOTA_EXCEEDED: '请求过于频繁或额度不足，请稍后重试或检查服务商配额',
  AI_UPSTREAM_ERROR: 'AI 服务商暂时不可用，请稍后重试',
  AI_TIMEOUT: 'AI 请求超时，请稍后重试或在 AI 设置中调大超时时间',
  AI_BAD_RESPONSE: 'AI 服务响应格式不兼容，请在 AI 设置中检查服务商与模型配置',
};

/** 将任务 errorMessage 中的结构化 AI 错误码映射为中文原因；无法识别时原样返回 */
export function mapAiTaskErrorText(raw?: string | null): string {
  const msg = (raw ?? '').trim();
  if (!msg) return '';
  for (const code of Object.keys(AI_ERROR_COPY) as AIErrorCode[]) {
    if (msg.includes(code)) return AI_ERROR_COPY[code];
  }
  return msg;
}

/** 列表单元格：中文化失败原因 + 悬浮查看完整原始原因 */
export function AiTaskErrorText({ raw }: { raw?: string | null }) {
  const msg = (raw ?? '').trim();
  if (!msg) return <>—</>;
  const mapped = mapAiTaskErrorText(msg);
  return (
    <Tooltip title={msg}>
      <Typography.Text
        ellipsis
        style={{ maxWidth: '100%', display: 'inline-block', verticalAlign: 'bottom' }}
      >
        {mapped}
      </Typography.Text>
    </Tooltip>
  );
}

type ApiEnvelope = {
  code?: number;
  message?: string;
  data?: { errorCode?: string } | null;
  traceId?: string;
};

export type ExtractedAIError = {
  /** 结构化错误码（后端已分类时存在） */
  errorCode?: AIErrorCode;
  /** 面向用户的中文原因 */
  reason?: string;
  traceId?: string;
};

/** axios 英文兜底文案（如 "Request failed with status code 400"）不得直接展示给用户 */
function isAxiosGenericMessage(msg: string): boolean {
  return /^request failed with status code \d+$/i.test(msg.trim()) || /^network error$/i.test(msg.trim());
}

function pickEnvelope(error: unknown): ApiEnvelope | undefined {
  if (!error || typeof error !== 'object') return undefined;
  // umi-request / axios 风格：error.response.data 为后端 envelope
  const resp = (error as { response?: { data?: unknown } }).response;
  const data = resp?.data;
  if (data && typeof data === 'object' && 'code' in data) {
    return data as ApiEnvelope;
  }
  // request.ts 的 ApiRequestError：message/code/data 直接在 error 上
  if ('code' in error && 'message' in error) {
    const e = error as { code: number; message: string; data?: unknown; traceId?: string };
    return { code: e.code, message: e.message, data: e.data as ApiEnvelope['data'], traceId: e.traceId };
  }
  return undefined;
}

/**
 * 从任意请求异常中提取后端 envelope 的中文原因与结构化 AI 错误码，
 * 屏蔽 axios 英文兜底文案（如「Request failed with status code 400」）。
 */
export function extractAIError(error: unknown): ExtractedAIError {
  const env = pickEnvelope(error);
  const rawCode = env?.data && typeof env.data === 'object' ? env.data.errorCode : undefined;
  const errorCode = rawCode && rawCode in AI_ERROR_COPY ? (rawCode as AIErrorCode) : undefined;
  let reason: string | undefined;
  if (errorCode) {
    reason = AI_ERROR_COPY[errorCode];
  } else if (env?.message && !isAxiosGenericMessage(env.message)) {
    reason = env.message;
  } else if (error instanceof Error && error.message && !isAxiosGenericMessage(error.message)) {
    reason = error.message;
  }
  return { errorCode, reason, traceId: env?.traceId };
}

/**
 * AI 能力失败的常驻提示：错误详情需要用户读完并跳转 AI 设置排查，
 * 用 notification（不自动消失）替代一闪而过的 message.error。
 */
export function notifyAIFailure(opts: {
  title: string;
  error?: unknown;
  fallback?: string;
  showSettingsLink?: boolean;
}) {
  const { title, error, fallback, showSettingsLink = true } = opts;
  const { reason } = extractAIError(error);
  const description = reason || fallback || '请求失败，请检查 AI 设置或稍后重试';
  const key = `ai-failure-${Date.now()}`;
  notification.error({
    key,
    message: title,
    description,
    duration: 0,
    btn: showSettingsLink ? (
      <Button
        type="primary"
        size="small"
        onClick={() => {
          notification.destroy(key);
          history.push('/settings/ai');
        }}
      >
        去 AI 设置
      </Button>
    ) : undefined,
  });
}

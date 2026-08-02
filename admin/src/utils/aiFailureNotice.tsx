import { history } from '@umijs/max';
import { Button, notification } from 'antd';

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
  const key = `ai-failure-${Date.now()}`;
  notification.error({
    key,
    message: title,
    description: (error as Error)?.message || fallback || '请求失败，请稍后重试',
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

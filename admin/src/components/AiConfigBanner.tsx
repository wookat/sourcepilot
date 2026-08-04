import { history } from '@umijs/max';
import { Alert, Button } from 'antd';
import type { CSSProperties } from 'react';
import { useEffect, useState } from 'react';
import { fetchSettingsList } from '@/services/settings';
import { usePermission } from '@/hooks/usePermission';
import { aiConfiguredFromSettings } from '@/utils/aiConfigStatus';

/**
 * 「AI 未配置」统一提示条：新手入门 / 采集 / AI 页面共用同一文案与跳转。
 * 仅设置管理权限账号探测并展示（可自行去配置）；无权限账号不探测，避免 403 噪音。
 */
export default function AiConfigBanner({ style }: { style?: CSSProperties }) {
  const { canManageSettings } = usePermission();
  const [unconfigured, setUnconfigured] = useState(false);

  useEffect(() => {
    if (!canManageSettings) return undefined;
    let cancelled = false;
    fetchSettingsList()
      .then((res) => {
        if (!cancelled) setUnconfigured(!aiConfiguredFromSettings(res.items));
      })
      .catch(() => {
        /* 探测失败不展示，避免误报 */
      });
    return () => {
      cancelled = true;
    };
  }, [canManageSettings]);

  if (!unconfigured) return null;

  return (
    <Alert
      type="warning"
      showIcon
      message="AI 服务尚未配置"
      description="标题优化、描述生成、图片处理等 AI 能力暂不可用。请先在「系统设置 → AI 设置」填写服务地址与 API Key。"
      action={
        <Button size="small" type="primary" onClick={() => history.push('/settings/ai')}>
          前往 AI 设置
        </Button>
      }
      style={{ marginBottom: 16, ...style }}
    />
  );
}

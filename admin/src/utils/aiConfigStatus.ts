import { pickGroup } from '@/utils/settingsForm';

type SettingItems = Parameters<typeof pickGroup>[0];

/** 与新手入门「配置 AI 与存储」步骤同口径：当前 provider 的 API Key 已填即视为已配置 */
export function aiConfiguredFromSettings(items: SettingItems): boolean {
  const ai = pickGroup(items, 'ai');
  const provider = (ai.provider || 'openai_compatible').trim();
  return Boolean((ai[`${provider}_api_key`] || ai.api_key || '').trim());
}

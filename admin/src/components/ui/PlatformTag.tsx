import { Tag } from 'antd';
import { platformDisplayLabel } from '@/constants/platformLabels';

/** 平台 → Tag 品牌色（antd 预设色） */
const PLATFORM_TAG_COLOR: Record<string, string> = {
  douyin_shop: 'volcano',
  tiktok: 'cyan',
  shopee: 'orange',
  lazada: 'geekblue',
  amazon: 'gold',
  mock: 'default',
  manual: 'default',
};

export type PlatformTagProps = {
  platform?: string | null;
  className?: string;
};

/** 平台语义 Tag：内部枚举 → 中文名 + 品牌色，整体不换行 */
export default function PlatformTag({ platform, className }: PlatformTagProps) {
  const k = (platform ?? '').trim().toLowerCase();
  if (!k) return <>—</>;
  return (
    <Tag
      color={PLATFORM_TAG_COLOR[k] ?? 'default'}
      className={className}
      style={{ whiteSpace: 'nowrap', marginInlineEnd: 0 }}
    >
      {platformDisplayLabel(platform)}
    </Tag>
  );
}

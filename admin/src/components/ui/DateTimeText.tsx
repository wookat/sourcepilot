import { Tooltip } from 'antd';
import { formatDateTime, formatDateTimeShort, type DateTimeInput } from '@/utils/formatTime';

export type DateTimeTextProps = {
  value?: DateTimeInput;
  fallback?: string;
};

/** 列表时间列统一短格式（MM-DD HH:mm），Tooltip 展示完整时间；详情页仍用 formatDateTime */
export default function DateTimeText({ value, fallback = '—' }: DateTimeTextProps) {
  const short = formatDateTimeShort(value, fallback);
  if (short === fallback) return <>{fallback}</>;
  return (
    <Tooltip title={formatDateTime(value, fallback)}>
      <span style={{ fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>{short}</span>
    </Tooltip>
  );
}

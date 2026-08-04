import { DatePicker, Segmented, Space } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback } from 'react';
import { useSearchParams } from '@umijs/max';
import { mergeQueryState } from '@/utils/urlState';

const DAY_OPTIONS = [
  { label: '近 7 天', value: 7 },
  { label: '近 30 天', value: 30 },
  { label: '近 90 天', value: 90 },
];

export const DEFAULT_REPORT_DAYS = 30;

export type ReportRange = { days?: number; start?: string; end?: string };

/** URL `?days=`/`?start=&end=` 归一化：优先自定义区间，其次 7/30/90，非法回落默认 */
export function normalizeReportRange(params: URLSearchParams): ReportRange {
  const start = params.get('start') ?? '';
  const end = params.get('end') ?? '';
  if (
    /^\d{4}-\d{2}-\d{2}$/.test(start) &&
    /^\d{4}-\d{2}-\d{2}$/.test(end) &&
    !dayjs(end).isBefore(dayjs(start))
  ) {
    return { start, end };
  }
  const n = Number(params.get('days'));
  return { days: DAY_OPTIONS.some((o) => o.value === n) ? n : DEFAULT_REPORT_DAYS };
}

export function reportRangeLabel(range: ReportRange): string {
  if (range.start && range.end) {
    return `${range.start} ~ ${range.end}`;
  }
  return `近 ${range.days ?? DEFAULT_REPORT_DAYS} 天`;
}

/** 报表时间范围控件：7/30/90 天快捷切换 + 自定义日期区间，状态同步到 URL */
export function RangeControls() {
  const [searchParams] = useSearchParams();
  const range = normalizeReportRange(searchParams);

  const setDays = useCallback((next: number) => {
    mergeQueryState(
      { days: next === DEFAULT_REPORT_DAYS ? undefined : next, start: undefined, end: undefined },
      { replace: true },
    );
  }, []);

  const setCustom = useCallback((values: [Dayjs | null, Dayjs | null] | null) => {
    const [s, e] = values ?? [null, null];
    if (s && e) {
      mergeQueryState(
        { start: s.format('YYYY-MM-DD'), end: e.format('YYYY-MM-DD'), days: undefined },
        { replace: true },
      );
    } else {
      mergeQueryState({ start: undefined, end: undefined }, { replace: true });
    }
  }, []);

  return (
    <Space wrap>
      <Segmented
        options={DAY_OPTIONS}
        value={range.start ? undefined : range.days}
        onChange={(v) => setDays(v as number)}
        aria-label="统计天数"
      />
      <DatePicker.RangePicker
        allowClear
        value={range.start && range.end ? [dayjs(range.start), dayjs(range.end)] : null}
        onChange={setCustom}
        placeholder={['开始日期', '结束日期']}
        aria-label="自定义区间"
      />
    </Space>
  );
}

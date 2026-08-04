import { DownloadOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { Alert, Button, Col, Row, Segmented, Skeleton, Space, Statistic, Typography, message } from 'antd';
import { Suspense, lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from '@umijs/max';
import { mergeQueryState } from '@/utils/urlState';
import { EmptyState, TmPageContainer } from '@/components/ui';
import {
  chartAxisXLabel,
  chartAxisXTickCount,
  chartTokens,
  formatAmount,
  formatCount,
  formatDateTickShort,
  makeCategoryLabelFilter,
  tabularNumsStyle,
} from '@/constants/chartTokens';
import { downloadDailyReportCsv, fetchOrderDailyStats, type DailyStatsDTO } from '@/services/orders';
import { useWideScreen } from '@/hooks/useWideScreen';

/** 图表库体积大，懒加载使页面壳先渲染，加载期间用 Skeleton 占位 */
const Line = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Line })));
const Column = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Column })));

const DAY_OPTIONS = [
  { label: '近 7 天', value: 7 },
  { label: '近 30 天', value: 30 },
  { label: '近 90 天', value: 90 },
];

const DEFAULT_DAYS = 30;

/** URL `?days=` 归一化：仅接受 7/30/90，非法值回落默认，刷新/分享链接保持所选天数 */
export function normalizeReportDays(raw: string | null): number {
  const n = Number(raw);
  return DAY_OPTIONS.some((o) => o.value === n) ? n : DEFAULT_DAYS;
}

type CountPoint = { date: string; type: string; value: number };
type AmountPoint = { date: string; currency: string; amount: number };
type BaseAmountPoint = { date: string; amount: number };

function toCountPoints(res: DailyStatsDTO): CountPoint[] {
  const out: CountPoint[] = [];
  for (const it of res.items ?? []) {
    out.push({ date: it.date, type: '订单数', value: it.orderCount });
    out.push({ date: it.date, type: '已付款数', value: it.paidCount });
  }
  return out;
}

function toAmountPoints(res: DailyStatsDTO): AmountPoint[] {
  const out: AmountPoint[] = [];
  for (const it of res.items ?? []) {
    for (const a of it.paidAmounts ?? []) {
      out.push({ date: it.date, currency: a.currency, amount: a.amount });
    }
  }
  return out;
}

function toBaseAmountPoints(res: DailyStatsDTO): BaseAmountPoint[] {
  return (res.items ?? []).map((it) => ({ date: it.date, amount: it.paidAmountBase ?? 0 }));
}

/** 经营报表：近 N 天按日订单趋势（口径与首页经营概览 stats/sales 一致），支持导出 CSV */
export default function OrderReports() {
  const [searchParams] = useSearchParams();
  const days = normalizeReportDays(searchParams.get('days'));
  const setDays = useCallback((next: number) => {
    mergeQueryState({ days: next === DEFAULT_DAYS ? undefined : next }, { replace: true });
  }, []);
  const [data, setData] = useState<DailyStatsDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const wideScreen = useWideScreen();
  const chartHeight = wideScreen ? chartTokens.height : chartTokens.heightCompact;
  const xTickCount = wideScreen ? chartAxisXTickCount.wide : chartAxisXTickCount.compact;

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchOrderDailyStats(days)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, [days]);

  const exportCsv = useCallback(() => {
    setExporting(true);
    downloadDailyReportCsv(days)
      .then(() => message.success('已导出 CSV'))
      .catch(() => message.error('导出失败，请稍后重试'))
      .finally(() => setExporting(false));
  }, [days]);

  useEffect(() => {
    load();
  }, [load]);

  const items = data?.items ?? [];
  const baseCurrency = data?.baseCurrency || 'CNY';
  const totals = useMemo(() => {
    let orders = 0;
    let paid = 0;
    let base = 0;
    const byCurrency = new Map<string, number>();
    const unconverted = new Set<string>();
    for (const it of items) {
      orders += it.orderCount;
      paid += it.paidCount;
      base += it.paidAmountBase ?? 0;
      for (const cur of it.unconvertedCurrencies ?? []) {
        unconverted.add(cur);
      }
      for (const a of it.paidAmounts ?? []) {
        byCurrency.set(a.currency, (byCurrency.get(a.currency) ?? 0) + a.amount);
      }
    }
    return {
      orders,
      paid,
      base,
      unconverted: [...unconverted].sort(),
      byCurrency: [...byCurrency.entries()].sort((x, y) => (x[0] < y[0] ? -1 : 1)),
    };
  }, [items]);

  const hasData = totals.orders > 0;
  const hasPaidAmounts = totals.byCurrency.length > 0;
  const countPoints = useMemo(() => (data ? toCountPoints(data) : []), [data]);
  const amountPoints = useMemo(() => (data ? toAmountPoints(data) : []), [data]);
  const baseAmountPoints = useMemo(() => (data ? toBaseAmountPoints(data) : []), [data]);
  const makeAxisX = useCallback(
    (dateCount: number) => ({
      ...chartAxisXLabel,
      tickCount: xTickCount,
      labelFilter: makeCategoryLabelFilter(dateCount, xTickCount),
      labelFormatter: formatDateTickShort,
    }),
    [xTickCount],
  );
  const countAxisX = useMemo(() => makeAxisX(items.length), [makeAxisX, items.length]);
  const amountAxisX = useMemo(
    () => makeAxisX(new Set(amountPoints.map((p) => p.date)).size),
    [makeAxisX, amountPoints],
  );

  return (
    <TmPageContainer
      title="经营报表"
      subTitle={`近 ${days} 天按日趋势，统计口径与首页经营概览一致`}
      extra={
        <Space wrap className="tm-page-header-extra">
          <Segmented
            options={DAY_OPTIONS}
            value={days}
            onChange={(v) => setDays(v as number)}
            aria-label="统计天数"
          />
          <Button icon={<DownloadOutlined />} loading={exporting} onClick={exportCsv}>
            导出 CSV
          </Button>
        </Space>
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          message="报表加载失败"
          description={error}
          action={<Typography.Link onClick={load}>重试</Typography.Link>}
          style={{ marginBottom: 16 }}
        />
      ) : null}

      {!loading && totals.unconverted.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`以下币种未配置汇率，未折算入本位币合计：${totals.unconverted.join('、')}`}
          description={
            <span>
              前往 <Link to="/settings/report-currency">报表本位币与汇率设置</Link> 配置汇率后，该部分金额会计入折算合计。
            </span>
          }
          style={{ marginBottom: 16 }}
        />
      ) : null}

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`近 ${days} 天合计（本位币 ${baseCurrency}）`}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="订单数" value={totals.orders} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="已付款订单" value={totals.paid} valueStyle={tabularNumsStyle} />
            </Col>
            {hasPaidAmounts ? (
              <Col xs={12} sm={8} md={6}>
                <Statistic
                  title={`销售额折算合计（${baseCurrency}）${totals.unconverted.length > 0 ? '，含未折算币种' : ''}`}
                  value={formatAmount(totals.base)}
                  valueStyle={tabularNumsStyle}
                />
              </Col>
            ) : null}
            {totals.byCurrency.map(([currency, amount]) => (
              <Col xs={12} sm={8} md={6} key={currency}>
                <Statistic
                  title={`原币销售额（${currency}）${totals.unconverted.includes(currency) ? '，未折算' : ''}`}
                  value={formatAmount(amount)}
                  valueStyle={tabularNumsStyle}
                />
              </Col>
            ))}
            {totals.byCurrency.length === 0 ? (
              <Col xs={12} sm={8} md={6}>
                <Statistic title="销售额" value="暂无已付款订单" valueStyle={{ fontSize: 16 }} />
              </Col>
            ) : null}
          </Row>
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="每日订单数 / 已付款数">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : hasData ? (
          <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
            <Line
              data={countPoints}
              xField="date"
              yField="value"
              colorField="type"
              height={chartHeight}
              autoFit
              scale={{ color: { range: [...chartTokens.seriesColors] } }}
              axis={{
                x: countAxisX,
                y: { labelFormatter: (v: number) => formatCount(Number(v)) },
              }}
              legend={{ color: { position: 'top' } }}
              tooltip={{ items: [{ channel: 'y', valueFormatter: (v: number) => formatCount(Number(v)) }] }}
            />
          </Suspense>
        ) : (
          <EmptyState
            compact
            title={`近 ${days} 天暂无订单`}
            description="创建或导入订单后，这里会展示按日趋势"
            actionLabel="去订单列表"
            actionPath="/orders/list"
          />
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`每日销售额折算（${baseCurrency}，已付款）`}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : hasPaidAmounts ? (
          <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
            <Column
              data={baseAmountPoints}
              xField="date"
              yField="amount"
              height={chartHeight}
              autoFit
              style={{ maxWidth: chartTokens.barMaxWidth }}
              scale={{ color: { range: [...chartTokens.seriesColors] } }}
              axis={{
                x: countAxisX,
                y: { labelFormatter: (v: number) => formatCount(Number(v)) },
              }}
              tooltip={{
                items: [(d: BaseAmountPoint) => ({ name: baseCurrency, value: formatAmount(d.amount) })],
              }}
            />
          </Suspense>
        ) : (
          <EmptyState
            compact
            title={`近 ${days} 天暂无已付款订单`}
            description="订单完成付款后，这里会展示按本位币折算的每日销售额"
          />
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="每日销售额（原币明细，已付款，按币种）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : amountPoints.length > 0 ? (
          <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
            <Column
              data={amountPoints}
              xField="date"
              yField="amount"
              colorField="currency"
              stack
              height={chartHeight}
              autoFit
              style={{ maxWidth: chartTokens.barMaxWidth }}
              scale={{ color: { range: [...chartTokens.seriesColors] } }}
              axis={{
                x: amountAxisX,
                y: { labelFormatter: (v: number) => formatCount(Number(v)) },
              }}
              legend={{ color: { position: 'top' } }}
              tooltip={{
                items: [(d: AmountPoint) => ({ name: d.currency, value: formatAmount(d.amount) })],
              }}
            />
          </Suspense>
        ) : (
          <EmptyState
            compact
            title={`近 ${days} 天暂无已付款订单`}
            description="订单完成付款后，这里会按币种展示每日销售额"
          />
        )}
      </ProCard>
    </TmPageContainer>
  );
}

import { DownloadOutlined } from '@ant-design/icons';
import { Column, Line } from '@ant-design/plots';
import { ProCard } from '@ant-design/pro-components';
import { Alert, Button, Col, Row, Segmented, Skeleton, Space, Statistic, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { chartTokens, formatAmount, formatCount, tabularNumsStyle } from '@/constants/chartTokens';
import { downloadDailyReportCsv, fetchOrderDailyStats, type DailyStatsDTO } from '@/services/orders';

const DAY_OPTIONS = [
  { label: '近 7 天', value: 7 },
  { label: '近 30 天', value: 30 },
  { label: '近 90 天', value: 90 },
];

type CountPoint = { date: string; type: string; value: number };
type AmountPoint = { date: string; currency: string; amount: number };

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

/** 经营报表：近 N 天按日订单趋势（口径与首页经营概览 stats/sales 一致），支持导出 CSV */
export default function OrderReports() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<DailyStatsDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

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
  const totals = useMemo(() => {
    let orders = 0;
    let paid = 0;
    const byCurrency = new Map<string, number>();
    for (const it of items) {
      orders += it.orderCount;
      paid += it.paidCount;
      for (const a of it.paidAmounts ?? []) {
        byCurrency.set(a.currency, (byCurrency.get(a.currency) ?? 0) + a.amount);
      }
    }
    return { orders, paid, byCurrency: [...byCurrency.entries()].sort((x, y) => (x[0] < y[0] ? -1 : 1)) };
  }, [items]);

  const hasData = totals.orders > 0;
  const countPoints = useMemo(() => (data ? toCountPoints(data) : []), [data]);
  const amountPoints = useMemo(() => (data ? toAmountPoints(data) : []), [data]);

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

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`近 ${days} 天合计`}>
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
            {totals.byCurrency.map(([currency, amount]) => (
              <Col xs={12} sm={8} md={6} key={currency}>
                <Statistic title={`销售额（${currency}）`} value={amount} precision={2} valueStyle={tabularNumsStyle} />
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
          <Line
            data={countPoints}
            xField="date"
            yField="value"
            colorField="type"
            height={chartTokens.height}
            autoFit
            scale={{ color: { range: [...chartTokens.seriesColors] } }}
            axis={{
              x: { labelAutoRotate: true, labelAutoHide: true },
              y: { labelFormatter: (v: number) => formatCount(Number(v)) },
            }}
            legend={{ color: { position: 'top' } }}
            tooltip={{ items: [{ channel: 'y', valueFormatter: (v: number) => formatCount(Number(v)) }] }}
          />
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

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="每日销售额（已付款，按币种）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : amountPoints.length > 0 ? (
          <Column
            data={amountPoints}
            xField="date"
            yField="amount"
            colorField="currency"
            stack
            height={chartTokens.height}
            autoFit
            scale={{ color: { range: [...chartTokens.seriesColors] } }}
            axis={{
              x: { labelAutoRotate: true, labelAutoHide: true },
              y: { labelFormatter: (v: number) => formatCount(Number(v)) },
            }}
            legend={{ color: { position: 'top' } }}
            tooltip={{
              items: [(d: AmountPoint) => ({ name: d.currency, value: formatAmount(d.amount) })],
            }}
          />
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

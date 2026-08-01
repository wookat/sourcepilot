import { Column, Line } from '@ant-design/plots';
import { ProCard } from '@ant-design/pro-components';
import { Alert, Col, Row, Skeleton, Statistic, Typography } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { fetchOrderDailyStats, type DailyStatsDTO } from '@/services/orders';

const DAYS = 30;

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

/** 经营报表：近 30 天按日订单趋势（口径与首页经营概览 stats/sales 一致） */
export default function OrderReports() {
  const [data, setData] = useState<DailyStatsDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchOrderDailyStats(DAYS)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, []);

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
    <TmPageContainer title="经营报表" subTitle={`近 ${DAYS} 天按日趋势，统计口径与首页经营概览一致`}>
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

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`近 ${DAYS} 天合计`}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="订单数" value={totals.orders} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="已付款订单" value={totals.paid} />
            </Col>
            {totals.byCurrency.map(([currency, amount]) => (
              <Col xs={12} sm={8} md={6} key={currency}>
                <Statistic title={`销售额（${currency}）`} value={amount} precision={2} />
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
            height={300}
            autoFit
            axis={{ x: { labelAutoRotate: true, labelAutoHide: true } }}
          />
        ) : (
          <EmptyState
            compact
            title="近 30 天暂无订单"
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
            height={300}
            autoFit
            axis={{ x: { labelAutoRotate: true, labelAutoHide: true } }}
          />
        ) : (
          <EmptyState
            compact
            title="近 30 天暂无已付款订单"
            description="订单完成付款后，这里会按币种展示每日销售额"
          />
        )}
      </ProCard>
    </TmPageContainer>
  );
}

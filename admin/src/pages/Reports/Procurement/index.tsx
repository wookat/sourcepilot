import { ProCard } from '@ant-design/pro-components';
import { Alert, Col, Row, Skeleton, Statistic, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { Suspense, lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from '@umijs/max';
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
import {
  fetchProcurementReport,
  type ProcurementDaily,
  type ProcurementReportDTO,
  type SupplierAgg,
} from '@/services/reports';
import { useWideScreen } from '@/hooks/useWideScreen';
import { RangeControls, normalizeReportRange, reportRangeLabel } from '../components/RangeControls';

const Column = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Column })));

/** 采购报表：金额/单量/在途/签收时效分布与供应商排行（口径：采购单创建时间，CNY） */
export default function ProcurementReport() {
  const [searchParams] = useSearchParams();
  const range = normalizeReportRange(searchParams);

  const [data, setData] = useState<ProcurementReportDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const wideScreen = useWideScreen();
  const chartHeight = wideScreen ? chartTokens.height : chartTokens.heightCompact;
  const xTickCount = wideScreen ? chartAxisXTickCount.wide : chartAxisXTickCount.compact;

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchProcurementReport(range)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.days, range.start, range.end]);

  useEffect(() => {
    load();
  }, [load]);

  const summary = data?.summary;
  const daily = data?.daily ?? [];
  const leadTime = data?.leadTime ?? [];
  const suppliers = data?.suppliers ?? [];
  const hasData = (summary?.poCount ?? 0) > 0;
  const hasLeadTime = (summary?.leadTimeSamples ?? 0) > 0;

  const dailyAxisX = useMemo(
    () => ({
      ...chartAxisXLabel,
      tickCount: xTickCount,
      labelFilter: makeCategoryLabelFilter(daily.length, xTickCount),
      labelFormatter: formatDateTickShort,
    }),
    [xTickCount, daily.length],
  );

  const supplierColumns = useMemo<ColumnsType<SupplierAgg>>(
    () => [
      {
        title: '供应商',
        dataIndex: 'supplierName',
        ellipsis: true,
        render: (v: string) => v || '未知供应商',
      },
      { title: '采购单量', dataIndex: 'poCount', width: 100, align: 'right' },
      {
        title: '采购金额(CNY)',
        dataIndex: 'amount',
        width: 140,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      { title: '已签收', dataIndex: 'deliveredCount', width: 90, align: 'right' },
      {
        title: '平均签收时效(天)',
        dataIndex: 'avgLeadTimeDays',
        width: 140,
        align: 'right',
        render: (v?: number) => (v == null ? '-' : v),
      },
    ],
    [],
  );

  return (
    <TmPageContainer
      title="采购报表"
      subTitle={`${reportRangeLabel(range)}，按采购单创建时间统计（CNY）`}
      extra={<RangeControls />}
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

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="合计">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="采购单量" value={summary?.poCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="采购金额（CNY，不含已取消）"
                value={formatAmount(summary?.totalAmount ?? 0)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="在途单量" value={summary?.inTransitCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="已签收单量" value={summary?.deliveredCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="平均签收时效（下单→签收，天）"
                value={summary?.avgLeadTimeDays == null ? '暂无签收样本' : summary.avgLeadTimeDays}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="已取消单量" value={summary?.cancelledCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
          </Row>
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="每日采购金额（CNY）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : hasData ? (
          <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
            <Column
              data={daily}
              xField="date"
              yField="amount"
              height={chartHeight}
              autoFit
              style={{ maxWidth: chartTokens.barMaxWidth }}
              scale={{ color: { range: [...chartTokens.seriesColors] } }}
              axis={{
                x: dailyAxisX,
                y: { labelFormatter: (v: number) => formatCount(Number(v)) },
              }}
              tooltip={{
                items: [(d: ProcurementDaily) => ({ name: 'CNY', value: formatAmount(d.amount) })],
              }}
            />
          </Suspense>
        ) : (
          <EmptyState
            compact
            title={`${reportRangeLabel(range)}暂无采购单`}
            description="创建采购单后，这里会展示每日采购金额"
            actionLabel="去采购单列表"
            actionPath="/procurement/orders"
          />
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="签收时效分布（下单→签收天数）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : hasLeadTime ? (
          <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
            <Column
              data={leadTime}
              xField="label"
              yField="count"
              height={chartHeight}
              autoFit
              style={{ maxWidth: chartTokens.barMaxWidth }}
              scale={{ color: { range: [...chartTokens.seriesColors] } }}
              axis={{ y: { labelFormatter: (v: number) => formatCount(Number(v)) } }}
              tooltip={{
                items: [
                  (d: { label: string; count: number }) => ({ name: d.label, value: formatCount(d.count) }),
                ],
              }}
            />
          </Suspense>
        ) : (
          <EmptyState compact title="暂无签收样本" description="采购单签收后，这里会展示下单到签收的天数分布" />
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="供应商排行（按采购金额）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : suppliers.length > 0 ? (
          <Table<SupplierAgg>
            rowKey="supplierId"
            size="small"
            columns={supplierColumns}
            dataSource={suppliers}
            scroll={{ x: 640 }}
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        ) : (
          <EmptyState compact title="暂无供应商数据" description="创建采购单后，这里会展示按供应商聚合的排行" />
        )}
      </ProCard>
    </TmPageContainer>
  );
}

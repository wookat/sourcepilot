import { DownloadOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Row,
  Segmented,
  Skeleton,
  Space,
  Statistic,
  Table,
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { Suspense, lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from '@umijs/max';
import { mergeQueryState } from '@/utils/urlState';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { chartTokens, formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import {
  downloadProfitReportCsv,
  fetchProfitReport,
  type ProfitDimension,
  type ProfitReportDTO,
  type ProfitRow,
} from '@/services/reports';
import { useWideScreen } from '@/hooks/useWideScreen';
import { RangeControls, normalizeReportRange, reportRangeLabel } from '../components/RangeControls';

const Column = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Column })));

const DIMENSION_OPTIONS = [
  { label: '按订单', value: 'order' },
  { label: '按商品', value: 'product' },
  { label: '按店铺', value: 'shop' },
];

const DIMENSION_COL_TITLE: Record<ProfitDimension, string> = {
  order: '订单号',
  product: '商品',
  shop: '店铺',
};

export function normalizeProfitDimension(raw: string | null): ProfitDimension {
  return raw === 'product' || raw === 'shop' ? raw : 'order';
}

function renderBaseAmount(v?: number): string {
  return v == null ? '未折算' : formatAmount(v);
}

/** 利润报表：订单/商品/店铺三维度，收入-采购成本-费用=毛利，多币种折算本位币 */
export default function ProfitReport() {
  const [searchParams] = useSearchParams();
  const range = normalizeReportRange(searchParams);
  const dimension = normalizeProfitDimension(searchParams.get('dimension'));
  const setDimension = useCallback((next: ProfitDimension) => {
    mergeQueryState({ dimension: next === 'order' ? undefined : next }, { replace: true });
  }, []);

  const [data, setData] = useState<ProfitReportDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const wideScreen = useWideScreen();
  const chartHeight = wideScreen ? chartTokens.height : chartTokens.heightCompact;

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchProfitReport(dimension, range)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dimension, range.days, range.start, range.end]);

  useEffect(() => {
    load();
  }, [load]);

  const exportCsv = useCallback(() => {
    setExporting(true);
    downloadProfitReportCsv(dimension, range)
      .then(() => message.success('已导出 CSV'))
      .catch(() => message.error('导出失败，请稍后重试'))
      .finally(() => setExporting(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dimension, range.days, range.start, range.end]);

  const base = data?.baseCurrency || 'CNY';
  const summary = data?.summary;
  const rows = data?.rows ?? [];
  const unconverted = summary?.unconvertedCurrencies ?? [];
  const hasData = (summary?.orderCount ?? 0) > 0;

  const chartPoints = useMemo(
    () =>
      rows
        .filter((r) => r.grossProfitBase != null)
        .slice(0, 10)
        .map((r) => ({ label: r.label, profit: r.grossProfitBase as number })),
    [rows],
  );

  const columns = useMemo<ColumnsType<ProfitRow>>(() => {
    const cols: ColumnsType<ProfitRow> = [
      {
        title: DIMENSION_COL_TITLE[dimension],
        dataIndex: 'label',
        ellipsis: true,
        render: (v: string, r) => (
          <Tooltip title={v}>
            <span>
              {v}
              {r.platform ? <Typography.Text type="secondary">（{r.platform}）</Typography.Text> : null}
            </span>
          </Tooltip>
        ),
      },
      { title: '订单数', dataIndex: 'orderCount', width: 88, align: 'right' },
      {
        title: '原币收入',
        dataIndex: 'revenue',
        width: 160,
        render: (v: ProfitRow['revenue']) =>
          (v ?? []).map((a) => `${a.currency} ${formatAmount(a.amount)}`).join(' / ') || '-',
      },
      {
        title: `收入(${base})`,
        dataIndex: 'revenueBase',
        width: 120,
        align: 'right',
        render: (v: number, r) => (
          <span style={tabularNumsStyle}>
            {formatAmount(v)}
            {(r.unconvertedCurrencies?.length ?? 0) > 0 ? (
              <Tooltip title={`未折算币种：${r.unconvertedCurrencies?.join('、')}`}>
                <Typography.Text type="warning">*</Typography.Text>
              </Tooltip>
            ) : null}
          </span>
        ),
      },
      {
        title: '采购成本(CNY)',
        dataIndex: 'costCny',
        width: 130,
        align: 'right',
        render: (v: number, r) => (
          <span style={tabularNumsStyle}>
            {formatAmount(v)}
            {r.missingCostLines > 0 ? (
              <Tooltip title={`${r.missingCostLines} 行缺参考进价，成本偏低`}>
                <Typography.Text type="warning">*</Typography.Text>
              </Tooltip>
            ) : null}
          </span>
        ),
      },
      {
        title: `成本(${base})`,
        dataIndex: 'costBase',
        width: 110,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{renderBaseAmount(v)}</span>,
      },
      {
        title: `费用(${base})`,
        dataIndex: 'feeBase',
        width: 110,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      {
        title: `毛利(${base})`,
        dataIndex: 'grossProfitBase',
        width: 110,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{renderBaseAmount(v)}</span>,
      },
      {
        title: '毛利率',
        dataIndex: 'marginPercent',
        width: 90,
        align: 'right',
        render: (v?: number) => (v == null ? '未折算' : `${v}%`),
      },
    ];
    if (dimension !== 'order') {
      cols.splice(2, 0, { title: '件数', dataIndex: 'quantity', width: 80, align: 'right' });
    }
    return cols;
  }, [dimension, base]);

  return (
    <TmPageContainer
      title="利润报表"
      subTitle={`${reportRangeLabel(range)}，已付款订单口径，折算本位币 ${base}`}
      extra={
        <Space wrap className="tm-page-header-extra">
          <Segmented
            options={DIMENSION_OPTIONS}
            value={dimension}
            onChange={(v) => setDimension(v as ProfitDimension)}
            aria-label="统计维度"
          />
          <RangeControls />
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

      {!loading && unconverted.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`以下币种未配置汇率，未折算入本位币合计：${unconverted.join('、')}`}
          description={
            <span>
              前往 <Link to="/settings/report-currency">报表本位币与汇率设置</Link> 配置汇率后，该部分金额会计入折算合计。
            </span>
          }
          style={{ marginBottom: 16 }}
        />
      ) : null}

      {!loading && (summary?.missingCostLines ?? 0) > 0 ? (
        <Alert
          type="info"
          showIcon
          message={`${summary?.missingCostLines} 个订单行缺少参考进价，采购成本按可估算行合计（成本偏低）`}
          description={
            <span>
              前往 <Link to="/sourcing/product-sources">商品货源档案</Link> 补齐主货源 SKU 映射与进价后口径更准确。
            </span>
          }
          style={{ marginBottom: 16 }}
        />
      ) : null}

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`合计（本位币 ${base}）`}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="已付款订单数" value={summary?.orderCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title={`收入折算（${base}）${unconverted.length > 0 ? '，不含未折算币种' : ''}`}
                value={formatAmount(summary?.revenueBase ?? 0)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="采购成本（CNY）"
                value={formatAmount(summary?.costCny ?? 0)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title={`费用（${base}）${(data?.feeItems?.length ?? 0) === 0 ? '，未配置费用项' : ''}`}
                value={formatAmount(summary?.feeBase ?? 0)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title={`毛利（${base}）`}
                value={renderBaseAmount(summary?.grossProfitBase)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="毛利率"
                value={summary?.marginPercent == null ? '未折算' : `${summary.marginPercent}%`}
                valueStyle={tabularNumsStyle}
              />
            </Col>
          </Row>
        )}
      </ProCard>

      {dimension !== 'order' ? (
        <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`毛利 Top 10（${base}）`}>
          {loading ? (
            <Skeleton active paragraph={{ rows: 6 }} />
          ) : chartPoints.length > 0 ? (
            <Suspense fallback={<Skeleton active paragraph={{ rows: 6 }} />}>
              <Column
                data={chartPoints}
                xField="label"
                yField="profit"
                height={chartHeight}
                autoFit
                style={{ maxWidth: chartTokens.barMaxWidth }}
                scale={{ color: { range: [...chartTokens.seriesColors] } }}
                axis={{ x: { labelAutoEllipsis: true, labelAutoRotate: true } }}
                tooltip={{
                  items: [
                    (d: { label: string; profit: number }) => ({ name: base, value: formatAmount(d.profit) }),
                  ],
                }}
              />
            </Suspense>
          ) : (
            <EmptyState compact title="暂无可折算毛利数据" description="配置汇率与参考进价后展示毛利分布" />
          )}
        </ProCard>
      ) : null}

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="明细">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : hasData ? (
          <>
            {data?.truncated ? (
              <Alert type="info" showIcon message="明细行数较多，仅展示前 500 行；完整数据请导出 CSV" style={{ marginBottom: 12 }} />
            ) : null}
            <Table<ProfitRow>
              rowKey="key"
              size="small"
              columns={columns}
              dataSource={rows}
              scroll={{ x: 960 }}
              pagination={{ pageSize: 20, showSizeChanger: false }}
            />
          </>
        ) : (
          <EmptyState
            compact
            title={`${reportRangeLabel(range)}暂无已付款订单`}
            description="订单完成付款后，这里会展示利润明细"
            actionLabel="去订单列表"
            actionPath="/orders/list"
          />
        )}
      </ProCard>
    </TmPageContainer>
  );
}

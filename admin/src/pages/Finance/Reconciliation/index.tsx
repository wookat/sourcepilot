import { DownloadOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Row,
  Segmented,
  Skeleton,
  Space,
  Statistic,
  Table,
  Tag,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { Link, useSearchParams } from '@umijs/max';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import { mergeQueryState, appendSourceToUrl } from '@/utils/urlState';
import {
  SETTLEMENT_LABEL,
  downloadReconciliationCsv,
  fetchReconciliation,
  type OrderFinance,
  type ReconStatusFilter,
  type ReconciliationDTO,
  type SettlementStatus,
} from '@/services/finance';
import { RangeControls, normalizeReportRange, reportRangeLabel } from '../../Reports/components/RangeControls';

const FILTER_OPTIONS = [
  { label: '全部', value: '' },
  { label: '异常项', value: 'flagged' },
  { label: '差异较大', value: 'large_diff' },
  { label: '未回款', value: 'unpaid' },
  { label: '少款', value: 'short' },
  { label: '多款', value: 'over' },
  { label: '已结清', value: 'settled' },
];

export function normalizeReconStatus(raw: string | null): ReconStatusFilter {
  return FILTER_OPTIONS.some((o) => o.value === raw) ? (raw as ReconStatusFilter) : '';
}

function baseText(v?: number): string {
  return v == null ? '未折算' : formatAmount(v);
}

/** 对账差异工作台：回款差异 + 实算 vs 估算毛利差异，按差异绝对值排序 */
export default function FinanceReconciliation() {
  const [searchParams] = useSearchParams();
  const range = normalizeReportRange(searchParams);
  const status = normalizeReconStatus(searchParams.get('status'));
  const setStatus = useCallback((next: ReconStatusFilter) => {
    mergeQueryState({ status: next || undefined }, { replace: true });
  }, []);

  const [data, setData] = useState<ReconciliationDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchReconciliation(range, status)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.days, range.start, range.end, status]);

  useEffect(() => {
    load();
  }, [load]);

  const exportCsv = useCallback(() => {
    setExporting(true);
    downloadReconciliationCsv(range, status)
      .then(() => message.success('已导出 CSV'))
      .catch(() => message.error('导出失败，请稍后重试'))
      .finally(() => setExporting(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.days, range.start, range.end, status]);

  const base = data?.baseCurrency || 'CNY';
  const summary = data?.summary;
  const rows = data?.rows ?? [];

  const columns = useMemo<ColumnsType<OrderFinance>>(
    () => [
      {
        title: '订单号',
        dataIndex: 'orderNo',
        width: 180,
        fixed: 'left',
        render: (v: string, r) => (
          <Link to={appendSourceToUrl(`/orders/${r.orderId}`, 'finance-reconciliation')}>{v}</Link>
        ),
      },
      { title: '店铺', dataIndex: 'shopName', width: 140, render: (v?: string) => v || '—' },
      { title: '币种', dataIndex: 'currency', width: 80 },
      {
        title: '应收',
        dataIndex: 'receivable',
        width: 110,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      {
        title: '已回款',
        dataIndex: 'received',
        width: 110,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      {
        title: '回款差异',
        dataIndex: 'diffAmount',
        width: 110,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      {
        title: '对账状态',
        dataIndex: 'settlementStatus',
        width: 100,
        render: (v: SettlementStatus, r) => {
          const s = SETTLEMENT_LABEL[v] ?? { text: v, color: 'default' };
          return (
            <Space size={4}>
              <Tag color={s.color}>{s.text}</Tag>
              {r.largeDiff ? <Tag color="red">差异较大</Tag> : null}
            </Space>
          );
        },
      },
      {
        title: `实算毛利（${base}）`,
        dataIndex: 'actualProfitBase',
        width: 140,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `估算毛利（${base}）`,
        dataIndex: 'estimatedProfitBase',
        width: 140,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `毛利差异（${base}）`,
        dataIndex: 'profitDiffBase',
        width: 140,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: '缺实付行',
        dataIndex: 'missingActualLines',
        width: 100,
        align: 'right',
        render: (v: number) => (v > 0 ? <Tag color="orange">{v}</Tag> : '—'),
      },
    ],
    [base],
  );

  return (
    <TmPageContainer
      title="对账差异工作台"
      subTitle="回款差异与实算/估算毛利差异集中处理"
      extra={
        <Space wrap>
          <RangeControls />
          <Button icon={<DownloadOutlined />} loading={exporting} onClick={exportCsv} disabled={!rows.length}>
            导出 CSV
          </Button>
        </Space>
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="对账数据加载失败"
          description={error}
          action={<Button size="small" onClick={load}>重试</Button>}
        />
      ) : null}
      <Card size="small" style={{ marginBottom: 16 }}>
        {loading && !data ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="订单数" value={summary?.orderCount ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="未回款" value={summary?.unpaidCount ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="少款" value={summary?.shortCount ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="多款" value={summary?.overCount ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="已结清" value={summary?.settledCount ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="毛利差异较大" value={summary?.largeDiffs ?? 0} />
            </Col>
            <Col xs={12} sm={8} md={6} lg={3}>
              <Statistic title="待处理异常" value={summary?.flaggedCount ?? 0} />
            </Col>
          </Row>
        )}
      </Card>
      <Card size="small">
        <div style={{ marginBottom: 16, overflowX: 'auto' }}>
          <Segmented
            value={status}
            onChange={(v) => setStatus(v as ReconStatusFilter)}
            options={FILTER_OPTIONS}
          />
        </div>
        {data?.truncated ? (
          <Alert type="info" showIcon style={{ marginBottom: 12 }} message="差异行数较多，仅展示前 500 行；完整数据请导出 CSV" />
        ) : null}
        <Table<OrderFinance>
          rowKey="orderId"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={rows}
          scroll={{ x: 'max-content' }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
          locale={{
            emptyText: <EmptyState title={`${reportRangeLabel(range)}内暂无对账数据`} />,
          }}
        />
      </Card>
    </TmPageContainer>
  );
}

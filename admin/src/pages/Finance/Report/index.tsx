import { DownloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Space, Table, Tag, Tooltip, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useSearchParams } from '@umijs/max';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import {
  downloadFinanceReportCsv,
  fetchFinanceReport,
  type FinanceReportDTO,
  type FinanceReportRow,
} from '@/services/finance';
import { RangeControls, normalizeReportRange, reportRangeLabel } from '../../Reports/components/RangeControls';

function baseText(v?: number): string {
  return v == null ? '未折算' : formatAmount(v);
}

/** 对账报表：按店铺 × 月份汇总回款率、费用构成与实算/估算毛利差异 */
export default function FinanceReport() {
  const [searchParams] = useSearchParams();
  const range = normalizeReportRange(searchParams);

  const [data, setData] = useState<FinanceReportDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchFinanceReport(range)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.days, range.start, range.end]);

  useEffect(() => {
    load();
  }, [load]);

  const exportCsv = useCallback(() => {
    setExporting(true);
    downloadFinanceReportCsv(range)
      .then(() => message.success('已导出 CSV'))
      .catch(() => message.error('导出失败，请稍后重试'))
      .finally(() => setExporting(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.days, range.start, range.end]);

  const base = data?.baseCurrency || 'CNY';
  const rows = data?.rows ?? [];

  const columns = useMemo<ColumnsType<FinanceReportRow>>(
    () => [
      { title: '月份', dataIndex: 'month', width: 90, fixed: 'left' },
      { title: '店铺', dataIndex: 'shopName', width: 160, fixed: 'left' },
      { title: '订单数', dataIndex: 'orderCount', width: 80, align: 'right' },
      {
        title: `应收（${base}）`,
        dataIndex: 'receivableBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `已回款（${base}）`,
        dataIndex: 'receivedBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: '回款率',
        dataIndex: 'returnRatePercent',
        width: 100,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{v == null ? '未折算' : `${formatAmount(v)}%`}</span>,
      },
      {
        title: '费用构成',
        dataIndex: 'feesByType',
        width: 220,
        render: (parts: FinanceReportRow['feesByType']) =>
          parts.length ? (
            <Space size={4} wrap>
              {parts.map((p) => (
                <Tag key={p.typeCode}>{`${p.typeLabel} ${formatAmount(p.base)}`}</Tag>
              ))}
            </Space>
          ) : (
            '—'
          ),
      },
      {
        title: `店铺月费（${base}）`,
        dataIndex: 'shopExpenseBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{v == null ? '—' : formatAmount(v)}</span>,
      },
      {
        title: `采购实付（${base}）`,
        dataIndex: 'actualCostBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `实算毛利（${base}）`,
        dataIndex: 'actualProfitBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `估算毛利（${base}）`,
        dataIndex: 'estimatedProfitBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: `毛利差异（${base}）`,
        dataIndex: 'profitDiffBase',
        width: 130,
        align: 'right',
        render: (v?: number) => <span style={tabularNumsStyle}>{baseText(v)}</span>,
      },
      {
        title: '对账异常',
        key: 'anomalies',
        width: 180,
        render: (_, r) => (
          <Space size={4} wrap>
            {r.unpaidCount > 0 ? <Tag>未回款 {r.unpaidCount}</Tag> : null}
            {r.shortCount > 0 ? <Tag color="red">少款 {r.shortCount}</Tag> : null}
            {r.overCount > 0 ? <Tag color="orange">多款 {r.overCount}</Tag> : null}
            {r.largeDiffCount > 0 ? <Tag color="red">差异较大 {r.largeDiffCount}</Tag> : null}
            {r.missingActualLines > 0 ? (
              <Tooltip title="未登记采购实付价的明细行数">
                <Tag color="orange">缺实付 {r.missingActualLines}</Tag>
              </Tooltip>
            ) : null}
            {r.unpaidCount + r.shortCount + r.overCount + r.largeDiffCount + r.missingActualLines === 0 ? '—' : null}
          </Space>
        ),
      },
    ],
    [base],
  );

  return (
    <TmPageContainer
      title="对账报表"
      subTitle="按店铺 × 月份汇总回款率、费用构成与毛利差异"
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
          message="对账报表加载失败"
          description={error}
          action={<Button size="small" onClick={load}>重试</Button>}
        />
      ) : null}
      <Card size="small">
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          实算毛利 = 回款（扣手续费）− 采购实付 − 订单费用 − 店铺月度费用；估算毛利沿用参考价口径。未配置汇率的币种不折算、不伪造。
        </Typography.Paragraph>
        <Table<FinanceReportRow>
          rowKey={(r) => `${r.shopId ?? 'none'}|${r.month}`}
          size="small"
          loading={loading}
          columns={columns}
          dataSource={rows}
          scroll={{ x: 'max-content' }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
          locale={{
            emptyText: <EmptyState description={`${reportRangeLabel(range)}内暂无对账数据`} />,
          }}
        />
      </Card>
    </TmPageContainer>
  );
}

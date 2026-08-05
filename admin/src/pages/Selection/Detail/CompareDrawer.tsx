import { EmptyState } from '@/components/ui';
import { Button, Drawer, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useEffect, useMemo, useState } from 'react';
import { extractApiErrorMessage } from '@/services/request';
import { downloadCSV } from '@/utils/csv';
import { fetchSelectionCompare, type CompareRow } from '@/services/selection';

function moneyText(v?: number, currency?: string): string {
  if (v === undefined || v === null) return '-';
  return `${v.toFixed(2)} ${currency || ''}`.trim();
}

function supplyText(row: CompareRow): string {
  if (!row.supply.ready) return '未匹配货源档案';
  return `已就绪${row.supply.supplierName ? `（${row.supply.supplierName}）` : ''}`;
}

function bannedText(row: CompareRow): string {
  const { forbiddenCount, warningCount, words } = row.banned;
  if (!forbiddenCount && !warningCount) return '无命中';
  const parts: string[] = [];
  if (forbiddenCount) parts.push(`禁用 ${forbiddenCount}`);
  if (warningCount) parts.push(`警告 ${warningCount}`);
  return `${parts.join(' / ')}${words?.length ? `：${words.join('、')}` : ''}`;
}

type MetricRow = {
  key: string;
  metric: string;
  render: (row: CompareRow) => React.ReactNode;
  csv: (row: CompareRow) => string | number;
};

const METRICS: MetricRow[] = [
  {
    key: 'marketPrice',
    metric: '海外在售价',
    render: (r) => moneyText(r.candidate.marketPrice, r.candidate.marketCurrency),
    csv: (r) => moneyText(r.candidate.marketPrice, r.candidate.marketCurrency),
  },
  {
    key: 'sourcePrice',
    metric: '1688 货源价',
    render: (r) =>
      r.bestMatch
        ? `${moneyText(r.bestMatch.minPrice, r.bestMatch.currency)} ~ ${moneyText(r.bestMatch.maxPrice, r.bestMatch.currency)}`
        : '未匹配',
    csv: (r) =>
      r.bestMatch
        ? `${moneyText(r.bestMatch.minPrice, r.bestMatch.currency)} ~ ${moneyText(r.bestMatch.maxPrice, r.bestMatch.currency)}`
        : '未匹配',
  },
  {
    key: 'estProfit',
    metric: '预估毛利',
    render: (r) => {
      const p = r.evaluation?.estProfit;
      if (p === undefined || p === null) return '-';
      return (
        <Typography.Text type={p < 0 ? 'danger' : 'success'}>
          {moneyText(p, r.candidate.marketCurrency)}
          {r.evaluation?.estMarginPercent !== undefined ? ` / ${r.evaluation.estMarginPercent.toFixed(1)}%` : ''}
        </Typography.Text>
      );
    },
    csv: (r) => {
      const p = r.evaluation?.estProfit;
      if (p === undefined || p === null) return '-';
      return `${moneyText(p, r.candidate.marketCurrency)}${
        r.evaluation?.estMarginPercent !== undefined ? ` / ${r.evaluation.estMarginPercent.toFixed(1)}%` : ''
      }`;
    },
  },
  {
    key: 'aiScore',
    metric: 'AI 评分',
    render: (r) => {
      const s = r.evaluation?.aiScore;
      if (s === undefined || s === null) return '-';
      return <Tag color={s >= 70 ? 'green' : s >= 50 ? 'orange' : 'red'}>{s.toFixed(0)} 分</Tag>;
    },
    csv: (r) => (r.evaluation?.aiScore !== undefined ? `${r.evaluation.aiScore.toFixed(0)} 分` : '-'),
  },
  {
    key: 'supply',
    metric: '供应链就绪度',
    render: (r) => (r.supply.ready ? <Tag color="green">{supplyText(r)}</Tag> : <Tag>未匹配货源档案</Tag>),
    csv: supplyText,
  },
  {
    key: 'banned',
    metric: '违禁词风险',
    render: (r) => {
      const { forbiddenCount, warningCount } = r.banned;
      if (!forbiddenCount && !warningCount) return <Tag color="green">无命中</Tag>;
      return (
        <Typography.Text type={forbiddenCount ? 'danger' : 'warning'}>{bannedText(r)}</Typography.Text>
      );
    },
    csv: bannedText,
  },
];

export type CompareDrawerProps = {
  candidateIds: string[];
  open: boolean;
  onClose: () => void;
};

/** 选品对比：多候选并排对比 + CSV 导出。 */
export default function CompareDrawer({ candidateIds, open, onClose }: CompareDrawerProps) {
  const [rows, setRows] = useState<CompareRow[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open || candidateIds.length < 2) return;
    let cancelled = false;
    setLoading(true);
    setRows([]);
    void (async () => {
      try {
        const res = await fetchSelectionCompare(candidateIds);
        if (!cancelled) setRows(res || []);
      } catch (e) {
        if (!cancelled) message.error(extractApiErrorMessage(e, '对比加载失败'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, candidateIds]);

  const columns = useMemo<ColumnsType<MetricRow>>(() => {
    const cols: ColumnsType<MetricRow> = [
      {
        title: '指标',
        dataIndex: 'metric',
        fixed: 'left',
        width: 120,
      },
    ];
    rows.forEach((row, i) => {
      cols.push({
        title: (
          <Typography.Text style={{ maxWidth: 180 }} ellipsis={{ tooltip: row.candidate.title }}>
            {row.candidate.title}
          </Typography.Text>
        ),
        key: row.candidate.id || String(i),
        width: 200,
        render: (_, metric) => metric.render(row),
      });
    });
    return cols;
  }, [rows]);

  const exportCSV = () => {
    const header = ['指标', ...rows.map((r) => r.candidate.title)];
    const body = METRICS.map((m) => [m.metric, ...rows.map((r) => m.csv(r))]);
    downloadCSV(`选品对比-${new Date().toISOString().slice(0, 10)}.csv`, [header, ...body]);
  };

  return (
    <Drawer
      title={`选品对比（${candidateIds.length} 个候选）`}
      open={open}
      onClose={onClose}
      width="min(960px, 100vw)"
      destroyOnClose
      extra={
        <Button type="primary" disabled={loading || rows.length === 0} onClick={exportCSV}>
          导出 CSV
        </Button>
      }
    >
      {loading ? (
        <Skeleton active paragraph={{ rows: 8 }} />
      ) : rows.length === 0 ? (
        <EmptyState title="暂无对比数据" description="请关闭后重新选择候选发起对比。" compact />
      ) : (
        <Table<MetricRow>
          rowKey="key"
          columns={columns}
          dataSource={METRICS}
          pagination={false}
          scroll={{ x: 120 + rows.length * 200 }}
          size="small"
        />
      )}
    </Drawer>
  );
}

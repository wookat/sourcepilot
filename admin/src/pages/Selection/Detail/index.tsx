import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ProColumns } from '@ant-design/pro-components';
import { Link, useParams } from '@umijs/renderer-react';
import { Button, Descriptions, Image, Popconfirm, Space, Tag, Tooltip, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { formatDateTime } from '@/utils/formatTime';
import {
  decideSelectionCandidate,
  fetchSelectionCandidates,
  fetchSelectionTask,
  selectionCandidateToDraft,
  type SelectionCandidateItem,
  type SelectionTaskRow,
} from '@/services/selection';

const STATUS_COLOR: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  success: 'success',
  partial: 'warning',
  failed: 'error',
};

const DECISION_COLOR: Record<string, string> = {
  pending: 'default',
  approved: 'success',
  rejected: 'error',
};

function money(v?: number, currency?: string) {
  if (v === undefined || v === null) return '-';
  return `${v.toFixed(2)} ${currency || ''}`.trim();
}

type AIReasons = {
  summary?: string;
  risks?: string[];
  sellingPoints?: string[];
};

function parseReasons(raw: unknown): AIReasons {
  if (!raw) return {};
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as AIReasons;
    } catch {
      return { summary: raw };
    }
  }
  return raw as AIReasons;
}

export default function SelectionTaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [task, setTask] = useState<SelectionTaskRow | null>(null);
  const [rows, setRows] = useState<SelectionCandidateItem[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [t, list] = await Promise.all([fetchSelectionTask(id), fetchSelectionCandidates(id)]);
      setTask(t);
      setRows(list || []);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = async (candidateId: string, decision: 'approved' | 'rejected') => {
    try {
      await decideSelectionCandidate(candidateId, decision);
      message.success(decision === 'approved' ? '已通过' : '已拒绝');
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '操作失败');
    }
  };

  const toDraft = async (candidateId: string) => {
    try {
      const draft = await selectionCandidateToDraft(candidateId);
      message.success(`已转商品草稿：${draft.title || draft.id}`);
      void load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '转草稿失败');
    }
  };

  const columns: ProColumns<SelectionCandidateItem>[] = [
    {
      title: '#',
      width: 48,
      render: (_, __, index) => index + 1,
    },
    {
      title: '商品',
      render: (_, row) => (
        <Space>
          {row.candidate.imageUrl ? (
            <Image src={row.candidate.imageUrl} width={44} height={44} preview={false} />
          ) : null}
          <Typography.Text style={{ maxWidth: 260 }} ellipsis={{ tooltip: row.candidate.title }}>
            {row.candidate.title}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '海外在售价',
      width: 130,
      render: (_, row) =>
        money(row.candidate.marketPrice, row.candidate.marketCurrency) +
        (row.candidate.marketSales30d ? ` / 30d销量 ${row.candidate.marketSales30d}` : ''),
    },
    {
      title: '1688 同款',
      width: 220,
      render: (_, row) => {
        const m = row.bestMatch;
        if (!m) return <Tag>未匹配</Tag>;
        return (
          <Space direction="vertical" size={0}>
            <span>
              {money(m.minPrice, m.currency)} ~ {money(m.maxPrice, m.currency)}
              {m.moq ? ` (MOQ ${m.moq})` : ''}
            </span>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {m.supplierName || '-'}
              {m.similarity !== undefined ? ` · 相似度 ${(m.similarity * 100).toFixed(0)}%` : ''}
              {m.sourceUrl ? (
                <>
                  {' · '}
                  <a href={m.sourceUrl} target="_blank" rel="noreferrer">
                    链接
                  </a>
                </>
              ) : null}
            </Typography.Text>
          </Space>
        );
      },
    },
    {
      title: '预期利润 / 利润率',
      width: 150,
      render: (_, row) => {
        const ev = row.evaluation;
        if (!ev || ev.estProfit === undefined) return '-';
        const negative = (ev.estProfit ?? 0) < 0;
        return (
          <Typography.Text type={negative ? 'danger' : 'success'}>
            {money(ev.estProfit, row.candidate.marketCurrency)} /{' '}
            {ev.estMarginPercent !== undefined ? `${ev.estMarginPercent.toFixed(1)}%` : '-'}
          </Typography.Text>
        );
      },
    },
    {
      title: 'AI 评分',
      width: 170,
      sorter: (a, b) => (a.evaluation?.aiScore ?? -1) - (b.evaluation?.aiScore ?? -1),
      render: (_, row) => {
        const ev = row.evaluation;
        if (!ev || ev.aiScore === undefined) return '-';
        const reasons = parseReasons(ev.aiReasons);
        return (
          <Tooltip
            title={
              <div>
                {reasons.summary && <div>{reasons.summary}</div>}
                {reasons.sellingPoints?.length ? <div>卖点：{reasons.sellingPoints.join('；')}</div> : null}
                {reasons.risks?.length ? <div>风险：{reasons.risks.join('；')}</div> : null}
                {ev.aiModel ? <div>模型：{ev.aiModel}</div> : null}
              </div>
            }
          >
            <Tag color={ev.aiScore >= 70 ? 'green' : ev.aiScore >= 50 ? 'orange' : 'red'}>
              {ev.aiScore.toFixed(0)} 分
            </Tag>
          </Tooltip>
        );
      },
    },
    {
      title: '状态',
      width: 90,
      render: (_, row) =>
        row.candidate.status === 'failed' ? (
          <Tooltip title={row.candidate.errorMessage}>
            <Tag color="error">failed</Tag>
          </Tooltip>
        ) : (
          <Tag>{row.candidate.status}</Tag>
        ),
    },
    {
      title: '审核',
      width: 90,
      render: (_, row) => {
        const d = row.evaluation?.decision || 'pending';
        return <Tag color={DECISION_COLOR[d]}>{d}</Tag>;
      },
    },
    {
      title: '操作',
      width: 220,
      render: (_, row) => {
        const cid = row.candidate.id;
        const ev = row.evaluation;
        const scored = row.candidate.status === 'scored';
        if (ev?.draftProductId) {
          return <Link to={`/product/drafts/${ev.draftProductId}`}>查看草稿</Link>;
        }
        return (
          <Space>
            {scored && ev?.decision !== 'approved' && (
              <a onClick={() => decide(cid, 'approved')}>通过</a>
            )}
            {scored && ev?.decision !== 'rejected' && (
              <a onClick={() => decide(cid, 'rejected')}>拒绝</a>
            )}
            {ev?.decision === 'approved' && (
              <Popconfirm title="转为商品草稿并进入刊登链路？" onConfirm={() => toDraft(cid)}>
                <Button size="small" type="primary">
                  转草稿
                </Button>
              </Popconfirm>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <TmPageContainer
      title={task?.name || 'AI 选品清单'}
      subTitle="按 AI 评分排序的可上架清单"
      onBack={() => window.history.back()}
      extra={[
        <Button key="refresh" onClick={() => void load()}>
          刷新
        </Button>,
      ]}
    >
      {task && (
        <Descriptions size="small" column={4} style={{ marginBottom: 16 }}>
          <Descriptions.Item label="状态">
            <Tag color={STATUS_COLOR[task.status]}>{task.status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="目标平台">
            {task.targetPlatform}
            {task.targetCountry ? ` / ${task.targetCountry}` : ''}
          </Descriptions.Item>
          <Descriptions.Item label="候选/打分/失败">
            {task.candidateCount} / {task.scoredCount} / {task.failedCount}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">{formatDateTime(task.createdAt)}</Descriptions.Item>
        </Descriptions>
      )}
      <ProTable<SelectionCandidateItem>
        rowKey={(row) => row.candidate.id}
        columns={columns}
        dataSource={rows}
        loading={loading}
        search={false}
        pagination={{ pageSize: 20 }}
        toolBarRender={false}
      />
    </TmPageContainer>
  );
}

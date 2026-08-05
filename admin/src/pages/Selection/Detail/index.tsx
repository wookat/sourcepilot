import { PlatformTag, StatusTag, TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ProColumns } from '@ant-design/pro-components';
import { Link, useParams } from '@umijs/renderer-react';
import { Alert, Button, Descriptions, Image, Popconfirm, Space, Tag, Tooltip, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { IMAGE_FALLBACK } from '@/constants/imageFallback';
import { formatDateTime } from '@/utils/formatTime';
import { usePermission } from '@/hooks/usePermission';
import {
  decideSelectionCandidate,
  fetchSelectionCandidates,
  fetchSelectionTask,
  selectionCandidateToDraft,
  type SelectionCandidateItem,
  type SelectionTaskRow,
} from '@/services/selection';
import { extractApiErrorMessage } from '@/services/request';
import CompareDrawer from './CompareDrawer';
import InsightsDrawer from './InsightsDrawer';

const POLL_MS = 4000;

function money(v?: number, currency?: string) {
  if (v === undefined || v === null) return '-';
  return `${v.toFixed(2)} ${currency || ''}`.trim();
}

type AIReasons = {
  summary?: string;
  risks?: string[];
  sellingPoints?: string[];
  fallback?: boolean;
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
  const { readonly } = usePermission();
  const [task, setTask] = useState<SelectionTaskRow | null>(null);
  const [rows, setRows] = useState<SelectionCandidateItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [compareOpen, setCompareOpen] = useState(false);
  const [insightsTarget, setInsightsTarget] = useState<SelectionCandidateItem | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [t, list] = await Promise.all([fetchSelectionTask(id), fetchSelectionCandidates(id)]);
      setTask(t);
      setRows(list || []);
    } catch (e) {
      message.error(extractApiErrorMessage(e, '加载失败'));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  // 处理中任务自动轮询；静默刷新避免表格 loading 闪烁
  const active = task?.status === 'pending' || task?.status === 'running';
  useEffect(() => {
    if (!id || !active) return;
    const timer = window.setInterval(async () => {
      if (document.visibilityState === 'hidden') return;
      try {
        const [t, list] = await Promise.all([fetchSelectionTask(id), fetchSelectionCandidates(id)]);
        setTask(t);
        setRows(list || []);
      } catch {
        // 轮询失败保留上一次成功快照，不打断用户
      }
    }, POLL_MS);
    return () => window.clearInterval(timer);
  }, [id, active]);

  const decide = async (candidateId: string, decision: 'approved' | 'rejected') => {
    try {
      await decideSelectionCandidate(candidateId, decision);
      message.success(decision === 'approved' ? '已通过' : '已拒绝');
      void load();
    } catch (e) {
      message.error(extractApiErrorMessage(e, '操作失败'));
    }
  };

  const toDraft = async (candidateId: string) => {
    try {
      const draft = await selectionCandidateToDraft(candidateId);
      message.success(`已转商品草稿：${draft.title || draft.id}`);
      void load();
    } catch (e) {
      message.error(extractApiErrorMessage(e, '转草稿失败'));
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
            <Image src={row.candidate.imageUrl} fallback={IMAGE_FALLBACK} width={44} height={44} preview={false} />
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
                {reasons.fallback ? <div>AI 未配置或调用失败，已使用规则兜底评分</div> : null}
              </div>
            }
          >
            <Space size={4}>
              <Tag color={ev.aiScore >= 70 ? 'green' : ev.aiScore >= 50 ? 'orange' : 'red'}>
                {ev.aiScore.toFixed(0)} 分
              </Tag>
              {reasons.fallback ? <Tag color="warning">规则兜底</Tag> : null}
            </Space>
          </Tooltip>
        );
      },
    },
    {
      title: '状态',
      width: 160,
      render: (_, row) =>
        row.candidate.status === 'failed' ? (
          <Space direction="vertical" size={0}>
            <StatusTag status="failed" />
            <Typography.Text
              type="danger"
              style={{ fontSize: 12, maxWidth: 140 }}
              ellipsis={{ tooltip: `失败原因：${row.candidate.errorMessage || '未返回具体原因'}` }}
            >
              {row.candidate.errorMessage || '未返回具体原因'}
            </Typography.Text>
          </Space>
        ) : (
          <StatusTag status={row.candidate.status} />
        ),
    },
    {
      title: '审核',
      width: 90,
      render: (_, row) => {
        return <StatusTag status={row.evaluation?.decision || 'pending'} />;
      },
    },
    {
      title: '操作',
      width: 260,
      render: (_, row) => {
        const cid = row.candidate.id;
        const ev = row.evaluation;
        const scored = row.candidate.status === 'scored';
        const insightsLink = (
          <a aria-label={`查看数据面板：${row.candidate.title}`} onClick={() => setInsightsTarget(row)}>
            数据面板
          </a>
        );
        if (ev?.draftProductId) {
          return (
            <Space>
              {insightsLink}
              <Link to={`/product/drafts/${ev.draftProductId}`}>查看草稿</Link>
            </Space>
          );
        }
        if (readonly) {
          return (
            <Space>
              {insightsLink}
              <Typography.Text type="secondary">只读</Typography.Text>
            </Space>
          );
        }
        return (
          <Space>
            {insightsLink}
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
        <Button
          key="compare"
          type="primary"
          disabled={selectedIds.length < 2}
          onClick={() => setCompareOpen(true)}
        >
          对比所选（{selectedIds.length}）
        </Button>,
        <Button key="refresh" onClick={() => void load()}>
          刷新
        </Button>,
      ]}
    >
      {task && (task.status === 'failed' || task.status === 'partial') && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message={task.status === 'failed' ? '任务失败' : '任务部分失败'}
          description={
            task.errorMessage
              ? `失败原因：${task.errorMessage}`
              : '未返回任务级失败原因，请查看下方失败候选的具体原因，或重试任务。'
          }
        />
      )}
      {active && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="任务处理中，页面将自动刷新"
        />
      )}
      {rows.some((row) => parseReasons(row.evaluation?.aiReasons).fallback) && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="本任务存在规则兜底评分"
          description="AI 服务商未配置或调用失败，部分候选的评分由利润率规则生成。可前往 系统设置 → AI 完成配置后重试任务，以获得 AI 评分。"
        />
      )}
      {task && (
        <Descriptions size="small" column={{ xs: 1, sm: 2, md: 4 }} style={{ marginBottom: 16 }}>
          <Descriptions.Item label="状态">
            <StatusTag status={task.status} />
          </Descriptions.Item>
          <Descriptions.Item label="目标平台">
            <PlatformTag platform={task.targetPlatform} />
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
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys) => setSelectedIds(keys.map(String).slice(0, 5)),
          getCheckboxProps: (row) => ({
            'aria-label': `选择候选：${row.candidate.title}`,
          }),
        }}
        tableAlertRender={false}
      />
      {selectedIds.length === 1 ? (
        <Typography.Text type="secondary">再勾选至少 1 个候选即可发起对比（最多 5 个）。</Typography.Text>
      ) : null}
      <InsightsDrawer
        open={!!insightsTarget}
        candidateId={insightsTarget?.candidate.id}
        title={insightsTarget?.candidate.title}
        onClose={() => setInsightsTarget(null)}
      />
      <CompareDrawer
        open={compareOpen}
        candidateIds={selectedIds}
        onClose={() => setCompareOpen(false)}
      />
    </TmPageContainer>
  );
}

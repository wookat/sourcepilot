import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { Link } from '@umijs/renderer-react';
import { Alert, Button, Space, Tag, Tooltip, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { formatDateTime } from '@/utils/formatTime';
import { usePermission } from '@/hooks/usePermission';
import {
  createSelectionTask,
  fetchSelectionTasks,
  retrySelectionTask,
  type SelectionTaskRow,
} from '@/services/selection';
import { extractApiErrorMessage } from '@/services/request';
import { fetchIntegrationsOverview } from '@/services/settings';

const STATUS_COLOR: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  success: 'success',
  partial: 'warning',
  failed: 'error',
};

const STATUS_FILTER_OPTIONS: Record<string, { text: string }> = {
  pending: { text: 'pending（待处理）' },
  running: { text: 'running（处理中）' },
  success: { text: 'success（成功）' },
  partial: { text: 'partial（部分失败）' },
  failed: { text: 'failed（失败）' },
};

const POLL_MS = 4000;

function hasActiveTask(rows: SelectionTaskRow[]) {
  return rows.some((row) => row.status === 'pending' || row.status === 'running');
}

const PLATFORM_OPTIONS = ['tiktok', 'shopee', 'lazada', 'amazon', 'douyin'].map((v) => ({
  label: v,
  value: v,
}));

type CreateFormValues = {
  name?: string;
  targetPlatform: string;
  targetCountry?: string;
  keywords?: string;
  items?: string;
  exchangeRate?: number;
  commissionPercent?: number;
  returnRatePercent?: number;
  minMarginPercent?: number;
};

function parseItems(text?: string) {
  if (!text?.trim()) return [];
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [title, price, currency, url] = line.split(',').map((s) => s.trim());
      const marketPrice = price ? Number(price) : undefined;
      return {
        title,
        marketPrice: Number.isFinite(marketPrice) ? marketPrice : undefined,
        marketCurrency: currency || undefined,
        sourceUrl: url || undefined,
      };
    })
    .filter((it) => it.title);
}

export default function SelectionTasksPage() {
  const actionRef = useRef<ActionType>();
  const { readonly } = usePermission();
  const [aiConfigured, setAiConfigured] = useState<boolean | undefined>(undefined);
  const [polling, setPolling] = useState<number | undefined>(undefined);

  useEffect(() => {
    fetchIntegrationsOverview()
      .then((data) => setAiConfigured(data.ai?.configured))
      .catch(() => setAiConfigured(undefined));
  }, []);

  useEffect(() => {
    const sync = () => {
      if (document.visibilityState === 'hidden') setPolling(undefined);
    };
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, []);

  const columns: ProColumns<SelectionTaskRow>[] = [
    {
      title: '任务',
      dataIndex: 'name',
      hideInSearch: true,
      render: (_, row) => (
        <Link to={`/selection/tasks/${row.id}`}>{row.name || row.id.slice(0, 8)}</Link>
      ),
    },
    { title: '目标平台', dataIndex: 'targetPlatform', width: 100, hideInSearch: true },
    { title: '国家', dataIndex: 'targetCountry', width: 80, hideInSearch: true },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      valueType: 'select',
      valueEnum: STATUS_FILTER_OPTIONS,
      fieldProps: { placeholder: '全部状态', allowClear: true },
      render: (_, row) => {
        const tag = <Tag color={STATUS_COLOR[row.status] || 'default'}>{row.status}</Tag>;
        if ((row.status === 'failed' || row.status === 'partial') && row.errorMessage) {
          return <Tooltip title={`失败原因：${row.errorMessage}`}>{tag}</Tooltip>;
        }
        return tag;
      },
    },
    {
      title: '候选/打分/失败',
      width: 140,
      hideInSearch: true,
      render: (_, row) => `${row.candidateCount} / ${row.scoredCount} / ${row.failedCount}`,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 170,
      hideInSearch: true,
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '操作',
      width: 160,
      hideInSearch: true,
      render: (_, row) => (
        <Space>
          <Link to={`/selection/tasks/${row.id}`}>查看清单</Link>
          {!readonly && (row.status === 'failed' || row.status === 'partial') && (
            <a
              onClick={async () => {
                try {
                  await retrySelectionTask(row.id);
                  message.success('已重新入队');
                  actionRef.current?.reload();
                } catch (e) {
                  message.error(extractApiErrorMessage(e, '重试失败'));
                }
              }}
            >
              重试
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <TmPageContainer title="AI 选品任务" subTitle="海外在售价 → 1688 同款 → 利润模型 → LLM 打分">
      <ProTable<SelectionTaskRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        polling={polling}
        request={async (params) => {
          const res = await fetchSelectionTasks({
            page: params.current,
            pageSize: params.pageSize,
            status: (params.status as string | undefined) || undefined,
          });
          const items = res.items || [];
          setPolling(
            hasActiveTask(items) && document.visibilityState !== 'hidden' ? POLL_MS : undefined,
          );
          return { data: items, total: res.total, success: true };
        }}
        toolBarRender={() => (readonly ? [] : [
          <ModalForm<CreateFormValues>
            key="create"
            title="新建选品任务"
            width={560}
            trigger={<Button type="primary">新建选品任务</Button>}
            onFinish={async (values) => {
              const items = parseItems(values.items);
              const keywords = (values.keywords || '')
                .split('\n')
                .map((s) => s.trim())
                .filter(Boolean);
              if (items.length === 0 && keywords.length === 0) {
                message.error('请至少填写一条候选商品或关键词');
                return false;
              }
              try {
                await createSelectionTask({
                  name: values.name,
                  targetPlatform: values.targetPlatform,
                  targetCountry: values.targetCountry,
                  items,
                  keywords,
                  params: {
                    exchangeRate: values.exchangeRate,
                    commissionPercent: values.commissionPercent,
                    returnRatePercent: values.returnRatePercent,
                    minMarginPercent: values.minMarginPercent,
                  },
                });
                message.success('任务已创建并入队');
                actionRef.current?.reload();
                return true;
              } catch (e) {
                message.error(extractApiErrorMessage(e, '创建失败'));
                return false;
              }
            }}
          >
            {aiConfigured === false && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 16 }}
                message="AI 服务商未配置，任务将使用规则兜底评分"
                description="可前往 系统设置 → AI 完成配置后再创建任务，以获得 AI 评分与建议。"
              />
            )}
            <ProFormText name="name" label="任务名称" placeholder="例如：7月 TikTok 美区选品" />
            <ProFormSelect
              name="targetPlatform"
              label="目标平台"
              options={PLATFORM_OPTIONS}
              rules={[{ required: true, message: '请选择目标平台' }]}
            />
            <ProFormText name="targetCountry" label="目标国家" placeholder="US / MY / TH ..." />
            <ProFormTextArea
              name="items"
              label="人工导入候选（每行：标题,在售价,币种,1688链接 后三项可留空）"
              placeholder={'LED Dog Collar,12.99,USD,\n宠物饮水器,,,'}
              fieldProps={{ rows: 4 }}
            />
            <ProFormTextArea
              name="keywords"
              label="关键词候选（每行一个，通过采集/行情服务获取价格）"
              placeholder={'pet water fountain\nmagnetic phone holder'}
              fieldProps={{ rows: 3 }}
            />
            <ProFormDigit name="exchangeRate" label="汇率 CNY/目标币（留空用系统默认）" min={0.0001} />
            <ProFormDigit name="commissionPercent" label="平台佣金 %（留空用系统默认）" min={0} max={100} />
            <ProFormDigit name="returnRatePercent" label="退货率 %（留空用系统默认）" min={0} max={100} />
            <ProFormDigit name="minMarginPercent" label="最低利润率 %（低于此不推荐）" min={0} max={100} />
          </ModalForm>,
        ])}
      />
    </TmPageContainer>
  );
}

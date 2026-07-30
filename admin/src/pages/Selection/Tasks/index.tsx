import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { Link } from '@umijs/renderer-react';
import { Button, Space, Tag, message } from 'antd';
import { useRef } from 'react';
import { formatDateTime } from '@/utils/formatTime';
import {
  createSelectionTask,
  fetchSelectionTasks,
  retrySelectionTask,
  type SelectionTaskRow,
} from '@/services/selection';

const STATUS_COLOR: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  success: 'success',
  partial: 'warning',
  failed: 'error',
};

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

  const columns: ProColumns<SelectionTaskRow>[] = [
    {
      title: '任务',
      dataIndex: 'name',
      render: (_, row) => (
        <Link to={`/selection/tasks/${row.id}`}>{row.name || row.id.slice(0, 8)}</Link>
      ),
    },
    { title: '目标平台', dataIndex: 'targetPlatform', width: 100 },
    { title: '国家', dataIndex: 'targetCountry', width: 80 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => <Tag color={STATUS_COLOR[row.status] || 'default'}>{row.status}</Tag>,
    },
    {
      title: '候选/打分/失败',
      width: 140,
      render: (_, row) => `${row.candidateCount} / ${row.scoredCount} / ${row.failedCount}`,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 170,
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '操作',
      width: 160,
      render: (_, row) => (
        <Space>
          <Link to={`/selection/tasks/${row.id}`}>查看清单</Link>
          {(row.status === 'failed' || row.status === 'partial') && (
            <a
              onClick={async () => {
                try {
                  await retrySelectionTask(row.id);
                  message.success('已重新入队');
                  actionRef.current?.reload();
                } catch (e) {
                  message.error(e instanceof Error ? e.message : '重试失败');
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
        search={false}
        request={async (params) => {
          const res = await fetchSelectionTasks({
            page: params.current,
            pageSize: params.pageSize,
          });
          return { data: res.items, total: res.total, success: true };
        }}
        toolBarRender={() => [
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
                message.error(e instanceof Error ? e.message : '创建失败');
                return false;
              }
            }}
          >
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
        ]}
      />
    </TmPageContainer>
  );
}

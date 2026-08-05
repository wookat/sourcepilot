import { TmPageContainer } from '@/components/ui';
import {
  AUTOMATION_ACTION_LABELS,
  AUTOMATION_EVENT_LABELS,
  AUTOMATION_LOG_STATUS_COLORS,
  AUTOMATION_LOG_STATUS_LABELS,
  listOrderAutomationLogs,
  retryOrderAutomationLog,
  type AutomationAction,
  type AutomationLogStatus,
  type AutomationTriggerEvent,
  type OrderAutomationLogRow,
} from '@/services/orderAutomation';
import { isReadonly } from '@/utils/permission';
import { Link, useModel } from '@umijs/max';
import { Alert, Button, Card, Input, Select, Space, Table, Tag, Tooltip, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

const STATUS_OPTIONS = (
  Object.keys(AUTOMATION_LOG_STATUS_LABELS) as AutomationLogStatus[]
).map((v) => ({ value: v, label: AUTOMATION_LOG_STATUS_LABELS[v] }));

const EVENT_OPTIONS = (
  Object.keys(AUTOMATION_EVENT_LABELS) as AutomationTriggerEvent[]
).map((v) => ({ value: v, label: AUTOMATION_EVENT_LABELS[v] }));

export default function OrderAutomationLogsPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<OrderAutomationLogRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string | undefined>();
  const [event, setEvent] = useState<string | undefined>();
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [retryingId, setRetryingId] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const res = await listOrderAutomationLogs({
        page,
        pageSize,
        status,
        triggerEvent: event,
        keyword: keyword.trim() || undefined,
      });
      setRows(res.items);
      setTotal(res.total);
    } catch (e) {
      setLoadError((e as Error).message || '加载执行日志失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, status, event, keyword]);

  useEffect(() => {
    void load();
  }, [load]);

  const retry = async (row: OrderAutomationLogRow) => {
    setRetryingId(row.id);
    try {
      const updated = await retryOrderAutomationLog(row.id);
      if (updated.status === 'success') {
        message.success(`「${row.ruleName}」重试成功`);
      } else {
        message.warning(`重试完成，状态：${AUTOMATION_LOG_STATUS_LABELS[updated.status]}`);
      }
      await load();
    } catch (e) {
      message.error((e as Error).message || '重试失败');
    } finally {
      setRetryingId('');
    }
  };

  return (
    <TmPageContainer
      title="自动化执行日志"
      subTitle="自动化订单规则的执行留痕：成功 / 失败（可重试）/ 跳过原因；订单时间线同步标注「自动规则」"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            allowClear
            placeholder="状态"
            style={{ width: 140 }}
            options={STATUS_OPTIONS}
            value={status}
            onChange={(v) => {
              setPage(1);
              setStatus(v);
            }}
          />
          <Select
            allowClear
            placeholder="触发时机"
            style={{ width: 200 }}
            options={EVENT_OPTIONS}
            value={event}
            onChange={(v) => {
              setPage(1);
              setEvent(v);
            }}
          />
          <Input.Search
            allowClear
            placeholder="搜索订单号 / 规则名"
            style={{ width: 240 }}
            onSearch={(v) => {
              setPage(1);
              setKeyword(v);
            }}
          />
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载执行日志失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<OrderAutomationLogRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          scroll={{ x: 1400 }}
          locale={{ emptyText: '暂无执行日志' }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
          columns={[
            {
              title: '时间',
              dataIndex: 'createdAt',
              width: 170,
              render: (v: string) => (v ? new Date(v).toLocaleString('zh-CN') : '-'),
            },
            { title: '规则', dataIndex: 'ruleName', width: 200 },
            {
              title: '订单号',
              dataIndex: 'orderNo',
              width: 180,
              render: (v: string, row) => <Link to={`/orders/${row.orderId}`}>{v}</Link>,
            },
            {
              title: '触发时机',
              dataIndex: 'triggerEvent',
              width: 160,
              render: (v: AutomationTriggerEvent) => AUTOMATION_EVENT_LABELS[v] || v,
            },
            {
              title: '自动动作',
              dataIndex: 'action',
              width: 150,
              render: (v: AutomationAction) => AUTOMATION_ACTION_LABELS[v] || v,
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 90,
              render: (v: AutomationLogStatus) => (
                <Tag color={AUTOMATION_LOG_STATUS_COLORS[v]}>
                  {AUTOMATION_LOG_STATUS_LABELS[v] || v}
                </Tag>
              ),
            },
            {
              title: '结果 / 原因',
              dataIndex: 'reason',
              width: 260,
              ellipsis: { showTitle: false },
              render: (v: string) =>
                v ? (
                  <Tooltip title={v} placement="topLeft">
                    <span>{v}</span>
                  </Tooltip>
                ) : (
                  '-'
                ),
            },
            { title: '尝试次数', dataIndex: 'attempts', width: 90 },
            {
              title: '操作',
              width: 100,
              render: (_, row) =>
                row.status === 'failed' ? (
                  <Button
                    size="small"
                    type="link"
                    disabled={readonly}
                    loading={retryingId === row.id}
                    onClick={() => void retry(row)}
                  >
                    重试
                  </Button>
                ) : null,
            },
          ]}
        />
      </Card>
    </TmPageContainer>
  );
}

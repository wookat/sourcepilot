import { EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history, useModel } from '@umijs/max';
import { Button, Card, Col, Form, Input, Modal, Row, Select, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { DateTimeText, ErrorAlert, TmPageContainer, TmProTable } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  OPERATION_PLATFORM_LABELS,
  OPERATION_TASK_STATUS_LABELS,
  OPERATION_TASK_TYPE_LABELS,
} from '@/constants/operationTasks';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  approveTask,
  createOperationIdempotencyKey,
  extractOperationTaskAPIError,
  getTask,
  listTasks,
  rejectTask,
  type OperationTaskAPIError,
  type OperationTaskSummary,
} from '@/services/operationTasks';
import { canReviewOperationTasks } from '@/utils/permission';
import {
  NonProductionBoundary,
  OperationAttemptStatusTag,
  OperationPriorityTag,
  OperationTaskStatusTag,
  copyableText,
  operationErrorMessage,
  platformLabel,
  taskTypeLabel,
} from './components/OperationTaskShared';

const { Text, Paragraph } = Typography;

const QUERY_KEYS = ['status', 'platform', 'taskType', 'cursor'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

function optionsFromLabels(labels: Record<string, string | { zhCN: string }>) {
  return Object.entries(labels).map(([value, label]) => ({
    value,
    label: typeof label === 'string' ? label : label.zhCN,
  }));
}

type BatchReviewKind = 'approve' | 'reject';

export default function OperationTasksPage() {
  const emptyLocale = useListEmptyLocale('operationTasks');
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const reviewable = canReviewOperationTasks(initialState?.currentUser?.role);
  const { state: urlState, setState: setUrlState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [items, setItems] = useState<OperationTaskSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<OperationTaskAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [batchModal, setBatchModal] = useState<BatchReviewKind | null>(null);
  const [batchLoading, setBatchLoading] = useState(false);
  const [batchForm] = Form.useForm();
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({
      status: urlState.status,
      platform: urlState.platform,
      taskType: urlState.taskType,
    });
  }, [form, urlState.platform, urlState.status, urlState.taskType]);

  const load = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const page = await listTasks({
        status: urlState.status,
        platform: urlState.platform,
        taskType: urlState.taskType,
        cursor: urlState.cursor,
        limit: LIMIT,
      });
      if (requestSeq.current !== seq) return;
      const nextItems = page.items || [];
      setItems(nextItems);
      setSelectedRowKeys((prev) => prev.filter((id) => nextItems.some((item) => item.id === id)));
      setNextCursor(page.nextCursor);
      setHasMore(page.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractOperationTaskAPIError(e));
      setItems([]);
      setNextCursor(undefined);
      setHasMore(false);
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [urlState.cursor, urlState.platform, urlState.status, urlState.taskType]);

  useEffect(() => {
    void load();
  }, [load]);

  const updateFilters = (values: QueryState) => {
    setCursorStack([]);
    setUrlState({
      status: values.status,
      platform: values.platform,
      taskType: values.taskType,
      cursor: undefined,
    }, { replace: true });
  };

  const clearFilters = () => {
    setCursorStack([]);
    form.resetFields();
    clearState(QUERY_KEYS, { replace: true });
  };

  const goNext = () => {
    if (!nextCursor) return;
    setCursorStack((prev) => [...prev, urlState.cursor || '']);
    setUrlState({ cursor: nextCursor }, { replace: true });
  };

  const goPrev = () => {
    setCursorStack((prev) => {
      const next = [...prev];
      const prevCursor = next.pop();
      setUrlState({ cursor: prevCursor || undefined }, { replace: true });
      return next;
    });
  };

  const selectedReviewableIds = useMemo(
    () =>
      items
        .filter((item) => selectedRowKeys.includes(item.id) && item.status === 'pending_review')
        .map((item) => item.id),
    [items, selectedRowKeys],
  );

  const openBatchModal = (kind: BatchReviewKind) => {
    if (selectedReviewableIds.length === 0) return;
    batchForm.resetFields();
    setBatchModal(kind);
  };

  const closeBatchModal = () => {
    if (batchLoading) return;
    setBatchModal(null);
  };

  const runBatchReview = async () => {
    if (!batchModal) return;
    const kind = batchModal;
    const values = await batchForm.validateFields();
    const ids = selectedReviewableIds;
    if (ids.length === 0) return;
    setBatchLoading(true);
    const failures: string[] = [];
    let ok = 0;
    try {
      for (const id of ids) {
        try {
          const detail = await getTask(id);
          const allowed = kind === 'approve' ? detail.allowedActions.canApprove : detail.allowedActions.canReject;
          if (!allowed || !detail.latestDraft) {
            failures.push(`${detail.title || id}：当前状态或权限不允许该操作`);
            continue;
          }
          const body = {
            draftVersion: detail.latestDraft.draftVersion,
            draftPayloadHash: detail.latestDraft.payloadHash,
            reason: values.reason,
            comment: values.comment,
            expectedTaskRevision: detail.revision,
          };
          if (kind === 'approve') {
            await approveTask(id, body, createOperationIdempotencyKey('batch-approve'));
          } else {
            await rejectTask(id, body, createOperationIdempotencyKey('batch-reject'));
          }
          ok += 1;
        } catch (e) {
          failures.push(operationErrorMessage(extractOperationTaskAPIError(e)) ?? '操作失败，请稍后重试。');
        }
      }
      if (ok > 0) message.success(`已${kind === 'approve' ? '批准' : '驳回'} ${ok} 个任务`);
      if (failures.length > 0) message.error(`${failures.length} 个任务操作失败：${failures[0]}`);
      setBatchModal(null);
      setSelectedRowKeys([]);
      await load();
    } finally {
      setBatchLoading(false);
    }
  };

  const columns = useMemo<ProColumns<OperationTaskSummary>[]>(() => [
    {
      title: '任务标题',
      dataIndex: 'title',
      width: 260,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Button type="link" style={{ padding: 0 }} onClick={() => history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(row.id)}`)}>
            {row.title || '未命名任务'}
          </Button>
          <Text type="secondary" ellipsis style={{ maxWidth: 240 }}>{row.summary || '—'}</Text>
        </Space>
      ),
    },
    { title: '任务类型', dataIndex: 'taskType', width: 130, render: (v) => taskTypeLabel(String(v || '')) },
    { title: '平台', dataIndex: 'platform', width: 110, render: (v) => platformLabel(String(v || '')) },
    { title: '状态', dataIndex: 'status', width: 130, render: (v) => <OperationTaskStatusTag status={String(v || '')} /> },
    { title: '优先级', dataIndex: 'priority', width: 100, render: (v) => <OperationPriorityTag priority={String(v || '')} /> },
    { title: '最新草稿', dataIndex: 'latestDraftVersion', width: 110, render: (v) => v ? `v${v}` : '—' },
    { title: '最新执行状态', dataIndex: 'latestExecutionStatus', width: 130, render: (v) => v ? <OperationAttemptStatusTag status={String(v)} /> : '—' },
    { title: '创建人', dataIndex: 'createdBy', width: 130, render: (v) => copyableText(String(v || ''), 10) },
    { title: '创建时间', dataIndex: 'createdAt', width: 130, render: (v) => <DateTimeText value={String(v || '')} /> },
    { title: '更新时间', dataIndex: 'updatedAt', width: 130, render: (v) => <DateTimeText value={String(v || '')} /> },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, row) => [
        <Button key="detail" type="link" icon={<EyeOutlined />} onClick={() => history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(row.id)}`)}>
          查看详情
        </Button>,
      ],
    },
  ], []);

  const filterActive = !!(urlState.status || urlState.platform || urlState.taskType);

  return (
    <TmPageContainer
      title={PAGE_COPY.operationTasks.title}
      subTitle={PAGE_COPY.operationTasks.description}
      extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <NonProductionBoundary />
        {error ? <ErrorAlert title={operationErrorMessage(error)} actionHint={error.traceId ? `排查编号：${error.traceId}` : '请稍后重试或联系管理员。'} /> : null}
        <Card>
          <Form form={form} layout="vertical" onFinish={updateFilters}>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="status" label="任务状态">
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_STATUS_LABELS)} placeholder="全部状态" />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="platform" label="平台">
                  <Select allowClear options={optionsFromLabels(OPERATION_PLATFORM_LABELS)} placeholder="全部平台" />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="taskType" label="任务类型">
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_TYPE_LABELS)} placeholder="全部类型" />
                </Form.Item>
              </Col>
              <Col xs={24} lg={6}>
                <Form.Item label="操作">
                  <Space wrap>
                    <Button type="primary" htmlType="submit">应用筛选</Button>
                    <Button onClick={clearFilters}>清除筛选</Button>
                  </Space>
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <Text type="secondary">筛选参数与 URL 同步；当前 API 使用 keyset cursor，不提供总页数。</Text>
        </Card>

        <TmProTable<OperationTaskSummary>
          rowKey="id"
          search={false}
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          locale={emptyLocale}
          scroll={{ x: 1560 }}
          options={false}
          toolBarRender={false}
          rowSelection={
            reviewable
              ? {
                  selectedRowKeys,
                  alwaysShowAlert: true,
                  onChange: (keys) => setSelectedRowKeys(keys.map(String)),
                }
              : undefined
          }
          tableAlertRender={({ selectedRowKeys: keys }) => (
            <Text>已选 {keys.length} 项，其中待审核 {selectedReviewableIds.length} 项（仅待审核任务可批量批准/驳回）</Text>
          )}
          tableAlertOptionRender={({ onCleanSelected }) => (
            <Space wrap>
              <Button
                type="primary"
                size="small"
                disabled={selectedReviewableIds.length === 0}
                onClick={() => openBatchModal('approve')}
              >
                批量批准（{selectedReviewableIds.length}）
              </Button>
              <Button
                danger
                size="small"
                disabled={selectedReviewableIds.length === 0}
                onClick={() => openBatchModal('reject')}
              >
                批量驳回（{selectedReviewableIds.length}）
              </Button>
              <a onClick={onCleanSelected}>取消选择</a>
            </Space>
          )}
        />

        <Modal
          title={batchModal === 'approve' ? `批量批准（${selectedReviewableIds.length} 个任务）` : `批量驳回（${selectedReviewableIds.length} 个任务）`}
          open={!!batchModal}
          onCancel={closeBatchModal}
          onOk={() => void runBatchReview()}
          confirmLoading={batchLoading}
          okText={batchModal === 'approve' ? '确认批量批准' : '确认批量驳回'}
          cancelText="取消"
          destroyOnHidden
        >
          <Form form={batchForm} layout="vertical">
            <Paragraph>
              将逐个对所选待审核任务的最新草稿执行{batchModal === 'approve' ? '批准' : '驳回'}，并汇总成功/失败结果；不会自动执行或发布商品。
            </Paragraph>
            <Form.Item
              name="reason"
              label={batchModal === 'approve' ? '批准说明' : '驳回原因'}
              rules={[{ required: true, whitespace: true, message: `请填写${batchModal === 'approve' ? '批准说明' : '驳回原因'}` }]}
            >
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
            <Form.Item name="comment" label="补充说明">
              <Input.TextArea rows={2} maxLength={500} showCount />
            </Form.Item>
          </Form>
        </Modal>

        <Card size="small">
          <Space wrap>
            <Button disabled={cursorStack.length === 0 || loading} onClick={goPrev}>上一批</Button>
            <Button type="primary" disabled={!hasMore || !nextCursor || loading} onClick={goNext}>下一批</Button>
            <Text type="secondary">
              当前显示 {items.length} 条；{filterActive ? '筛选已启用' : '未启用筛选'}；不会伪造页码或总数。
            </Text>
          </Space>
        </Card>
      </Space>
    </TmPageContainer>
  );
}

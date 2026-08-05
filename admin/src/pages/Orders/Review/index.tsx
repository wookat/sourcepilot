import { TmPageContainer } from '@/components/ui';
import {
  approveReviewOrders,
  listOrderReviewWorkbench,
  rejectReviewOrders,
  REVIEW_ACTION_LABELS,
  REVIEW_STATUS_COLORS,
  REVIEW_STATUS_LABELS,
  type ReviewDecisionResult,
  type ReviewOrderRow,
} from '@/services/orderReview';
import { isReadonly } from '@/utils/permission';
import { history, useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

const STATUS_FILTER_OPTIONS = [
  { value: '', label: '待处理（待审核 + 挂起）' },
  { value: 'pending_review', label: '待人工审核' },
  { value: 'held', label: '已挂起' },
  { value: 'approved', label: '已放行' },
  { value: 'rejected', label: '已拒绝' },
  { value: 'auto_passed', label: '自动通过' },
];

function summarizeDecision(kind: string, res: ReviewDecisionResult) {
  if (res.failed === 0) {
    message.success(`${kind}成功 ${res.done} 单`);
    return;
  }
  const firstErr = res.results.find((r) => !r.ok);
  message.warning(
    `${kind}完成：成功 ${res.done} 单，失败 ${res.failed} 单${
      firstErr ? `（${firstErr.orderNo || firstErr.orderId}：${firstErr.error}）` : ''
    }`,
  );
}

export default function OrderReviewWorkbenchPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<ReviewOrderRow[]>([]);
  const [total, setTotal] = useState(0);
  const [pendingTotal, setPendingTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState('');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [acting, setActing] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const res = await listOrderReviewWorkbench({
        page,
        pageSize,
        reviewStatus: statusFilter || undefined,
        keyword: keyword || undefined,
      });
      setRows(res.items || []);
      setTotal(res.total);
      setPendingTotal(res.pendingTotal);
      setSelected([]);
    } catch (e) {
      setLoadError((e as Error).message || '加载审单工作台失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, keyword]);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = async (kind: 'approve' | 'reject', ids: string[]) => {
    if (!ids.length) return;
    setActing(true);
    try {
      const res =
        kind === 'approve' ? await approveReviewOrders(ids) : await rejectReviewOrders(ids);
      summarizeDecision(kind === 'approve' ? '放行' : '拒绝', res);
      await load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setActing(false);
    }
  };

  const confirmReject = (ids: string[]) => {
    Modal.confirm({
      title: `拒绝 ${ids.length} 个订单？`,
      content: '拒绝后订单将进入取消动线（订单状态置为已取消），不可继续采购和发货。',
      okText: '拒绝并取消订单',
      okButtonProps: { danger: true },
      cancelText: '再想想',
      onOk: () => decide('reject', ids),
    });
  };

  const actionable = (r: ReviewOrderRow) =>
    r.reviewStatus === 'pending_review' || r.reviewStatus === 'held';

  return (
    <TmPageContainer
      title="审单工作台"
      subTitle="命中审单规则的待审核 / 挂起订单在这里集中处理；放行后回到正常流，拒绝进入取消动线。固定异常拦截（缺货源 / 负毛利等）请到「订单异常」处理"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            value={statusFilter}
            style={{ width: 220 }}
            options={STATUS_FILTER_OPTIONS}
            onChange={(v) => {
              setPage(1);
              setStatusFilter(v);
            }}
          />
          <Input.Search
            allowClear
            placeholder="搜索订单号 / 买家"
            style={{ width: 240 }}
            onSearch={(v) => {
              setPage(1);
              setKeyword(v.trim());
            }}
          />
          <Tooltip title={readonly ? '只读账号不可操作' : ''}>
            <Button
              type="primary"
              disabled={readonly || !selected.length}
              loading={acting}
              onClick={() => void decide('approve', selected)}
            >
              批量放行{selected.length ? `（${selected.length}）` : ''}
            </Button>
          </Tooltip>
          <Tooltip title={readonly ? '只读账号不可操作' : ''}>
            <Button
              danger
              disabled={readonly || !selected.length}
              loading={acting}
              onClick={() => confirmReject(selected)}
            >
              批量拒绝{selected.length ? `（${selected.length}）` : ''}
            </Button>
          </Tooltip>
          <Typography.Text type="secondary">待处理共 {pendingTotal} 单</Typography.Text>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载审单工作台失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<ReviewOrderRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          scroll={{ x: 1000 }}
          locale={{ emptyText: '暂无命中审单规则的订单' }}
          rowSelection={{
            selectedRowKeys: selected,
            getCheckboxProps: (r) => ({ disabled: readonly || !actionable(r) }),
            onChange: (keys) => setSelected(keys as string[]),
          }}
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
              title: '订单号',
              dataIndex: 'orderNo',
              width: 180,
              render: (v: string, r) => (
                <a onClick={() => history.push(`/orders/${r.id}`)}>{v}</a>
              ),
            },
            {
              title: '审核状态',
              dataIndex: 'reviewStatus',
              width: 110,
              render: (v: string) => (
                <Tag color={REVIEW_STATUS_COLORS[v] || 'default'}>
                  {REVIEW_STATUS_LABELS[v] || v}
                </Tag>
              ),
            },
            {
              title: '命中规则与原因',
              key: 'hits',
              render: (_, r) =>
                r.hits.length ? (
                  <Space direction="vertical" size={2}>
                    {r.hits.map((h) => (
                      <span key={h.id}>
                        <Tag color={h.decisive ? 'orange' : 'default'}>
                          {h.ruleName}
                          {h.decisive ? '（生效）' : ''}
                        </Tag>
                        <Typography.Text type="secondary">
                          {REVIEW_ACTION_LABELS[h.action] || h.action}：{h.reason}
                        </Typography.Text>
                      </span>
                    ))}
                  </Space>
                ) : (
                  <Typography.Text type="secondary">—</Typography.Text>
                ),
            },
            { title: '买家', dataIndex: 'customerName', width: 120 },
            {
              title: '金额',
              dataIndex: 'totalAmount',
              width: 120,
              render: (v: number, r) => `${r.currency} ${v.toFixed(2)}`,
            },
            { title: '店铺', dataIndex: 'shopName', width: 140, render: (v?: string) => v || '—' },
            {
              title: '操作',
              width: 150,
              render: (_, r) =>
                actionable(r) ? (
                  <Space size={4}>
                    <Popconfirm
                      title={`放行订单「${r.orderNo}」？`}
                      okText="放行"
                      disabled={readonly}
                      onConfirm={() => void decide('approve', [r.id])}
                    >
                      <Button size="small" type="link" disabled={readonly}>
                        放行
                      </Button>
                    </Popconfirm>
                    <Button
                      size="small"
                      type="link"
                      danger
                      disabled={readonly}
                      onClick={() => confirmReject([r.id])}
                    >
                      拒绝
                    </Button>
                  </Space>
                ) : (
                  <Typography.Text type="secondary">已处理</Typography.Text>
                ),
            },
          ]}
        />
      </Card>
    </TmPageContainer>
  );
}

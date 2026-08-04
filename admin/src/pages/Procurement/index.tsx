import GenerateResultAlerts from '@/components/procurement/GenerateResultAlerts';
import { TmPageContainer } from '@/components/ui';
import { platformLabel } from '@/constants/userFriendly';
import { commonStatusLabel } from '@/constants/copywriting';
import {
  cancelPurchaseOrder,
  confirmPurchaseOrder,
  downloadPurchaseOrderCsv,
  downloadPurchaseOrdersBatchCsv,
  fetchPurchaseOrders,
  generatePurchaseOrders,
  markPurchaseOrderDelivered,
  markPurchaseOrderPaid,
  submitPurchaseOrder,
  voidPurchaseOrder,
  type GenerateResult,
  type PurchaseOrder,
} from '@/services/procurement';
import { queryOrders, type OrderListRow } from '@/services/orders';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { isReadonly } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';
import { Link, useModel, useSearchParams } from '@umijs/max';
import BatchBackfillModal, { type BatchMode } from './BatchBackfillModal';
import {
  Alert,
  Button,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

export const PO_STATUS_TAG: Record<string, { text: string; color: string }> = {
  draft: { text: '草稿', color: 'default' },
  pending_confirm: { text: '待确认', color: 'gold' },
  placing: { text: '下单中(人工)', color: 'processing' },
  placed: { text: '已下单', color: 'blue' },
  paid: { text: '已付款', color: 'cyan' },
  shipped: { text: '已发货', color: 'geekblue' },
  delivered: { text: '已签收', color: 'green' },
  failed: { text: '失败', color: 'red' },
  cancelled: { text: '已取消', color: 'default' },
  voided: { text: '已作废', color: 'default' },
};

export const PO_VOIDABLE_STATUSES = ['delivered', 'failed', 'cancelled'];

export function confirmVoidPurchaseOrder(id: string, onDone: () => void) {
  Modal.confirm({
    title: '作废该采购单？',
    content:
      '作废用于处置测试单或错误单据：作废后单据保留审计记录，但不再参与统计与待办。注意：已入库的库存不会自动回滚，如需调整库存请到库存模块手工处理。',
    okText: '确认作废',
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await voidPurchaseOrder(id, '人工作废');
        message.success('已作废');
        onDone();
      } catch (e) {
        message.error((e as Error).message || '作废失败');
      }
    },
  });
}

const BATCH_SELECTABLE_STATUSES = ['draft', 'pending_confirm', 'placing', 'placed', 'shipped'];

const BATCH_ACTIONS: Record<
  'draft' | 'pending_confirm' | 'placed' | 'shipped',
  { actionText: string; emptyText: string; run: (id: string) => Promise<unknown> }
> = {
  draft: {
    actionText: '提交',
    emptyText: '所选中没有草稿状态的采购单',
    run: (id) => submitPurchaseOrder(id),
  },
  pending_confirm: {
    actionText: '确认',
    emptyText: '所选中没有待确认状态的采购单',
    run: (id) => confirmPurchaseOrder(id),
  },
  placed: {
    actionText: '标记付款',
    emptyText: '所选中没有已下单状态的采购单',
    run: (id) => markPurchaseOrderPaid(id),
  },
  shipped: {
    actionText: '标记签收',
    emptyText: '所选中没有已发货状态的采购单',
    run: (id) => markPurchaseOrderDelivered(id),
  },
};

export default function ProcurementOrdersPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = !isReadonly(initialState?.currentUser?.role);
  const emptyLocale = useListEmptyLocale('purchaseOrders', { permissionScoped: true });
  const [rows, setRows] = useState<PurchaseOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string | undefined>(() => {
    const s = (searchParams.get('status') || '').trim();
    return PO_STATUS_TAG[s] ? s : undefined;
  });
  const [batchMode, setBatchMode] = useState<BatchMode | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [batchActionLoading, setBatchActionLoading] = useState(false);

  const [genOpen, setGenOpen] = useState(false);
  const [salesOrders, setSalesOrders] = useState<OrderListRow[]>([]);
  const [selectedOrderIds, setSelectedOrderIds] = useState<string[]>([]);
  const [generating, setGenerating] = useState(false);
  const [genResult, setGenResult] = useState<GenerateResult | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchPurchaseOrders({ page, pageSize, status });
      setRows(res.items || []);
      setTotal(res.total || 0);
    } catch (e) {
      message.error((e as Error).message || '加载采购单失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, status]);

  useEffect(() => {
    void load();
  }, [load]);

  const runBatchAction = useCallback(
    async (targetStatus: 'draft' | 'pending_confirm' | 'placed' | 'shipped') => {
      const action = BATCH_ACTIONS[targetStatus];
      const targets = rows.filter(
        (r) => selectedRowKeys.includes(r.id) && r.status === targetStatus,
      );
      if (targets.length === 0) {
        message.warning(action.emptyText);
        return;
      }
      setBatchActionLoading(true);
      const failures: { id: string; message: string }[] = [];
      let succeeded = 0;
      for (const row of targets) {
        try {
          await action.run(row.id);
          succeeded += 1;
        } catch (e) {
          failures.push({ id: row.id, message: (e as Error)?.message || '操作失败' });
        }
      }
      setBatchActionLoading(false);
      const { actionText } = action;
      if (failures.length === 0) {
        message.success(`已批量${actionText} ${succeeded} 张采购单`);
      } else {
        Modal.warning({
          title: `批量${actionText}部分失败（成功 ${succeeded} / 失败 ${failures.length}）`,
          content: (
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {failures.map((f) => (
                <li key={f.id}>
                  {f.id.slice(0, 8)}：{f.message}
                </li>
              ))}
            </ul>
          ),
        });
      }
      setSelectedRowKeys([]);
      void load();
    },
    [rows, selectedRowKeys, load],
  );

  const runBatchExport = useCallback(async () => {
    if (selectedRowKeys.length === 0) return;
    setBatchActionLoading(true);
    try {
      await downloadPurchaseOrdersBatchCsv(selectedRowKeys);
      message.success(`已导出 ${selectedRowKeys.length} 张采购单的合并采购清单`);
    } catch (e) {
      message.error((e as Error).message || '导出失败');
    } finally {
      setBatchActionLoading(false);
    }
  }, [selectedRowKeys]);

  const selectedDraftCount = rows.filter(
    (r) => selectedRowKeys.includes(r.id) && r.status === 'draft',
  ).length;
  const selectedPendingConfirmCount = rows.filter(
    (r) => selectedRowKeys.includes(r.id) && r.status === 'pending_confirm',
  ).length;
  const selectedPlacedCount = rows.filter(
    (r) => selectedRowKeys.includes(r.id) && r.status === 'placed',
  ).length;
  const selectedShippedCount = rows.filter(
    (r) => selectedRowKeys.includes(r.id) && r.status === 'shipped',
  ).length;

  const openGenerate = async () => {
    setGenOpen(true);
    setGenResult(null);
    try {
      const res = await queryOrders({ page: 1, pageSize: 100 });
      setSalesOrders(res.list || []);
    } catch {
      message.error('加载销售订单失败');
    }
  };

  const doGenerate = async () => {
    if (selectedOrderIds.length === 0) {
      message.warning('请选择至少一个销售订单');
      return;
    }
    setGenerating(true);
    try {
      const res = await generatePurchaseOrders({ orderIds: selectedOrderIds });
      setGenResult(res);
      if ((res.orders || []).length > 0) {
        message.success(`已生成 ${res.orders.length} 张采购单`);
        void load();
      }
    } catch (e) {
      message.error((e as Error).message || '生成失败');
    } finally {
      setGenerating(false);
    }
  };

  return (
    <TmPageContainer title="采购协同（人工下单过渡）" subTitle="按货源档案聚合生成采购清单，导出后人工下单并回填">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="1688 官方 API 暂不可用：当前为人工下单过渡模式。系统生成采购清单（含 1688 链接、SKU、数量、参考价），导出 CSV 后请人工下单，并在详情页回填 1688 订单号 / 运单号。"
      />
      <Space style={{ marginBottom: 16 }} wrap>
        {writable && (
          <>
            <Button type="primary" onClick={() => void openGenerate()}>
              从销售订单生成采购单
            </Button>
            <Button onClick={() => setBatchMode('placed')}>批量回填 1688 订单号</Button>
            <Button onClick={() => setBatchMode('logistics')}>批量回填快递单号</Button>
          </>
        )}
        <Select
          allowClear
          placeholder="按状态筛选"
          style={{ width: 180 }}
          value={status}
          onChange={(v) => {
            setPage(1);
            setStatus(v);
            const next = new URLSearchParams(searchParams);
            if (v) {
              next.set('status', v);
            } else {
              next.delete('status');
            }
            setSearchParams(next, { replace: true });
          }}
          options={Object.entries(PO_STATUS_TAG).map(([value, cfg]) => ({ value, label: cfg.text }))}
        />
      </Space>
      {writable && (
        <Alert
          type="info"
          style={{ marginBottom: 12 }}
          message={
            <Space wrap>
              <span>已选 {selectedRowKeys.length} 张采购单</span>
              <Button
                size="small"
                type="primary"
                loading={batchActionLoading}
                disabled={selectedDraftCount === 0}
                onClick={() => void runBatchAction('draft')}
              >
                批量提交（{selectedDraftCount}）
              </Button>
              <Button
                size="small"
                loading={batchActionLoading}
                disabled={selectedPendingConfirmCount === 0}
                onClick={() => void runBatchAction('pending_confirm')}
              >
                批量确认（{selectedPendingConfirmCount}）
              </Button>
              <Button
                size="small"
                loading={batchActionLoading}
                disabled={selectedPlacedCount === 0}
                onClick={() => void runBatchAction('placed')}
              >
                批量标记付款（{selectedPlacedCount}）
              </Button>
              <Button
                size="small"
                loading={batchActionLoading}
                disabled={selectedShippedCount === 0}
                onClick={() => void runBatchAction('shipped')}
              >
                批量标记签收（{selectedShippedCount}）
              </Button>
              <Button
                size="small"
                loading={batchActionLoading}
                disabled={selectedRowKeys.length === 0}
                onClick={() => void runBatchExport()}
              >
                批量导出清单（{selectedRowKeys.length}）
              </Button>
              <a onClick={() => setSelectedRowKeys([])}>取消选择</a>
            </Space>
          }
        />
      )}
      <Table<PurchaseOrder>
        rowKey="id"
        loading={loading}
        locale={emptyLocale}
        rowSelection={
          writable
            ? {
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys.map(String)),
                getCheckboxProps: (r) => ({
                  disabled: !BATCH_SELECTABLE_STATUSES.includes(r.status),
                }),
              }
            : undefined
        }
        dataSource={rows}
        scroll={{ x: 1000 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
        columns={[
          {
            title: '采购单',
            dataIndex: 'id',
            width: 120,
            render: (v: string) => <Link to={`/procurement/orders/${v}`}>{v.slice(0, 8)}</Link>,
          },
          { title: '供应商', dataIndex: 'supplierName', width: 160 },
          {
            title: '状态',
            dataIndex: 'status',
            width: 120,
            render: (v: string) => {
              const cfg = PO_STATUS_TAG[v] || { text: v, color: 'default' };
              return <Tag color={cfg.color}>{cfg.text}</Tag>;
            },
          },
          {
            title: '金额',
            dataIndex: 'totalAmount',
            width: 120,
            render: (v: number, row) => `${v.toFixed(2)} ${row.currency}`,
          },
          {
            title: '1688订单号',
            dataIndex: 'externalOrderId',
            width: 160,
            render: (v) => v || '-',
          },
          {
            title: '创建时间',
            dataIndex: 'createdAt',
            width: 180,
            render: (v: string) => formatDateTime(v),
          },
          {
            title: '操作',
            width: 260,
            render: (_, row) => (
              <Space wrap>
                <a onClick={() => void downloadPurchaseOrderCsv(row.id).catch((e) => message.error(e.message))}>
                  导出CSV
                </a>
                {row.status === 'draft' && (
                  <a
                    onClick={async () => {
                      try {
                        await submitPurchaseOrder(row.id);
                        message.success('已提交待确认');
                        void load();
                      } catch (e) {
                        message.error((e as Error).message || '提交失败');
                      }
                    }}
                  >
                    提交
                  </a>
                )}
                {row.status === 'pending_confirm' && (
                  <a
                    onClick={async () => {
                      try {
                        await confirmPurchaseOrder(row.id);
                        message.success('已确认，请导出清单人工下单');
                        void load();
                      } catch (e) {
                        message.error((e as Error).message || '确认失败');
                      }
                    }}
                  >
                    确认
                  </a>
                )}
                {['draft', 'pending_confirm', 'placing', 'placed', 'failed'].includes(row.status) && (
                  <Popconfirm
                    title="取消该采购单？"
                    onConfirm={async () => {
                      try {
                        await cancelPurchaseOrder(row.id, '人工取消');
                        message.success('已取消');
                        void load();
                      } catch (e) {
                        message.error((e as Error).message || '取消失败');
                      }
                    }}
                  >
                    <a style={{ color: '#ff4d4f' }}>取消</a>
                  </Popconfirm>
                )}
                {writable && PO_VOIDABLE_STATUSES.includes(row.status) && (
                  <a
                    style={{ color: '#ff4d4f' }}
                    onClick={() => confirmVoidPurchaseOrder(row.id, () => void load())}
                  >
                    作废
                  </a>
                )}
              </Space>
            ),
          },
        ]}
      />
      {batchMode && (
        <BatchBackfillModal
          mode={batchMode}
          open
          onClose={() => setBatchMode(null)}
          onDone={() => void load()}
        />
      )}
      <Modal
        title="从销售订单生成采购单"
        open={genOpen}
        width={640}
        onCancel={() => {
          setGenOpen(false);
          setSelectedOrderIds([]);
        }}
        onOk={() => void doGenerate()}
        okButtonProps={{ loading: generating }}
        okText="生成采购清单"
      >
        <Typography.Paragraph type="secondary">
          选择已付款的海外销售订单，系统按「商品货源档案」中的主供应商聚合生成采购清单。
        </Typography.Paragraph>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          placeholder="选择销售订单"
          value={selectedOrderIds}
          onChange={setSelectedOrderIds}
          optionFilterProp="label"
          options={salesOrders.map((o) => ({
            value: o.id,
            label: `${o.orderNo || o.id.slice(0, 8)}（${platformLabel(o.platform)} / ${commonStatusLabel(o.status)}）`,
          }))}
        />
        {genResult && ((genResult.blockers || []).length > 0 || (genResult.warnings || []).length > 0) && (
          <div style={{ marginTop: 16 }}>
            <GenerateResultAlerts
              blockers={genResult.blockers}
              warnings={genResult.warnings}
              onNavigate={() => {
                setGenOpen(false);
                setSelectedOrderIds([]);
              }}
            />
          </div>
        )}
      </Modal>
    </TmPageContainer>
  );
}

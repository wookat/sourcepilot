import { TmPageContainer } from '@/components/ui';
import {
  cancelPurchaseOrder,
  confirmPurchaseOrder,
  downloadPurchaseOrderCsv,
  fetchPurchaseOrders,
  generatePurchaseOrders,
  submitPurchaseOrder,
  type GenerateResult,
  type PurchaseOrder,
} from '@/services/procurement';
import { queryOrders, type OrderListRow } from '@/services/orders';
import { Link } from '@umijs/max';
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
};

export default function ProcurementOrdersPage() {
  const [rows, setRows] = useState<PurchaseOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string>();
  const [loading, setLoading] = useState(false);

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
        <Button type="primary" onClick={() => void openGenerate()}>
          从销售订单生成采购单
        </Button>
        <Select
          allowClear
          placeholder="按状态筛选"
          style={{ width: 180 }}
          value={status}
          onChange={(v) => {
            setPage(1);
            setStatus(v);
          }}
          options={Object.entries(PO_STATUS_TAG).map(([value, cfg]) => ({ value, label: cfg.text }))}
        />
      </Space>
      <Table<PurchaseOrder>
        rowKey="id"
        loading={loading}
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
          { title: '创建时间', dataIndex: 'createdAt', width: 180 },
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
              </Space>
            ),
          },
        ]}
      />
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
            label: `${o.orderNo || o.id.slice(0, 8)}（${o.platform} / ${o.status}）`,
          }))}
        />
        {genResult && (genResult.blockers || []).length > 0 && (
          <Alert
            style={{ marginTop: 16 }}
            type="warning"
            showIcon
            message="部分订单行未能进入采购清单"
            description={
              <ul style={{ margin: 0, paddingLeft: 18 }}>
                {(genResult.blockers || []).map((b, i) => (
                  <li key={i}>
                    订单 {b.orderId.slice(0, 8)}：{b.message}
                    {b.skuName ? `（${b.skuName}）` : ''}
                  </li>
                ))}
              </ul>
            }
          />
        )}
      </Modal>
    </TmPageContainer>
  );
}

import { TmPageContainer } from '@/components/ui';
import {
  cancelPurchaseOrder,
  confirmPurchaseOrder,
  downloadPurchaseOrderCsv,
  fetchPurchaseOrder,
  fillPurchaseLogistics,
  markPurchaseOrderDelivered,
  markPurchaseOrderPaid,
  markPurchaseOrderPlaced,
  retryPurchaseOrder,
  submitPurchaseOrder,
  updatePurchaseItemPrice,
  type PurchaseOrder,
  type PurchaseOrderItem,
} from '@/services/procurement';
import { isReadonly } from '@/utils/permission';
import { Link, useModel, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Steps,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { confirmVoidPurchaseOrder, PO_STATUS_TAG, PO_VOIDABLE_STATUSES } from './index';

const FLOW = ['draft', 'pending_confirm', 'placing', 'placed', 'paid', 'shipped', 'delivered'];

export default function ProcurementOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = !isReadonly(initialState?.currentUser?.role);
  const [po, setPo] = useState<PurchaseOrder | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const [placedOpen, setPlacedOpen] = useState(false);
  const [placedForm] = Form.useForm();
  const [logisticsOpen, setLogisticsOpen] = useState(false);
  const [logisticsForm] = Form.useForm();
  const [paidOpen, setPaidOpen] = useState(false);
  const [paidForm] = Form.useForm();
  const [priceEdit, setPriceEdit] = useState<{ itemId: string; value?: number } | null>(null);
  const [priceSaving, setPriceSaving] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(undefined);
    try {
      setPo(await fetchPurchaseOrder(id));
    } catch (e) {
      setError((e as Error).message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (fn: () => Promise<unknown>, ok: string) => {
    try {
      await fn();
      message.success(ok);
      void load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    }
  };

  if (loading && !po) {
    return (
      <TmPageContainer title="采购单详情">
        <Spin />
      </TmPageContainer>
    );
  }
  if (error || !po || !id) {
    return (
      <TmPageContainer title="采购单详情">
        {error ? <Alert type="error" showIcon message={error} /> : <Empty />}
      </TmPageContainer>
    );
  }

  const statusCfg = PO_STATUS_TAG[po.status] || { text: po.status, color: 'default' };
  const stepIdx = FLOW.indexOf(po.status);
  const priceEditable = ['draft', 'pending_confirm'].includes(po.status);
  const missingPriceCount = (po.items || []).filter(
    (it) => it.expectedPrice === undefined || it.expectedPrice === null,
  ).length;
  const salesOrderIds = Array.from(
    new Set((po.items || []).map((it) => it.salesOrderId).filter((v): v is string => !!v)),
  );

  const savePrice = async (itemId: string, value?: number) => {
    if (value === undefined || value === null || value <= 0) {
      message.warning('请输入大于 0 的参考价');
      return;
    }
    setPriceSaving(true);
    try {
      setPo(await updatePurchaseItemPrice(po.id, itemId, value));
      message.success('参考价已更新，金额已重算');
      setPriceEdit(null);
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setPriceSaving(false);
    }
  };

  return (
    <TmPageContainer
      title={`采购单 ${po.id.slice(0, 8)}`}
      subTitle={<Tag color={statusCfg.color}>{statusCfg.text}</Tag>}
    >
      {['failed', 'cancelled', 'voided'].includes(po.status) ? (
        <Alert
          style={{ marginBottom: 16 }}
          type={po.status === 'failed' ? 'error' : 'warning'}
          showIcon
          message={
            po.status === 'failed'
              ? `采购失败：${po.errorMessage || '未知原因'}`
              : po.status === 'voided'
                ? '采购单已作废：不再参与统计与待办，已入库库存未自动回滚'
                : '采购单已取消'
          }
        />
      ) : (
        <Steps
          style={{ marginBottom: 24 }}
          size="small"
          current={stepIdx < 0 ? 0 : stepIdx}
          items={FLOW.map((s) => ({ title: PO_STATUS_TAG[s]?.text || s }))}
        />
      )}

      <Space style={{ marginBottom: 16 }} wrap>
        <Button onClick={() => void downloadPurchaseOrderCsv(po.id).catch((e) => message.error(e.message))}>
          导出采购清单 CSV
        </Button>
        {po.status === 'draft' && (
          <Button type="primary" onClick={() => void run(() => submitPurchaseOrder(po.id), '已提交待确认')}>
            提交待确认
          </Button>
        )}
        {po.status === 'pending_confirm' && (
          <Button type="primary" onClick={() => void run(() => confirmPurchaseOrder(po.id), '已确认，请人工前往 1688 下单')}>
            确认采购
          </Button>
        )}
        {po.status === 'placing' && (
          <Button type="primary" onClick={() => setPlacedOpen(true)}>
            回填 1688 订单号
          </Button>
        )}
        {po.status === 'placed' && (
          <Button type="primary" onClick={() => setPaidOpen(true)}>
            标记已付款
          </Button>
        )}
        {po.status === 'paid' && (
          <Button type="primary" onClick={() => setLogisticsOpen(true)}>
            回填运单号
          </Button>
        )}
        {po.status === 'shipped' && (
          <Button type="primary" onClick={() => void run(() => markPurchaseOrderDelivered(po.id), '已标记签收，采购数量已入库到本地库存')}>
            标记签收
          </Button>
        )}
        {po.status === 'failed' && (
          <Button onClick={() => void run(() => retryPurchaseOrder(po.id), '已重新进入下单流程')}>重试</Button>
        )}
        {['draft', 'pending_confirm', 'placing', 'placed', 'failed'].includes(po.status) && (
          <Popconfirm title="取消该采购单？" onConfirm={() => void run(() => cancelPurchaseOrder(po.id, '人工取消'), '已取消')}>
            <Button danger>取消采购单</Button>
          </Popconfirm>
        )}
        {writable && PO_VOIDABLE_STATUSES.includes(po.status) && (
          <Button danger onClick={() => confirmVoidPurchaseOrder(po.id, () => void load())}>
            作废采购单
          </Button>
        )}
      </Space>

      <Descriptions bordered size="small" column={2} style={{ marginBottom: 24 }}>
        <Descriptions.Item label="供应商">{po.supplierName}</Descriptions.Item>
        <Descriptions.Item label="采购平台">{po.sourcePlatform}</Descriptions.Item>
        <Descriptions.Item label="1688 订单号">{po.externalOrderId || '-'}</Descriptions.Item>
        <Descriptions.Item label="金额">{`${po.totalAmount.toFixed(2)} ${po.currency}`}</Descriptions.Item>
        <Descriptions.Item label="支付状态">{po.payStatus}</Descriptions.Item>
        <Descriptions.Item label="支付渠道">{po.payChannel || '-'}</Descriptions.Item>
        <Descriptions.Item label="创建时间">{po.createdAt}</Descriptions.Item>
        <Descriptions.Item label="确认时间">{po.confirmedAt || '-'}</Descriptions.Item>
        <Descriptions.Item label="来源销售订单" span={2}>
          {salesOrderIds.length === 0 ? (
            '-'
          ) : (
            <Space wrap>
              {salesOrderIds.map((sid) => (
                <Link key={sid} to={`/orders/${sid}`}>{sid.slice(0, 8)}</Link>
              ))}
            </Space>
          )}
        </Descriptions.Item>
      </Descriptions>

      <Typography.Title level={5}>采购明细</Typography.Title>
      {missingPriceCount > 0 && (
        <Alert
          style={{ marginBottom: 12 }}
          type="warning"
          showIcon
          message={`${missingPriceCount} 行缺参考进价，采购单金额不含这些行`}
          description={
            priceEditable
              ? '可直接在下方明细的「参考价」列补填；也可到货源档案的 SKU 映射中维护参考进价'
              : '当前状态不可修改参考价，可在回填 1688 订单号时填写实际价'
          }
        />
      )}
      <Table
        rowKey="id"
        size="small"
        dataSource={po.items || []}
        pagination={false}
        scroll={{ x: 1000 }}
        columns={[
          { title: '商品', dataIndex: 'productTitle', ellipsis: true },
          { title: 'SKU', dataIndex: 'skuName', width: 160 },
          {
            title: '1688 链接',
            dataIndex: 'sourceUrl',
            ellipsis: true,
            render: (v?: string) =>
              v ? (
                <Typography.Link href={v} target="_blank">
                  {v}
                </Typography.Link>
              ) : (
                '-'
              ),
          },
          { title: '货源规格', dataIndex: 'externalSkuId', width: 120, render: (v) => v || '-' },
          { title: '数量', dataIndex: 'quantity', width: 80 },
          {
            title: '来源订单',
            dataIndex: 'salesOrderId',
            width: 110,
            render: (v?: string) => (v ? <Link to={`/orders/${v}`}>{v.slice(0, 8)}</Link> : '-'),
          },
          {
            title: '参考价',
            dataIndex: 'expectedPrice',
            width: 180,
            render: (v: number | undefined, row: PurchaseOrderItem) => {
              if (priceEdit && priceEdit.itemId === row.id) {
                return (
                  <Space.Compact style={{ width: '100%' }}>
                    <InputNumber
                      size="small"
                      min={0.01}
                      step={0.01}
                      style={{ width: '100%' }}
                      value={priceEdit.value}
                      onChange={(nv) => setPriceEdit({ itemId: row.id, value: nv ?? undefined })}
                      onPressEnter={() => void savePrice(row.id, priceEdit.value)}
                    />
                    <Button
                      size="small"
                      type="primary"
                      loading={priceSaving}
                      onClick={() => void savePrice(row.id, priceEdit.value)}
                    >
                      保存
                    </Button>
                    <Button size="small" disabled={priceSaving} onClick={() => setPriceEdit(null)}>
                      取消
                    </Button>
                  </Space.Compact>
                );
              }
              const text =
                v !== undefined && v !== null ? (
                  v.toFixed(2)
                ) : (
                  <Tag color="warning">缺参考价</Tag>
                );
              if (!priceEditable) return text;
              return (
                <Space>
                  {text}
                  <a onClick={() => setPriceEdit({ itemId: row.id, value: v ?? undefined })}>填价</a>
                </Space>
              );
            },
          },
          {
            title: '实际价',
            dataIndex: 'actualPrice',
            width: 100,
            render: (v?: number) => (v !== undefined && v !== null ? v.toFixed(2) : '-'),
          },
        ]}
      />

      <Typography.Title level={5} style={{ marginTop: 24 }}>
        物流
      </Typography.Title>
      {(po.logistics || []).length === 0 ? (
        <Empty description="暂无物流信息" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <Table
          rowKey="id"
          size="small"
          dataSource={po.logistics || []}
          pagination={false}
          columns={[
            { title: '运单号', dataIndex: 'trackingNo' },
            { title: '承运商', dataIndex: 'carrier', render: (v) => v || '-' },
            { title: '状态', dataIndex: 'status', render: (v) => v || '-' },
          ]}
        />
      )}

      <Typography.Title level={5} style={{ marginTop: 24 }}>
        状态流转记录
      </Typography.Title>
      <Table
        rowKey="id"
        size="small"
        dataSource={po.events || []}
        pagination={false}
        columns={[
          { title: '时间', dataIndex: 'createdAt', width: 200 },
          {
            title: '从',
            dataIndex: 'fromStatus',
            width: 140,
            render: (v?: string) => (v ? PO_STATUS_TAG[v]?.text || v : '-'),
          },
          {
            title: '到',
            dataIndex: 'toStatus',
            width: 140,
            render: (v: string) => PO_STATUS_TAG[v]?.text || v,
          },
          { title: '来源', dataIndex: 'source', width: 120 },
        ]}
      />

      <Modal
        title="回填 1688 订单号"
        open={placedOpen}
        destroyOnClose
        onCancel={() => setPlacedOpen(false)}
        onOk={async () => {
          const values = await placedForm.validateFields();
          await run(() => markPurchaseOrderPlaced(po.id, values.externalOrderId.trim()), '已记录 1688 订单号');
          setPlacedOpen(false);
        }}
      >
        <Form form={placedForm} layout="vertical">
          <Form.Item
            name="externalOrderId"
            label="1688 订单号"
            rules={[{ required: true, message: '请输入 1688 订单号' }]}
          >
            <Input placeholder="人工在 1688 下单后获得的订单号" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="标记已付款"
        open={paidOpen}
        destroyOnClose
        onCancel={() => setPaidOpen(false)}
        onOk={async () => {
          const values = await paidForm.validateFields();
          await run(() => markPurchaseOrderPaid(po.id, values.payChannel), '已标记付款');
          setPaidOpen(false);
        }}
      >
        <Form form={paidForm} layout="vertical" initialValues={{ payChannel: 'alipay' }}>
          <Form.Item name="payChannel" label="支付渠道">
            <Select
              options={[
                { value: 'alipay', label: '支付宝' },
                { value: 'bank', label: '对公转账' },
                { value: 'other', label: '其他' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="回填运单号"
        open={logisticsOpen}
        destroyOnClose
        onCancel={() => setLogisticsOpen(false)}
        onOk={async () => {
          const values = await logisticsForm.validateFields();
          await run(
            () => fillPurchaseLogistics(po.id, values.trackingNo.trim(), values.carrier?.trim()),
            '已记录运单号',
          );
          setLogisticsOpen(false);
        }}
      >
        <Form form={logisticsForm} layout="vertical">
          <Form.Item name="trackingNo" label="运单号" rules={[{ required: true, message: '请输入运单号' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="carrier" label="承运商（可选）">
            <Input placeholder="如：中通 / 圆通 / 顺丰" />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

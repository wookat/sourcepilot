import { TmPageContainer, TechnicalDetails, TaskJsonBlock } from '@/components/ui';
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { appendSourceToUrl } from '@/utils/urlState';
import { history, useModel, useParams, useSearchParams } from '@umijs/max';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_ITEM_SKU_MATCH_STATUS,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  ORDER_SYNC_SUMMARY,
} from '@/constants/status';
import {
  createOrderShipment,
  deductOrderInventory,
  deleteOrderShipment,
  getOrder,
  getOrderInventoryEffects,
  getOrderSKUMatches,
  restoreOrderInventory,
  updateOrder,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderShipmentRow,
  type OrderSkuMatchRow,
} from '@/services/orders';
import type { OrderInventoryEffectRow } from '@/services/inventory';
import OrderSkuMatchTab from '@/pages/Orders/SkuMatchTab';
import { PRODUCT_COPY } from '@/constants/copywriting';
import {
  INVENTORY_DEDUCT_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { canWriteOrders } from '@/utils/orderPerm';
import { fetchOrderCostEstimate, type OrderCostEstimate } from '@/services/procurement';

function tagFromMap(raw: string, map: Record<string, { text: string; color: string }>) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw || '—'}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

function summarizeInvResp(action: string, sum?: Record<string, unknown>) {
  if (!sum) return `${action}完成`;
  if (sum.skipped) return `已跳过（幂等）：${String(sum.skipReason || '无需重复执行')}`;
  const msg = typeof sum.message === 'string' ? sum.message.trim() : '';
  if (msg && msg.toLowerCase() !== 'ok') return msg;
  return `${action}成功，影响流水已更新`;
}

export default function OrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const itemIdFocus = searchParams.get('itemId')?.trim();
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = canWriteOrders(initialState?.currentUser?.role);

  const [detail, setDetail] = useState<OrderDetailDTO | null>(null);
  const [skuRows, setSkuRows] = useState<OrderSkuMatchRow[]>([]);
  const [invRows, setInvRows] = useState<OrderInventoryEffectRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('overview');
  const [shipModal, setShipModal] = useState<{ open: boolean; row?: OrderShipmentRow | null }>({
    open: false,
  });
  const [shipForm] = Form.useForm();
  const [markPaidLoading, setMarkPaidLoading] = useState(false);
  const [invActionLoading, setInvActionLoading] = useState(false);
  const [costEst, setCostEst] = useState<OrderCostEstimate | null>(null);

  const openShipModal = (row?: OrderShipmentRow) => {
    shipForm.resetFields();
    if (row) {
      shipForm.setFieldsValue({
        carrier: row.carrier,
        trackingNo: row.trackingNo,
        trackingUrl: row.trackingUrl,
        status: row.status,
      });
    }
    setShipModal({ open: true, row: row ?? null });
  };

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [d, sku, inv] = await Promise.all([
        getOrder(id),
        getOrderSKUMatches(id),
        getOrderInventoryEffects(id, { page: 1, pageSize: 100 }),
      ]);
      setDetail(d);
      setSkuRows(sku.items ?? []);
      setInvRows(inv.list ?? []);
      try {
        setCostEst(await fetchOrderCostEstimate(id));
      } catch {
        setCostEst(null);
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载订单失败');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (itemIdFocus) setActiveTab('sku');
  }, [itemIdFocus]);

  useEffect(() => {
    const tab = searchParams.get('tab')?.trim();
    if (tab === 'inventory' || tab === 'inv') setActiveTab('inv');
    else if (tab === 'sku') setActiveTab('sku');
    else if (tab === 'exceptions') setActiveTab('exceptions');
  }, [searchParams]);

  const listSummary = useMemo(() => {
    if (!detail) return null;
    const matched = skuRows.filter((r) => ['matched', 'manual_bound'].includes(String(r.matchStatus))).length;
    const total = skuRows.length || detail.items.length;
    let skuStatus = 'none';
    if (total > 0) {
      if (skuRows.some((r) => r.matchStatus === 'ambiguous')) skuStatus = 'ambiguous';
      else if (skuRows.some((r) => r.matchStatus === 'unmatched')) skuStatus = 'unmatched';
      else if (matched >= total) skuStatus = 'all_matched';
      else skuStatus = 'partial';
    }
    const invFailed = invRows.some((r) => r.status === 'failed');
    const invOk = invRows.some((r) => r.effectType === 'deduct' && r.status === 'success');
    let invStatus = 'none';
    if (skuStatus === 'unmatched' || skuStatus === 'ambiguous') invStatus = 'blocked';
    else if (invOk && invFailed) invStatus = 'partial';
    else if (invFailed) invStatus = 'failed';
    else if (invOk) invStatus = 'success';
    const syncSt =
      detail.platform === 'manual' || !detail.externalOrderId ? 'manual' : detail.externalOrderId ? 'synced' : 'unknown';
    return { skuStatus, invStatus, syncSt, matched, total };
  }, [detail, skuRows, invRows]);

  if (!id) {
    return (
      <TmPageContainer title="订单详情">
        <Alert type="error" message="缺少订单 ID" />
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer
      title={detail ? `订单 ${detail.orderNo}` : '订单详情'}
      loading={loading}
      onBack={() => {
        if (window.history.length > 1) {
          history.goBack();
          return;
        }
        history.push('/orders/list');
      }}
      extra={
        <Space wrap>
          <Button
            onClick={() =>
              history.push(
                appendSourceToUrl(`/orders/exceptions?orderId=${encodeURIComponent(id!)}`, 'order_detail'),
              )
            }
          >
            异常工作台
          </Button>
          <Button onClick={() => history.push('/ops/task-center/failures?taskType=inventory_sync')}>
            失败任务中心
          </Button>
          <Button
            onClick={() =>
              history.push(
                appendSourceToUrl(
                  `/inventory/deductions?orderId=${encodeURIComponent(id!)}`,
                  'order_detail',
                ),
              )
            }
          >
            扣减记录
          </Button>
          <Button type="link" onClick={() => history.goBack()}>
            返回列表
          </Button>
        </Space>
      }
    >
      {detail?.shopSummary?.authStatus === 'unauthorized' || detail?.shopSummary?.authStatus === 'expired' ? (
        <Alert
          showIcon
          type="warning"
          style={{ marginBottom: 16 }}
          message="抖店/平台凭证待真实授权"
          description="当前为 Demo / RC 环境展示。未配置真实店铺凭证时，平台订单不会自动同步成功，请勿误以为已接入真实订单。"
        />
      ) : null}

      {detail && listSummary && listSummary.total > 0 && listSummary.matched < listSummary.total ? (
        <Alert
          showIcon
          type="info"
          style={{ marginBottom: 16 }}
          message="存在未完全匹配的 SKU"
          description={
            <span>
              请先在「规格匹配」Tab 人工确认候选或绑定本地 SKU，再执行库存扣减。{' '}
              <Typography.Link onClick={() => setActiveTab('sku')}>前往规格匹配</Typography.Link>
            </span>
          }
        />
      ) : null}

      {detail ? (
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'overview',
              label: '订单概览',
              children: (
                <Row gutter={[16, 16]}>
                  <Col span={24}>
                    <Card size="small" title="基本信息">
                      <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small" bordered>
                        <Descriptions.Item label="订单号">{detail.orderNo}</Descriptions.Item>
                        <Descriptions.Item label="平台订单号">{detail.externalOrderId || '—'}</Descriptions.Item>
                        <Descriptions.Item label="平台">{detail.platform}</Descriptions.Item>
                        <Descriptions.Item label="店铺">
                          {detail.shopSummary?.shopName || '—'}
                          {detail.shopSummary?.platform ? ` (${detail.shopSummary.platform})` : ''}
                        </Descriptions.Item>
                        <Descriptions.Item label="订单状态">
                          {tagFromMap(detail.status, ORDER_STATUS)}
                        </Descriptions.Item>
                        <Descriptions.Item label="付款状态">
                          <Space>
                            {tagFromMap(detail.paymentStatus, ORDER_PAYMENT_STATUS)}
                            {writable && detail.paymentStatus === 'unpaid' ? (
                              <Popconfirm
                                title="确认将该订单标记为已付款？"
                                onConfirm={async () => {
                                  setMarkPaidLoading(true);
                                  try {
                                    await updateOrder(detail.id, { paymentStatus: 'paid' });
                                    message.success('已标记为已付款');
                                    await load();
                                  } catch (e: unknown) {
                                    message.error((e as Error)?.message || '标记失败');
                                  } finally {
                                    setMarkPaidLoading(false);
                                  }
                                }}
                              >
                                <Button size="small" loading={markPaidLoading}>
                                  标记已付款
                                </Button>
                              </Popconfirm>
                            ) : null}
                          </Space>
                        </Descriptions.Item>
                        <Descriptions.Item label="履约状态">
                          {tagFromMap(detail.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}
                        </Descriptions.Item>
                        <Descriptions.Item label="金额">
                          {detail.currency} {detail.totalAmount}
                        </Descriptions.Item>
                        <Descriptions.Item label="下单时间">
                          {detail.orderedAt ? formatDateTime(detail.orderedAt) : '—'}
                        </Descriptions.Item>
                        <Descriptions.Item label="创建时间">{formatDateTime(detail.createdAt)}</Descriptions.Item>
                        <Descriptions.Item label="更新时间">{formatDateTime(detail.updatedAt)}</Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="规格匹配">
                      {listSummary ? tagFromMap(listSummary.skuStatus, ORDER_SKU_MATCH_SUMMARY) : '—'}
                      <div style={{ marginTop: 8 }}>
                        {listSummary ? `${listSummary.matched}/${listSummary.total} 行已匹配` : ''}
                      </div>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="库存影响">
                      {listSummary ? tagFromMap(listSummary.invStatus, ORDER_INVENTORY_DEDUCT_SUMMARY) : '—'}
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="同步状态">
                      {listSummary ? tagFromMap(listSummary.syncSt, ORDER_SYNC_SUMMARY) : '—'}
                    </Card>
                  </Col>
                  <Col span={24}>
                    <Card size="small" title="成本 / 毛利估算（基于主货源参考进价）">
                      {costEst ? (
                        <>
                          <Descriptions column={{ xs: 1, sm: 2, md: 4 }} size="small">
                            <Descriptions.Item label="销售额">
                              {costEst.currency} {costEst.totalAmount}
                            </Descriptions.Item>
                            <Descriptions.Item label="预估采购成本">
                              CNY {costEst.estimatedCostCny}
                              {costEst.estimatedCost != null && costEst.currency !== 'CNY'
                                ? ` ≈ ${costEst.currency} ${costEst.estimatedCost}`
                                : ''}
                            </Descriptions.Item>
                            <Descriptions.Item label="预估毛利">
                              {costEst.grossProfit != null ? (
                                <Typography.Text type={costEst.grossProfit >= 0 ? 'success' : 'danger'} strong>
                                  {costEst.currency} {costEst.grossProfit}
                                </Typography.Text>
                              ) : (
                                '—'
                              )}
                            </Descriptions.Item>
                            <Descriptions.Item label="毛利率">
                              {costEst.marginPercent != null ? `${costEst.marginPercent}%` : '—'}
                            </Descriptions.Item>
                          </Descriptions>
                          {costEst.missingLines > 0 ? (
                            <Alert
                              showIcon
                              type="warning"
                              style={{ marginTop: 8 }}
                              message={`${costEst.missingLines} 行无法估算成本（未匹配 SKU / 缺主货源 / 缺参考进价），毛利仅在全部行可估算时计算`}
                            />
                          ) : null}
                          {costEst.exchangeRate == null && costEst.currency !== 'CNY' ? (
                            <Alert
                              showIcon
                              type="info"
                              style={{ marginTop: 8 }}
                              message="未配置汇率（系统设置 → 定价 exchangeRate），成本仅按 CNY 展示，暂无法折算毛利"
                            />
                          ) : null}
                        </>
                      ) : (
                        <Typography.Text type="secondary">暂无可用估算数据</Typography.Text>
                      )}
                    </Card>
                  </Col>
                </Row>
              ),
            },
            {
              key: 'buyer',
              label: '买家信息',
              children: (
                <Descriptions column={1} bordered size="small">
                  <Descriptions.Item label="买家">{detail.customerName}</Descriptions.Item>
                  <Descriptions.Item label="邮箱">{detail.customerEmail || '—'}</Descriptions.Item>
                  <Descriptions.Item label="电话">{detail.customerPhone || '—'}</Descriptions.Item>
                  <Descriptions.Item label="说明">
                    <Typography.Text type="secondary">联系方式已脱敏展示；完整信息需相应权限。</Typography.Text>
                  </Descriptions.Item>
                </Descriptions>
              ),
            },
            {
              key: 'items',
              label: '商品明细',
              children: (
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={detail.items}
                  rowClassName={(r) => (itemIdFocus && r.id === itemIdFocus ? 'ant-table-row-selected' : '')}
                  columns={[
                    { title: '平台商品 ID', dataIndex: 'externalItemId', width: 120, render: (v) => v || '—' },
                    { title: '平台规格编号', dataIndex: 'externalSkuId', width: 120, render: (v) => v || '—' },
                    { title: '标题', dataIndex: 'productTitle', ellipsis: true },
                    { title: '规格', dataIndex: 'skuName', width: 120, render: (v) => v || '—' },
                    { title: '数量', dataIndex: 'quantity', width: 64 },
                    { title: '单价', dataIndex: 'unitPrice', width: 88 },
                    {
                      title: '匹配状态',
                      width: 108,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return tagFromMap(String(m?.matchStatus || ''), ORDER_ITEM_SKU_MATCH_STATUS);
                      },
                    },
                    {
                      title: '本地规格编号',
                      width: 120,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return m?.localSkuCode || row.skuCode || '—';
                      },
                    },
                    {
                      title: '置信度',
                      width: 72,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return m?.confidence ?? '—';
                      },
                    },
                  ]}
                />
              ),
            },
            {
              key: 'sku',
              label: '规格匹配',
              children: (
                <OrderSkuMatchTab
                  orderId={detail.id}
                  onRefreshOrder={load}
                  readOnly={!writable}
                  focusItemId={itemIdFocus}
                />
              ),
            },
            {
              key: 'inv',
              label: '库存影响',
              children: (
                <>
                  {listSummary?.invStatus === 'blocked' ? (
                    <Alert
                      showIcon
                      type="warning"
                      style={{ marginBottom: 12 }}
                      message="SKU 未就绪，暂不能扣减库存"
                      description={
                        <>
                          {INVENTORY_SKU_NOT_BOUND_MESSAGE} {INVENTORY_SKU_AMBIGUOUS_MESSAGE}{' '}
                          <Typography.Link onClick={() => setActiveTab('sku')}>前往规格匹配</Typography.Link>
                        </>
                      }
                    />
                  ) : null}
                  <Space wrap style={{ marginBottom: 12 }}>
                    {detail.inventorySummary ? (
                      <>
                        <Tag color={detail.inventorySummary.hasDeductionSuccess ? 'success' : 'default'}>
                          扣库存{detail.inventorySummary.hasDeductionSuccess ? '：已有成功' : '：尚未成功'}
                        </Tag>
                        <Tag color={detail.inventorySummary.hasRestoreSuccess ? 'processing' : 'default'}>
                          回滚{detail.inventorySummary.hasRestoreSuccess ? '：有过成功' : '：未记录'}
                        </Tag>
                      </>
                    ) : null}
                    <Typography.Link href={`/inventory/deductions?orderId=${encodeURIComponent(detail.id)}`}>
                      扣减记录
                    </Typography.Link>
                    <Typography.Link href={`/inventory/sync-tasks?orderId=${encodeURIComponent(detail.id)}`}>
                      同步任务
                    </Typography.Link>
                    <Typography.Link href={`/orders/exceptions?orderId=${encodeURIComponent(detail.id)}&exceptionType=inventory`}>
                      库存异常
                    </Typography.Link>
                    {writable ? (
                      <>
                        <Popconfirm
                          title="扣减绑定 SKU 的本地库存（幂等，重复执行不会重复扣减）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, { syncInventory: false });
                              message.success(
                                summarizeInvResp('库存扣减', r.inventoryDeduction as Record<string, unknown>),
                              );
                              await load();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" loading={invActionLoading}>
                            手工扣库存
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title="回滚本订单已成功扣减的库存（幂等，仅对已成功扣减且未回补的行生效）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await restoreOrderInventory(detail.id, {
                                syncInventory: false,
                                reason: 'manual_ui',
                              });
                              message.success(
                                summarizeInvResp('库存回滚', r.inventoryRestoration as Record<string, unknown>),
                              );
                              await load();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" danger loading={invActionLoading}>
                            手工回滚库存
                          </Button>
                        </Popconfirm>
                      </>
                    ) : null}
                  </Space>
                  <Table
                    rowKey="id"
                    size="small"
                    dataSource={invRows}
                    pagination={{ pageSize: 10 }}
                    columns={[
                      {
                        title: '类型',
                        dataIndex: 'effectType',
                        width: 100,
                        render: (v) => (v === 'deduct' ? '扣减' : v === 'restore' ? '回滚' : v),
                      },
                      {
                        title: '状态',
                        dataIndex: 'status',
                        width: 88,
                        render: (v) => {
                          const cfg = inventoryTagFromMap(String(v), INVENTORY_DEDUCT_STATUS);
                          return <Tag color={cfg.color}>{cfg.text}</Tag>;
                        },
                      },
                      { title: PRODUCT_COPY.sku, dataIndex: 'skuCode', width: 120, render: (v) => v || '—' },
                      { title: '数量', dataIndex: 'quantity', width: 64 },
                      { title: '扣减前', dataIndex: 'beforeStock', width: 72, render: (v) => v ?? '—' },
                      { title: '扣减后', dataIndex: 'afterStock', width: 72, render: (v) => v ?? '—' },
                      {
                        title: '原因 / 错误',
                        render: (_, r) => r.errorMessage || r.reason || '—',
                        ellipsis: true,
                      },
                      {
                        title: '时间',
                        dataIndex: 'createdAt',
                        width: 156,
                        render: (v) => formatDateTime(v),
                      },
                    ]}
                  />
                </>
              ),
            },
            {
              key: 'shipments',
              label: '物流',
              children: (
                <>
                  {writable ? (
                    <Space style={{ marginBottom: 12 }}>
                      <Button type="primary" onClick={() => openShipModal()}>
                        新增物流
                      </Button>
                      <Typography.Text type="secondary">
                        物流状态为已发货/运输中/已签收时，订单状态会自动向前流转。
                      </Typography.Text>
                    </Space>
                  ) : null}
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={false}
                    dataSource={detail.shipments ?? []}
                    locale={{ emptyText: '暂无物流记录' }}
                    scroll={{ x: 720 }}
                    columns={[
                      { title: '承运商', dataIndex: 'carrier', width: 120 },
                      { title: '运单号', dataIndex: 'trackingNo', width: 160, render: (v) => v || '—' },
                      {
                        title: '状态',
                        dataIndex: 'status',
                        width: 96,
                        render: (v) => tagFromMap(String(v), ORDER_SHIPMENT_STATUS),
                      },
                      {
                        title: '发货时间',
                        dataIndex: 'shippedAt',
                        width: 156,
                        render: (v) => (v ? formatDateTime(v) : '—'),
                      },
                      {
                        title: '签收时间',
                        dataIndex: 'deliveredAt',
                        width: 156,
                        render: (v) => (v ? formatDateTime(v) : '—'),
                      },
                      {
                        title: '追踪',
                        dataIndex: 'trackingUrl',
                        width: 72,
                        render: (u) =>
                          u ? (
                            <a href={u} target="_blank" rel="noopener noreferrer">
                              打开
                            </a>
                          ) : (
                            '—'
                          ),
                      },
                      ...(writable
                        ? [
                            {
                              title: '操作',
                              width: 120,
                              render: (_: unknown, row: OrderShipmentRow) => (
                                <Space>
                                  <Typography.Link onClick={() => openShipModal(row)}>编辑</Typography.Link>
                                  <Popconfirm
                                    title="确认删除该物流记录？"
                                    onConfirm={async () => {
                                      await deleteOrderShipment(detail.id, row.id);
                                      message.success('已删除');
                                      await load();
                                    }}
                                  >
                                    <Typography.Link type="danger">删除</Typography.Link>
                                  </Popconfirm>
                                </Space>
                              ),
                            },
                          ]
                        : []),
                    ]}
                  />
                </>
              ),
            },
            {
              key: 'exceptions',
              label: '异常记录',
              children: (
                <Space direction="vertical">
                  <Typography.Paragraph type="secondary">
                    订单相关异常统一在异常工作台处理；此处提供快捷入口。
                  </Typography.Paragraph>
                  <Button
                    type="primary"
                    onClick={() =>
                      history.push(
                        appendSourceToUrl(
                          `/orders/exceptions?orderId=${encodeURIComponent(id!)}`,
                          'order_detail',
                        ),
                      )
                    }
                  >
                    打开该订单的异常工作台
                  </Button>
                </Space>
              ),
            },
            {
              key: 'tech',
              label: '技术详情',
              children: (
                <TechnicalDetails>
                  <TaskJsonBlock title="订单 ID" value={{ id: detail.id, tenantId: detail.tenantId }} />
                  <TaskJsonBlock title="原始 items 数量" value={{ count: detail.items.length }} last />
                </TechnicalDetails>
              ),
            },
          ]}
        />
      ) : (
        !loading && <Alert type="info" message="未找到订单" description="请从订单列表重新进入，或检查是否有访问权限。" />
      )}

      <Modal
        title={shipModal.row ? '编辑物流' : '新增物流'}
        open={shipModal.open}
        onCancel={() => setShipModal({ open: false })}
        destroyOnHidden
        onOk={async () => {
          const v = await shipForm.validateFields();
          if (!detail) return;
          try {
            if (shipModal.row) {
              await updateOrderShipment(detail.id, shipModal.row.id, v as Record<string, unknown>);
            } else {
              await createOrderShipment(detail.id, v as Record<string, unknown>);
            }
            message.success('已保存');
            setShipModal({ open: false });
            await load();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '保存失败');
          }
        }}
      >
        <Form form={shipForm} layout="vertical">
          <Form.Item name="carrier" label="承运商" rules={[{ required: true, message: '请填写承运商' }]}>
            <Input placeholder="如：云途物流 / J&T" />
          </Form.Item>
          <Form.Item name="trackingNo" label="运单号">
            <Input />
          </Form.Item>
          <Form.Item name="trackingUrl" label="追踪 URL">
            <Input />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]} initialValue="shipped">
            <Select
              options={Object.entries(ORDER_SHIPMENT_STATUS).map(([value, cfg]) => ({
                value,
                label: cfg.text,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

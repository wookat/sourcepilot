import WarehouseSelect from '@/components/inventory/WarehouseSelect';
import FinancePanel from './FinancePanel';
import { PlatformTag, TmPageContainer, TechnicalDetails, TaskJsonBlock } from '@/components/ui';
import { platformLabel } from '@/constants/userFriendly';
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
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
  createOrderItem,
  createOrderShipment,
  deductOrderInventory,
  deleteOrder,
  deleteOrderItem,
  deleteOrderShipment,
  getOrder,
  getOrderInventoryEffects,
  getOrderSKUMatches,
  restoreOrderInventory,
  updateOrder,
  updateOrderItem,
  refreshShipmentTracking,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderItemRow,
  type OrderShipmentRow,
  type OrderSkuMatchRow,
} from '@/services/orders';
import {
  addOrderTags,
  listOrderTags,
  removeOrderTag,
  type OrderTagRow,
} from '@/services/orderTags';
import type { OrderInventoryEffectRow } from '@/services/inventory';
import { recommendForOrders } from '@/services/waybill';
import OrderSkuMatchTab from '@/pages/Orders/SkuMatchTab';
import { localSkuCodeDisplay } from '@/pages/Orders/localSkuDisplay';
import CarrierSelect, { matchCarrier, useEnabledCarriers } from '@/components/CarrierSelect';
import { PRODUCT_COPY } from '@/constants/copywriting';
import {
  INVENTORY_DEDUCT_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  inventoryBindBlockHint,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { canWriteOrders } from '@/utils/orderPerm';
import GenerateResultAlerts from '@/components/procurement/GenerateResultAlerts';
import {
  fetchOrderCostEstimate,
  fetchPurchaseOrders,
  generatePurchaseOrders,
  type GenerateResult,
  type OrderCostEstimate,
  type PurchaseOrder,
} from '@/services/procurement';
import { PO_STATUS_TAG } from '@/pages/Procurement';
import {
  AUTOMATION_ACTION_LABELS,
  AUTOMATION_EVENT_LABELS,
  AUTOMATION_LOG_STATUS_COLORS,
  AUTOMATION_LOG_STATUS_LABELS,
  listOrderAutomationTrail,
  type OrderAutomationLogRow,
} from '@/services/orderAutomation';

function tagFromMap(raw: string, map: Record<string, { text: string; color: string }>) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw || '—'}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

function summarizeInvResp(action: string, sum?: Record<string, unknown>) {
  if (!sum) return `${action}完成`;
  if (sum.skipped) return `已跳过（幂等）：${String(sum.skipReason || '无需重复执行')}`;
  const synced = typeof sum.linesSynced === 'number' ? sum.linesSynced : 0;
  const failed = typeof sum.linesFailed === 'number' ? sum.linesFailed : 0;
  if (synced === 0 && failed === 0 && !sum.error) {
    return `已跳过（幂等）：没有需要${action}的行，未重复执行`;
  }
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
  const { carriers, loading: carriersLoading } = useEnabledCarriers();
  const [refreshingShipmentId, setRefreshingShipmentId] = useState('');
  const [shipModal, setShipModal] = useState<{ open: boolean; row?: OrderShipmentRow | null }>({
    open: false,
  });
  const [shipForm] = Form.useForm();
  const [shipRecommending, setShipRecommending] = useState(false);
  const [itemModal, setItemModal] = useState<{ open: boolean; row?: OrderItemRow | null }>({
    open: false,
  });
  const [itemForm] = Form.useForm();
  const [markPaidLoading, setMarkPaidLoading] = useState(false);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [invActionLoading, setInvActionLoading] = useState(false);
  const [deductWarehouseId, setDeductWarehouseId] = useState<string | undefined>();
  const [costEst, setCostEst] = useState<OrderCostEstimate | null>(null);
  const [relatedPOs, setRelatedPOs] = useState<PurchaseOrder[]>([]);
  const [genLoading, setGenLoading] = useState(false);
  const [genResult, setGenResult] = useState<GenerateResult | null>(null);
  const [allTags, setAllTags] = useState<OrderTagRow[]>([]);
  const [tagUpdating, setTagUpdating] = useState(false);
  const [automationRows, setAutomationRows] = useState<OrderAutomationLogRow[] | null>(null);
  const [automationLoading, setAutomationLoading] = useState(false);
  const [automationError, setAutomationError] = useState('');

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

  const openItemModal = (row?: OrderItemRow) => {
    itemForm.resetFields();
    if (row) {
      itemForm.setFieldsValue({
        productTitle: row.productTitle,
        skuCode: row.skuCode,
        skuName: row.skuName,
        quantity: row.quantity,
        unitPrice: row.unitPrice,
        totalPrice: row.totalPrice,
      });
    }
    setItemModal({ open: true, row: row ?? null });
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
      try {
        const pos = await fetchPurchaseOrders({ page: 1, pageSize: 50, salesOrderId: id });
        setRelatedPOs(pos.items || []);
      } catch {
        setRelatedPOs([]);
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
    else if (tab === 'automation') setActiveTab('automation');
  }, [searchParams]);

  useEffect(() => {
    if (activeTab !== 'automation' || !id || automationRows !== null) return;
    setAutomationLoading(true);
    setAutomationError('');
    listOrderAutomationTrail(id)
      .then((rows) => setAutomationRows(rows))
      .catch((e: unknown) => {
        setAutomationRows([]);
        setAutomationError((e as Error)?.message || '加载自动化轨迹失败');
      })
      .finally(() => setAutomationLoading(false));
  }, [activeTab, id, automationRows]);

  useEffect(() => {
    if (!writable) return;
    void (async () => {
      try {
        setAllTags(await listOrderTags());
      } catch {
        /* 标签加载失败不阻塞详情 */
      }
    })();
  }, [writable]);

  const handleAddTag = async (tagId: string) => {
    if (!detail) return;
    setTagUpdating(true);
    try {
      const tags = await addOrderTags(detail.id, [tagId]);
      setDetail({ ...detail, tags });
    } catch (e) {
      message.error((e as Error)?.message || '添加标签失败');
    } finally {
      setTagUpdating(false);
    }
  };

  const handleRemoveTag = async (tagId: string) => {
    if (!detail) return;
    setTagUpdating(true);
    try {
      const tags = await removeOrderTag(detail.id, tagId);
      setDetail({ ...detail, tags });
    } catch (e) {
      message.error((e as Error)?.message || '移除标签失败');
    } finally {
      setTagUpdating(false);
    }
  };

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
          window.history.back();
          return;
        }
        history.push('/orders/list');
      }}
      extra={
        <Space wrap>
          {writable && detail?.paymentStatus === 'paid' ? (
            <Button
              type="primary"
              loading={genLoading}
              onClick={async () => {
                setGenLoading(true);
                try {
                  const res = await generatePurchaseOrders({ orderIds: [id!] });
                  setGenResult(res);
                  if ((res.orders || []).length > 0) {
                    message.success(`已生成 ${res.orders.length} 张采购单`);
                    await load();
                  } else if (
                    (res.blockers || []).length === 0 &&
                    (res.warnings || []).length === 0
                  ) {
                    message.info('没有可进入采购清单的明细行');
                  }
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '生成采购单失败');
                } finally {
                  setGenLoading(false);
                }
              }}
            >
              生成采购单
            </Button>
          ) : null}
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
          {writable &&
          detail &&
          !['cancelled', 'refunded', 'closed'].includes(detail.status) ? (
            <Popconfirm
              title="取消此订单？"
              description="取消后订单退出待收款/待采购/待发货流程；已扣减的库存按库存策略自动回滚。"
              onConfirm={async () => {
                setCancelLoading(true);
                try {
                  await updateOrder(detail.id, { status: 'cancelled' });
                  message.success('订单已取消');
                  await load();
                } catch (e) {
                  message.error((e as Error)?.message || '取消失败');
                } finally {
                  setCancelLoading(false);
                }
              }}
            >
              <Button danger loading={cancelLoading}>
                取消订单
              </Button>
            </Popconfirm>
          ) : null}
          {writable && detail ? (
            <Popconfirm
              title="软删除此订单？"
              description="删除后订单将从列表隐藏，关联的采购单不受影响。"
              onConfirm={async () => {
                try {
                  await deleteOrder(detail.id);
                  message.success('已删除');
                  history.push('/orders/list');
                } catch (e) {
                  message.error((e as Error)?.message || '删除失败');
                }
              }}
            >
              <Button danger>删除订单</Button>
            </Popconfirm>
          ) : null}
          <Button type="link" onClick={() => window.history.back()}>
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
                        <Descriptions.Item label="平台"><PlatformTag platform={detail.platform} /></Descriptions.Item>
                        <Descriptions.Item label="店铺">
                          {detail.shopSummary?.shopName || '—'}
                          {detail.shopSummary?.platform ? ` (${platformLabel(detail.shopSummary.platform)})` : ''}
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
                        <Descriptions.Item label="标签">
                          <Space size={4} wrap>
                            {(detail.tags || []).map((t) => (
                              <Tag
                                key={t.id}
                                color={t.color === 'default' ? undefined : t.color}
                                closable={writable && !tagUpdating}
                                onClose={(e) => {
                                  e.preventDefault();
                                  void handleRemoveTag(t.id);
                                }}
                              >
                                {t.name}
                              </Tag>
                            ))}
                            {writable ? (
                              <Select
                                size="small"
                                style={{ minWidth: 140 }}
                                placeholder="添加标签"
                                value={null}
                                loading={tagUpdating}
                                disabled={tagUpdating}
                                showSearch
                                optionFilterProp="label"
                                options={allTags
                                  .filter((t) => !(detail.tags || []).some((x) => x.id === t.id))
                                  .map((t) => ({ value: t.id, label: t.name }))}
                                notFoundContent="暂无可添加标签，可在「系统设置 → 订单标签」维护"
                                onSelect={(v) => void handleAddTag(String(v))}
                              />
                            ) : (detail.tags || []).length === 0 ? (
                              '—'
                            ) : null}
                          </Space>
                        </Descriptions.Item>
                        <Descriptions.Item label="金额">
                          {detail.currency} {detail.totalAmount}
                        </Descriptions.Item>
                        <Descriptions.Item label="计划物流商">
                          {detail.plannedCarrierName || detail.plannedCarrierCode ? (
                            <Space size={4} wrap>
                              <span>{detail.plannedCarrierName || detail.plannedCarrierCode}</span>
                              <Tag color={detail.plannedCarrierMode === 'apply' ? 'green' : 'blue'}>
                                {detail.plannedCarrierMode === 'apply' ? '自动应用' : '自动推荐'}
                              </Tag>
                              {detail.plannedCarrierRule ? (
                                <Typography.Text type="secondary">
                                  规则：{detail.plannedCarrierRule}
                                </Typography.Text>
                              ) : null}
                            </Space>
                          ) : (
                            '—'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="分配仓库">
                          {detail.assignedWarehouseName ? (
                            <Space size={4} wrap>
                              <span>{detail.assignedWarehouseName}</span>
                              <Tag color="blue">
                                {detail.assignedWarehouseStrategy === 'stock_first'
                                  ? '库存充足优先'
                                  : '默认仓'}
                              </Tag>
                            </Space>
                          ) : (
                            '—'
                          )}
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
                              message="未配置汇率（系统设置 → 定价 → 默认汇率），成本仅按 CNY 展示，暂无法折算毛利"
                            />
                          ) : null}
                        </>
                      ) : (
                        <Typography.Text type="secondary">暂无可用估算数据</Typography.Text>
                      )}
                    </Card>
                  </Col>
                  <Col span={24}>
                    <Card size="small" title="关联采购单">
                      {relatedPOs.length > 0 ? (
                        <Table<PurchaseOrder>
                          rowKey="id"
                          size="small"
                          dataSource={relatedPOs}
                          pagination={false}
                          scroll={{ x: 640 }}
                          columns={[
                            {
                              title: '采购单',
                              dataIndex: 'id',
                              width: 120,
                              render: (v: string) => (
                                <Typography.Link onClick={() => history.push(`/procurement/orders/${v}`)}>
                                  {v.slice(0, 8)}
                                </Typography.Link>
                              ),
                            },
                            { title: '供应商', dataIndex: 'supplierName', width: 160 },
                            {
                              title: '状态',
                              dataIndex: 'status',
                              width: 120,
                              render: (v: string) => tagFromMap(v, PO_STATUS_TAG),
                            },
                            {
                              title: '金额',
                              dataIndex: 'totalAmount',
                              width: 120,
                              render: (v: number, r) => `${r.currency || 'CNY'} ${v}`,
                            },
                            { title: '1688 订单号', dataIndex: 'externalOrderId', width: 140, render: (v?: string) => v || '—' },
                            {
                              title: '创建时间',
                              dataIndex: 'createdAt',
                              width: 170,
                              render: (v: string) => formatDateTime(v),
                            },
                          ]}
                        />
                      ) : (
                        <Typography.Text type="secondary">
                          该订单暂无关联采购单。
                          {writable && detail.paymentStatus === 'paid'
                            ? '可点击右上角「生成采购单」按主货源聚合生成采购清单。'
                            : '订单标记已付款后可生成采购单。'}
                        </Typography.Text>
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
                <>
                  {writable ? (
                    <Space style={{ marginBottom: 12 }}>
                      <Button type="primary" onClick={() => openItemModal()}>
                        新增明细
                      </Button>
                      <Typography.Text type="secondary">
                        明细变化后请复核订单总额与成本估算；新明细可在「规格匹配」中绑定本地规格。
                      </Typography.Text>
                    </Space>
                  ) : null}
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
                        const display = localSkuCodeDisplay(row, m);
                        if (!display.bound) {
                          return <Typography.Text type="secondary">未绑定</Typography.Text>;
                        }
                        return display.text;
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
                    ...(writable
                      ? [
                          {
                            title: '操作',
                            width: 120,
                            render: (_: unknown, row: OrderItemRow) => (
                              <Space>
                                <Typography.Link onClick={() => openItemModal(row)}>编辑</Typography.Link>
                                <Popconfirm
                                  title="确认删除该商品明细？"
                                  onConfirm={async () => {
                                    try {
                                      await deleteOrderItem(detail.id, row.id);
                                      message.success('已删除');
                                      await load();
                                    } catch (e: unknown) {
                                      message.error((e as Error)?.message || '删除失败');
                                    }
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
                          {inventoryBindBlockHint(listSummary?.skuStatus) ||
                            `${INVENTORY_SKU_NOT_BOUND_MESSAGE} ${INVENTORY_SKU_AMBIGUOUS_MESSAGE}`}{' '}
                          <Typography.Link onClick={() => setActiveTab('sku')}>前往规格匹配</Typography.Link>
                        </>
                      }
                    />
                  ) : null}
                  <Alert
                    showIcon
                    type="info"
                    style={{ marginBottom: 12 }}
                    message="库存口径说明：发货不会自动扣减库存"
                    description="本平台的库存扣减与发货解耦：扣减由本页「手工扣库存」或系统设置中的库存策略（如自动扣减）触发，发货时不会强制扣减；取消订单时已扣减的库存按策略自动回滚。未扣减不影响发货，但建议发货前后及时扣减，保持库存准确。"
                  />
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
                        <WarehouseSelect
                          size="small"
                          style={{ minWidth: 180 }}
                          placeholder="扣减仓库（默认按仓库优先级）"
                          value={deductWarehouseId}
                          onChange={(v) => setDeductWarehouseId(v || undefined)}
                        />
                        <Popconfirm
                          title="扣减绑定 SKU 的本地库存（幂等，重复执行不会重复扣减）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, {
                                syncInventory: false,
                                warehouseId: deductWarehouseId,
                              });
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
                        物流状态为已发货/运输中/已签收时，订单状态会自动向前流转。按手工扣库存策略，发货不会自动扣减库存（属预期行为），可到「库存影响」Tab
                        手工扣减。
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
                      { title: '物流商', dataIndex: 'carrier', width: 120 },
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
                              width: 180,
                              render: (_: unknown, row: OrderShipmentRow) => (
                                <Space>
                                  <Typography.Link onClick={() => openShipModal(row)}>编辑</Typography.Link>
                                  <Typography.Link
                                    disabled={refreshingShipmentId === row.id}
                                    onClick={async () => {
                                      setRefreshingShipmentId(row.id);
                                      try {
                                        const res = await refreshShipmentTracking(detail.id, row.id);
                                        if (res.supported) {
                                          message.success(res.message);
                                          await load();
                                        } else {
                                          message.info(res.message);
                                        }
                                      } catch (e) {
                                        message.error((e as Error)?.message || '刷新轨迹失败');
                                      } finally {
                                        setRefreshingShipmentId('');
                                      }
                                    }}
                                  >
                                    刷新轨迹
                                  </Typography.Link>
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
              key: 'automation',
              label: '自动化轨迹',
              children: (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                    该订单命中的自动化订单规则执行记录（成功/失败/跳过）；全部规则日志见「订单管理 → 自动化执行日志」。
                  </Typography.Paragraph>
                  {automationError ? <Alert type="error" showIcon message={automationError} /> : null}
                  <Table<OrderAutomationLogRow>
                    rowKey="id"
                    size="small"
                    loading={automationLoading}
                    dataSource={automationRows ?? []}
                    pagination={false}
                    locale={{ emptyText: '该订单暂无自动化执行记录' }}
                    columns={[
                      {
                        title: '时间',
                        dataIndex: 'createdAt',
                        width: 170,
                        render: (v: string) => formatDateTime(v),
                      },
                      {
                        title: '触发时机',
                        dataIndex: 'triggerEvent',
                        width: 160,
                        render: (v: string) =>
                          AUTOMATION_EVENT_LABELS[v as keyof typeof AUTOMATION_EVENT_LABELS] || v,
                      },
                      { title: '规则', dataIndex: 'ruleName', width: 180 },
                      {
                        title: '动作',
                        dataIndex: 'action',
                        width: 150,
                        render: (v: string) =>
                          AUTOMATION_ACTION_LABELS[v as keyof typeof AUTOMATION_ACTION_LABELS] || v,
                      },
                      {
                        title: '结果',
                        dataIndex: 'status',
                        width: 90,
                        render: (v: string) => (
                          <Tag color={AUTOMATION_LOG_STATUS_COLORS[v as keyof typeof AUTOMATION_LOG_STATUS_COLORS]}>
                            {AUTOMATION_LOG_STATUS_LABELS[v as keyof typeof AUTOMATION_LOG_STATUS_LABELS] || v}
                          </Tag>
                        ),
                      },
                      { title: '说明', dataIndex: 'reason', ellipsis: true },
                      { title: '尝试次数', dataIndex: 'attempts', width: 90 },
                    ]}
                  />
                  <Button
                    onClick={() =>
                      history.push(
                        appendSourceToUrl(
                          `/orders/automation-logs?keyword=${encodeURIComponent(detail.orderNo || '')}`,
                          'order_detail',
                        ),
                      )
                    }
                  >
                    打开自动化执行日志
                  </Button>
                </Space>
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
              key: 'finance',
              label: '财务对账',
              children: <FinancePanel orderId={detail.id} writable={writable} />,
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
        title="生成采购单结果"
        open={!!genResult && ((genResult.blockers || []).length > 0 || (genResult.warnings || []).length > 0)}
        footer={null}
        onCancel={() => setGenResult(null)}
      >
        <GenerateResultAlerts
          blockers={genResult?.blockers}
          warnings={genResult?.warnings}
          onNavigate={() => setGenResult(null)}
        />
      </Modal>

      <Modal
        title={itemModal.row ? '编辑商品明细' : '新增商品明细'}
        open={itemModal.open}
        onCancel={() => setItemModal({ open: false })}
        forceRender
        onOk={async () => {
          const v = await itemForm.validateFields();
          if (!detail) return;
          try {
            if (itemModal.row) {
              await updateOrderItem(detail.id, itemModal.row.id, v as Record<string, unknown>);
            } else {
              await createOrderItem(detail.id, v as Record<string, unknown>);
            }
            message.success('已保存');
            setItemModal({ open: false });
            await load();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '保存失败');
          }
        }}
      >
        <Form
          form={itemForm}
          layout="vertical"
          onValuesChange={(changed, all) => {
            if ('quantity' in changed || 'unitPrice' in changed) {
              const qty = Number(all.quantity) || 0;
              const price = Number(all.unitPrice) || 0;
              itemForm.setFieldsValue({ totalPrice: Math.round(qty * price * 100) / 100 });
            }
          }}
        >
          <Form.Item name="productTitle" label="商品标题" rules={[{ required: true, message: '请填写商品标题' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="skuCode" label="规格编码">
            <Input />
          </Form.Item>
          <Form.Item name="skuName" label="规格名称">
            <Input />
          </Form.Item>
          <Form.Item name="quantity" label="数量" initialValue={1} rules={[{ required: true, message: '请填写数量' }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="unitPrice" label="单价">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="totalPrice" label="小计" extra="修改数量或单价时自动按 数量 × 单价 重算，可手工覆盖">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={shipModal.row ? '编辑物流' : '新增物流'}
        open={shipModal.open}
        onCancel={() => setShipModal({ open: false })}
        forceRender
        onOk={async () => {
          const v = await shipForm.validateFields();
          if (!detail) return;
          try {
            const matched = matchCarrier(carriers, v.carrier as string);
            const payload = { ...v, carrierCode: matched?.code } as Record<string, unknown>;
            if (shipModal.row) {
              await updateOrderShipment(detail.id, shipModal.row.id, payload);
            } else {
              await createOrderShipment(detail.id, payload);
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
          {!shipModal.row && detail && !detail.inventorySummary?.hasDeductionSuccess ? (
            <Alert
              showIcon
              type="warning"
              style={{ marginBottom: 12 }}
              message="本单尚未扣减库存"
              description="发货不会自动扣减库存；保存后可到「库存影响」Tab 手工扣减，避免库存与实际不符。"
            />
          ) : null}
          <Form.Item
            name="carrier"
            label="物流商"
            rules={[{ required: true, message: '请选择或填写物流商' }]}
            extra={
              <Space direction="vertical" size={0}>
                <span>选择预置物流商可按其规则校验运单号；也可直接输入自定义承运商名称</span>
                <Button
                  size="small"
                  type="link"
                  style={{ padding: 0 }}
                  loading={shipRecommending}
                  onClick={async () => {
                    if (!detail) return;
                    setShipRecommending(true);
                    try {
                      const recs = await recommendForOrders([
                        { key: detail.orderNo, orderId: detail.id },
                      ]);
                      const rec = recs[0];
                      if (rec?.matched && rec.carrierCode) {
                        shipForm.setFieldValue('carrier', rec.carrierCode);
                        message.success(
                          `已按规则推荐：${rec.carrierName || rec.carrierCode}（命中规则：${rec.ruleName}，可手动修改）`,
                        );
                      } else {
                        message.info(rec?.message || '没有命中任何发货规则，可手动选择物流商');
                      }
                    } catch (e) {
                      message.error((e as Error).message || '按规则推荐失败');
                    } finally {
                      setShipRecommending(false);
                    }
                  }}
                >
                  按规则推荐物流商
                </Button>
              </Space>
            }
          >
            <CarrierSelect carriers={carriers} loading={carriersLoading} />
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

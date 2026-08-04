import { ModalForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { DateTimeText, PlatformTag, TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { platformLabel } from '@/constants/userFriendly';
import {
  Badge,
  Alert,
  Button,
  Drawer,
  Form,
  Grid,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { Breakpoint } from 'antd';
import dayjs from 'dayjs';
import { formatDateTime } from '@/utils/formatTime';
import { history, useLocation, useModel } from '@umijs/max';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  ORDER_SYNC_SUMMARY,
} from '@/constants/status';
import {
  createOrder,
  createOrderItem,
  createOrderShipment,
  deductOrderInventory,
  deleteOrder,
  deleteOrderItem,
  deleteOrderShipment,
  downloadOrdersShippingCsv,
  getOrderInventoryEffects,
  getOrder,
  queryOrders,
  restoreOrderInventory,
  updateOrder,
  updateOrderItem,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderItemRow,
  type OrderListRow,
  type OrderShipmentRow,
} from '@/services/orders';
import OrderSkuMatchTab from '@/pages/Orders/SkuMatchTab';
import ImportOrdersModal from '@/pages/Orders/ImportOrdersModal';
import BatchShipModal from '@/pages/Orders/BatchShipModal';
import CarrierSelect, { matchCarrier, useEnabledCarriers } from '@/components/CarrierSelect';
import type { OrderInventoryEffectRow } from '@/services/inventory';
import {
  fetchOrderCostEstimateBatch,
  generatePurchaseOrders,
  type GenerateResult,
  type OrderCostEstimateSummary,
} from '@/services/procurement';
import GenerateResultAlerts from '@/components/procurement/GenerateResultAlerts';
import { chartTokens, tabularNumsStyle } from '@/constants/chartTokens';
import { canWriteOrders } from '@/utils/orderPerm';
import { fetchSettingsList } from '@/services/settings';
import { queryShops } from '@/services/shops';
import { pickGroup } from '@/utils/settingsForm';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { appendSourceToUrl, parsePositiveInt, queryTimeRange } from '@/utils/urlState';

const ORDER_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'payStatus',
  'skuStatus',
  'inventoryStatus',
  'status',
  'fulfillmentStatus',
  'platform',
  'shopId',
  'hasException',
  'hasPurchase',
  'source',
  'start',
  'end',
  'jumpOrder',
] as const;

function truthyInventorySetting(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function summarizeInvResp(sum?: Record<string, unknown>) {
  if (!sum) return '';
  if (sum.skipped) return `跳过：${String(sum.skipReason || '')}`;
  if (typeof sum.message === 'string' && sum.message) return sum.message;
  return '已完成';
}

const TERMINAL_ORDER_STATUSES = ['cancelled', 'refunded', 'closed'];

/** 次要列在 <768px 小屏折叠，只保留订单号 / 状态 / 金额 / 操作；完整信息进订单详情查看。 */
const DESKTOP_ONLY: Breakpoint[] = ['md'];

const ORDER_STATUS_OPTS = Object.keys(ORDER_STATUS).map((v) => ({
  label: ORDER_STATUS[v as keyof typeof ORDER_STATUS].text,
  value: v,
}));
const PAY_OPTS = Object.keys(ORDER_PAYMENT_STATUS).map((v) => ({
  label: ORDER_PAYMENT_STATUS[v as keyof typeof ORDER_PAYMENT_STATUS].text,
  value: v,
}));
const FULL_OPTS = Object.keys(ORDER_FULFILLMENT_STATUS).map((v) => ({
  label: ORDER_FULFILLMENT_STATUS[v as keyof typeof ORDER_FULFILLMENT_STATUS].text,
  value: v,
}));
const SHIP_OPTS = Object.keys(ORDER_SHIPMENT_STATUS).map((v) => ({
  label: ORDER_SHIPMENT_STATUS[v as keyof typeof ORDER_SHIPMENT_STATUS].text,
  value: v,
}));

type StatusTagMap = Record<string, { text: string; color: string }>;

function statusTag(raw: string, map: StatusTagMap) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function OrdersPage() {
  const emptyLocale = useListEmptyLocale('orderList', { permissionScoped: true });
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof ORDER_QUERY_KEYS)[number], string | undefined>>(ORDER_QUERY_KEYS);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detail, setDetail] = useState<OrderDetailDTO | null>(null);
  const [editForm] = Form.useForm();
  const [itemModal, setItemModal] = useState<{ open: boolean; row?: OrderItemRow | null }>({ open: false });
  const [itemForm] = Form.useForm();
  const { carriers, loading: carriersLoading } = useEnabledCarriers();
  const [shipModal, setShipModal] = useState<{ open: boolean; row?: OrderShipmentRow | null }>({ open: false });
  const [shipForm] = Form.useForm();
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);
  const [importOpen, setImportOpen] = useState(false);
  const [batchShipOpen, setBatchShipOpen] = useState(false);
  const { search: ordersSearch } = useLocation();
  const [createInvDefaults, setCreateInvDefaults] = useState<{ deduct: boolean; sync: boolean }>({
    deduct: false,
    sync: false,
  });
  const [invEffectRows, setInvEffectRows] = useState<OrderInventoryEffectRow[]>([]);
  const [costMap, setCostMap] = useState<Record<string, OrderCostEstimateSummary>>({});
  const [invActionLoading, setInvActionLoading] = useState(false);
  const detailIdRef = useRef<string | undefined>();
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = canWriteOrders(initialState?.currentUser?.role);
  const screens = Grid.useBreakpoint();
  const [wideScreen, setWideScreen] = useState(
    () => typeof window === 'undefined' || window.innerWidth >= 768,
  );
  useEffect(() => {
    if (screens.md !== undefined) setWideScreen(screens.md);
  }, [screens.md]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [tableRows, setTableRows] = useState<OrderListRow[]>([]);
  const [batchGenLoading, setBatchGenLoading] = useState(false);
  const [batchPayLoading, setBatchPayLoading] = useState(false);
  const [batchExportLoading, setBatchExportLoading] = useState(false);
  const [batchDeliverLoading, setBatchDeliverLoading] = useState(false);
  const [genResult, setGenResult] = useState<GenerateResult | null>(null);

  const selectedPaidIds = useMemo(
    () =>
      tableRows
        .filter(
          (r) =>
            selectedRowKeys.includes(r.id) &&
            r.paymentStatus === 'paid' &&
            !TERMINAL_ORDER_STATUSES.includes(r.status),
        )
        .map((r) => r.id),
    [tableRows, selectedRowKeys],
  );
  const selectedUnpaidIds = useMemo(
    () =>
      tableRows
        .filter(
          (r) =>
            selectedRowKeys.includes(r.id) &&
            r.paymentStatus === 'unpaid' &&
            !TERMINAL_ORDER_STATUSES.includes(r.status),
        )
        .map((r) => r.id),
    [tableRows, selectedRowKeys],
  );
  const selectedShippedIds = useMemo(
    () =>
      tableRows
        .filter((r) => selectedRowKeys.includes(r.id) && r.status === 'shipped')
        .map((r) => r.id),
    [tableRows, selectedRowKeys],
  );

  const handleBatchMarkPaid = useCallback(async () => {
    const ids = selectedUnpaidIds;
    if (ids.length === 0) return;
    setBatchPayLoading(true);
    const failures: string[] = [];
    let ok = 0;
    try {
      for (const id of ids) {
        try {
          await updateOrder(id, { paymentStatus: 'paid' });
          ok += 1;
        } catch (e: unknown) {
          failures.push((e as Error)?.message || id);
        }
      }
      if (ok > 0) message.success(`已标记 ${ok} 单为已付款`);
      if (failures.length > 0) message.error(`${failures.length} 单标记失败：${failures[0]}`);
      setSelectedRowKeys([]);
      actionRef.current?.reload();
    } finally {
      setBatchPayLoading(false);
    }
  }, [selectedUnpaidIds]);

  const handleBatchExportShipping = useCallback(async () => {
    const ids = selectedPaidIds;
    if (ids.length === 0) return;
    setBatchExportLoading(true);
    try {
      await downloadOrdersShippingCsv(ids);
      message.success(`已导出 ${ids.length} 单发货清单`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '导出失败');
    } finally {
      setBatchExportLoading(false);
    }
  }, [selectedPaidIds]);

  const handleBatchMarkDelivered = useCallback(async () => {
    const ids = selectedShippedIds;
    if (ids.length === 0) return;
    setBatchDeliverLoading(true);
    const failures: string[] = [];
    let ok = 0;
    try {
      for (const id of ids) {
        try {
          await updateOrder(id, {
            status: 'delivered',
            deliveredAt: new Date().toISOString(),
          });
          ok += 1;
        } catch (e: unknown) {
          failures.push((e as Error)?.message || id);
        }
      }
      if (ok > 0) message.success(`已标记 ${ok} 单为已送达`);
      if (failures.length > 0) message.error(`${failures.length} 单标记失败：${failures[0]}`);
      setSelectedRowKeys([]);
      actionRef.current?.reload();
    } finally {
      setBatchDeliverLoading(false);
    }
  }, [selectedShippedIds]);

  const handleBatchGenerate = useCallback(async () => {
    const ids = selectedPaidIds;
    if (ids.length === 0) return;
    setBatchGenLoading(true);
    try {
      const res = await generatePurchaseOrders({ orderIds: ids });
      setGenResult(res);
      if ((res.orders || []).length > 0) {
        message.success(`已生成 ${res.orders.length} 张采购单`);
      } else if ((res.blockers || []).length === 0 && (res.warnings || []).length === 0) {
        message.info('没有可进入采购清单的明细行');
      }
      setSelectedRowKeys([]);
      actionRef.current?.reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '生成采购单失败');
    } finally {
      setBatchGenLoading(false);
    }
  }, [selectedPaidIds]);

  const invEffectFailures = useMemo(
    () => invEffectRows.filter((r) => r.status === 'failed'),
    [invEffectRows],
  );

  useEffect(() => {
    detailIdRef.current = detail?.id;
  }, [detail?.id]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOptions(
          res.list.map((s) => ({
            label: `${s.shopName} (${platformLabel(s.platform)})`,
            value: s.id,
          })),
        );
      } catch {
        /* ignore */
      }
    })();
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const { items } = await fetchSettingsList();
        const g = pickGroup(items, 'inventory');
        setCreateInvDefaults({
          deduct: truthyInventorySetting(g.auto_deduct_manual_orders),
          sync:
            truthyInventorySetting(g.auto_sync_inventory_after_order_deduct) ||
            truthyInventorySetting(g.auto_sync_platform_inventory_after_deduct),
        });
      } catch {
        /* ignore */
      }
    })();
  }, []);

  const refreshDetail = useCallback(async (id?: string) => {
    const oid = id ?? detailIdRef.current;
    if (!oid) return;
    const d = await getOrder(oid);
    setDetail(d);
    editForm.setFieldsValue({
      customerName: d.customerName,
      customerEmail: d.customerEmail,
      customerPhone: d.customerPhone,
      status: d.status,
      paymentStatus: d.paymentStatus,
      fulfillmentStatus: d.fulfillmentStatus,
      currency: d.currency,
      totalAmount: d.totalAmount,
      shopId: d.shopId,
    });
  }, [editForm]);

  const loadInvEffects = useCallback(async (orderId: string) => {
    try {
      const r = await getOrderInventoryEffects(orderId, { page: 1, pageSize: 100 });
      setInvEffectRows(r.list);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载库存影响失败');
    }
  }, []);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      paymentStatus: urlState.payStatus,
      skuMatchStatus: urlState.skuStatus,
      inventoryDeductStatus: urlState.inventoryStatus,
      status: urlState.status,
      fulfillmentStatus: urlState.fulfillmentStatus,
      platform: urlState.platform,
      shopId: urlState.shopId,
      hasException: urlState.hasException,
      hasPurchase: urlState.hasPurchase,
      createdAt: queryTimeRange(urlState.start, urlState.end),
    });
    actionRef.current?.reload();
  }, [
    urlState.fulfillmentStatus,
    urlState.hasException,
    urlState.hasPurchase,
    urlState.inventoryStatus,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.payStatus,
    urlState.platform,
    urlState.shopId,
    urlState.skuStatus,
    urlState.start,
    urlState.end,
    urlState.status,
  ]);

  useEffect(() => {
    const q = new URLSearchParams(ordersSearch);
    const jid = q.get('jumpOrder')?.trim();
    if (!jid) return;
    history.replace(`/orders/${encodeURIComponent(jid)}`);
  }, [ordersSearch]);

  const columns: ProColumns<OrderListRow>[] = useMemo(
    () => [
      {
        title: '关联店铺',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: shopOptions, allowClear: true, showSearch: true },
      },
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '订单号 / 买家 / 平台单号', ...keywordFieldProps },
      },
      { title: '订单号', dataIndex: 'orderNo', copyable: true, ellipsis: true, width: 176 },
      {
        title: '外部单号',
        dataIndex: 'externalOrderId',
        width: 140,
        responsive: DESKTOP_ONLY,
        search: false,
        copyable: true,
        ellipsis: true,
        render: (_, r) => r.externalOrderId || '—',
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 96,
        responsive: DESKTOP_ONLY,
        fieldProps: { allowClear: true },
        render: (_, r) => <PlatformTag platform={r.platform} />,
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        search: false,
        width: 140,
        responsive: DESKTOP_ONLY,
        ellipsis: true,
        render: (_, r) =>
          r.shopName ? (
            <span>
              {r.shopName}
              {r.shopPlatform ? ` / ${platformLabel(r.shopPlatform)}` : ''}
            </span>
          ) : (
            '—'
          ),
      },
      { title: '客户', dataIndex: 'customerName', ellipsis: true, width: 120, responsive: DESKTOP_ONLY },
      {
        title: '订单状态',
        dataIndex: 'status',
        width: 108,
        valueType: 'select',
        valueEnum: ORDER_STATUS,
        render: (_, r) => statusTag(r.status, ORDER_STATUS),
      },
      {
        title: '支付',
        dataIndex: 'paymentStatus',
        width: 94,
        responsive: DESKTOP_ONLY,
        valueType: 'select',
        valueEnum: ORDER_PAYMENT_STATUS,
        render: (_, r) => statusTag(r.paymentStatus, ORDER_PAYMENT_STATUS),
      },
      {
        title: '商品数',
        dataIndex: 'itemCount',
        search: false,
        width: 72,
        responsive: DESKTOP_ONLY,
        render: (_, r) => r.itemCount ?? '—',
      },
      {
        title: '规格匹配',
        dataIndex: 'skuMatchStatus',
        width: 108,
        responsive: DESKTOP_ONLY,
        valueType: 'select',
        valueEnum: ORDER_SKU_MATCH_SUMMARY,
        render: (_, r) => {
          const st = r.skuMatchStatus || 'none';
          const cfg = ORDER_SKU_MATCH_SUMMARY[st as keyof typeof ORDER_SKU_MATCH_SUMMARY];
          const label = cfg?.text || st;
          return (
            <span>
              <Tag color={cfg?.color}>{label}</Tag>
              {r.skuTotalCount ? (
                <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                  {' '}
                  {r.skuMatchedCount ?? 0}/{r.skuTotalCount}
                </Typography.Text>
              ) : null}
            </span>
          );
        },
      },
      {
        title: '库存扣减',
        dataIndex: 'inventoryDeductStatus',
        width: 100,
        responsive: DESKTOP_ONLY,
        valueType: 'select',
        valueEnum: ORDER_INVENTORY_DEDUCT_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.inventoryDeductStatus || 'none';
          const cfg = ORDER_INVENTORY_DEDUCT_SUMMARY[st as keyof typeof ORDER_INVENTORY_DEDUCT_SUMMARY];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: '同步',
        dataIndex: 'syncStatus',
        width: 96,
        responsive: DESKTOP_ONLY,
        valueType: 'select',
        valueEnum: ORDER_SYNC_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.syncStatus || 'unknown';
          const cfg = ORDER_SYNC_SUMMARY[st as keyof typeof ORDER_SYNC_SUMMARY];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: '是否有异常',
        dataIndex: 'hasException',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          true: { text: '有异常' },
          false: { text: '无异常' },
        },
      },
      {
        title: '采购覆盖',
        dataIndex: 'hasPurchase',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          '1': { text: '已生成采购单' },
          '0': { text: '未生成采购单' },
        },
      },
      {
        title: '异常',
        dataIndex: 'openExceptionCount',
        width: 72,
        responsive: DESKTOP_ONLY,
        search: false,
        render: (_, r) =>
          (r.openExceptionCount ?? 0) > 0 ? (
            <Badge count={r.openExceptionCount} size="small">
              <Tag color="error">待处理</Tag>
            </Badge>
          ) : (
            <Tag>无</Tag>
          ),
      },
      {
        title: '履约',
        dataIndex: 'fulfillmentStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: ORDER_FULFILLMENT_STATUS,
      },
      {
        title: '金额',
        search: false,
        width: 120,
        render: (_, r) => `${r.currency} ${r.totalAmount}`,
      },
      {
        title: '预估毛利',
        dataIndex: 'estimatedProfit',
        search: false,
        width: 132,
        responsive: DESKTOP_ONLY,
        render: (_, r) => {
          const est = costMap[r.id];
          if (!est) return '—';
          if (est.missingLines > 0) {
            return (
              <Tooltip title={`${est.missingLines} 行缺参考进价，无法估算成本`}>
                <Tag color="warning">缺价</Tag>
              </Tooltip>
            );
          }
          if (est.exchangeRate == null) {
            return (
              <Tooltip title="未配置汇率，无法折算毛利（定价设置 → 默认汇率）">
                <Tag>未配汇率</Tag>
              </Tooltip>
            );
          }
          if (est.grossProfit == null) return '—';
          const color = est.grossProfit >= 0 ? chartTokens.trendUp : chartTokens.trendDown;
          return (
            <Tooltip
              title={`预估采购成本 CNY ${est.estimatedCostCny.toFixed(2)}，汇率 ${est.exchangeRate}`}
            >
              <span style={{ color, fontWeight: 500, ...tabularNumsStyle }}>
                {r.currency} {est.grossProfit.toFixed(2)}
                {est.marginPercent != null ? (
                  <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                    {' '}
                    {est.marginPercent.toFixed(1)}%
                  </Typography.Text>
                ) : null}
              </span>
            </Tooltip>
          );
        },
      },
      {
        title: '物流',
        dataIndex: 'latestShipmentStatus',
        search: false,
        width: 96,
        responsive: DESKTOP_ONLY,
        render: (_, r) =>
          r.latestShipmentStatus ? statusTag(r.latestShipmentStatus, ORDER_SHIPMENT_STATUS) : '—',
      },
      {
        title: '下单时间',
        dataIndex: 'orderedAt',
        search: false,
        width: 120,
        responsive: DESKTOP_ONLY,
        render: (_, r) => <DateTimeText value={r.orderedAt} />,
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 120,
        responsive: DESKTOP_ONLY,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
        render: (_, r) => <DateTimeText value={r.createdAt} />,
      },
      {
        title: '更新时间',
        dataIndex: 'updatedAt',
        width: 120,
        responsive: DESKTOP_ONLY,
        search: false,
        render: (_, r) => <DateTimeText value={r.updatedAt} />,
      },
      {
        title: '操作',
        valueType: 'option',
        width: wideScreen ? 220 : 128,
        fixed: wideScreen ? 'right' : undefined,
        render: (_, r) => (
          <Space wrap size={4}>
            <a onClick={() => history.push(`/orders/${encodeURIComponent(r.id)}`)}>详情</a>
            {(r.openExceptionCount ?? 0) > 0 ? (
              <a
                onClick={() =>
                  history.push(
                    appendSourceToUrl(
                      `/orders/exceptions?orderId=${encodeURIComponent(r.id)}`,
                      'order_detail',
                    ),
                  )
                }
              >
                异常
              </a>
            ) : null}
            <a onClick={() => history.push(`/orders/sync-tasks?shopId=${encodeURIComponent(r.shopId || '')}`)}>
              同步
            </a>
          </Space>
        ),
      },
    ],
    [shopOptions, keywordFieldProps, costMap, wideScreen],
  );

  const openItemModal = (row?: OrderItemRow) => {
    setItemModal({ open: true, row: row ?? null });
    itemForm.resetFields();
    if (row) itemForm.setFieldsValue(row);
  };

  const openShipModal = (row?: OrderShipmentRow) => {
    setShipModal({ open: true, row: row ?? null });
    shipForm.resetFields();
    if (row) shipForm.setFieldsValue(row);
  };

  const itemColumns = detail
    ? [
        { title: '商品标题', dataIndex: 'productTitle', ellipsis: true },
        { title: '规格编号', dataIndex: 'skuCode', width: 120 },
        { title: '数量', dataIndex: 'quantity', width: 72 },
        { title: '单价', dataIndex: 'unitPrice', width: 88 },
        { title: '小计', dataIndex: 'totalPrice', width: 88 },
        {
          title: '操作',
          key: 'op',
          width: 132,
          render: (_: unknown, row: OrderItemRow) => (
            <Space>
              <a onClick={() => openItemModal(row)}>编辑</a>
              <Popconfirm
                title="删除？"
                onConfirm={async () => {
                  await deleteOrderItem(detail.id, row.id);
                  message.success('已删除');
                  await refreshDetail();
                }}
              >
                <a>删除</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const shipColumns = detail
    ? [
        { title: '承运商', dataIndex: 'carrier', width: 110 },
        { title: '运单号', dataIndex: 'trackingNo', width: 150 },
        {
          title: '状态',
          dataIndex: 'status',
          width: 94,
          render: (v: string) => statusTag(v, ORDER_SHIPMENT_STATUS),
        },
        {
          title: '追踪',
          dataIndex: 'trackingUrl',
          render: (u: string) =>
            u ? (
              <a href={u} target="_blank" rel="noopener noreferrer">
                打开
              </a>
            ) : (
              '—'
            ),
        },
        {
          title: '操作',
          width: 132,
          render: (_: unknown, row: OrderShipmentRow) => (
            <Space>
              <a onClick={() => openShipModal(row)}>编辑</a>
              <Popconfirm
                title="删除？"
                onConfirm={async () => {
                  await deleteOrderShipment(detail.id, row.id);
                  message.success('已删除');
                  await refreshDetail();
                }}
              >
                <a>删除</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const inventoryEffectCols = useMemo(
    () => [
      { title: '规格编号', dataIndex: 'productSkuId', ellipsis: true, width: 120 },
      { title: '类型', dataIndex: 'effectType', width: 100 },
      { title: '状态', dataIndex: 'status', width: 92 },
      { title: '数量', dataIndex: 'quantity', width: 64 },
      {
        title: '原因 / 错误',
        key: 'msg',
        ellipsis: true,
        render: (_: unknown, r: OrderInventoryEffectRow) => r.errorMessage || r.reason || '—',
      },
      {
        title: '时间',
        dataIndex: 'createdAt',
        width: 152,
        render: (v: string) => formatDateTime(v),
      },
    ],
    [],
  );

  return (
    <TmPageContainer title={PAGE_COPY.orderList.title} subTitle={PAGE_COPY.orderList.description}>
      <KeywordSafetyHint visible={showSensitiveHint} />
      <ProTable<OrderListRow>
        key={wideScreen ? 'wide' : 'narrow'}
        rowKey="id"
        locale={emptyLocale}
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ layout: 'vertical' }}
        rowSelection={
          writable
            ? {
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys.map(String)),
                getCheckboxProps: (r) => ({
                  disabled:
                    (r.paymentStatus !== 'paid' && r.paymentStatus !== 'unpaid') ||
                    TERMINAL_ORDER_STATUSES.includes(r.status),
                }),
              }
            : undefined
        }
        tableAlertOptionRender={({ onCleanSelected }) => (
          <Space wrap>
            <Button
              type="primary"
              size="small"
              loading={batchGenLoading}
              disabled={selectedPaidIds.length === 0}
              onClick={() => void handleBatchGenerate()}
            >
              批量生成采购单（{selectedPaidIds.length}）
            </Button>
            <Button
              size="small"
              loading={batchPayLoading}
              disabled={selectedUnpaidIds.length === 0}
              onClick={() => void handleBatchMarkPaid()}
            >
              批量标记已付款（{selectedUnpaidIds.length}）
            </Button>
            <Button
              size="small"
              loading={batchExportLoading}
              disabled={selectedPaidIds.length === 0}
              onClick={() => void handleBatchExportShipping()}
            >
              批量导出发货清单（{selectedPaidIds.length}）
            </Button>
            <Button
              size="small"
              disabled={selectedRowKeys.length === 0}
              onClick={() =>
                history.push(`/orders/print?ids=${encodeURIComponent(selectedRowKeys.join(','))}`)
              }
            >
              打印拣货单（{selectedRowKeys.length}）
            </Button>
            <Button
              size="small"
              loading={batchDeliverLoading}
              disabled={selectedShippedIds.length === 0}
              onClick={() => void handleBatchMarkDelivered()}
            >
              批量标记送达（{selectedShippedIds.length}）
            </Button>
            <a onClick={onCleanSelected}>取消选择</a>
          </Space>
        )}
        onSubmit={() => {
          // URL query 是筛选的唯一来源：提交时把表单值写回 URL，urlState 变化 effect 会触发 reload
          const v = (formRef.current?.getFieldsValue?.() ?? {}) as Record<string, unknown>;
          const range = v.createdAt as [unknown, unknown] | undefined;
          setTablePage(1);
          setUrlState(
            {
              page: undefined,
              keyword: prepareKeyword(v.keyword) || undefined,
              payStatus: (v.paymentStatus as string | undefined)?.trim() || undefined,
              skuStatus: (v.skuMatchStatus as string | undefined)?.trim() || undefined,
              inventoryStatus: (v.inventoryDeductStatus as string | undefined)?.trim() || undefined,
              status: (v.status as string | undefined)?.trim() || undefined,
              fulfillmentStatus: (v.fulfillmentStatus as string | undefined)?.trim() || undefined,
              platform: (v.platform as string | undefined)?.trim() || undefined,
              shopId: (v.shopId as string | undefined)?.trim() || undefined,
              hasException: String(v.hasException ?? '') === 'true' ? 'true' : undefined,
              hasPurchase: ['0', '1'].includes(String(v.hasPurchase ?? '')) ? String(v.hasPurchase) : undefined,
              start: range?.[0] ? dayjs(range[0] as string).toISOString() : undefined,
              end: range?.[1] ? dayjs(range[1] as string).toISOString() : undefined,
            },
            { replace: true },
          );
        }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(ORDER_QUERY_KEYS, { replace: true });
        }}
        toolBarRender={() => (writable ? [
          <ModalForm
            key={`c-${createInvDefaults.deduct}-${createInvDefaults.sync}`}
            initialValues={{
              deductInventory: createInvDefaults.deduct,
              syncInventory: createInvDefaults.sync,
            }}
            title="新建手工订单"
            trigger={<Button type="primary">新建订单</Button>}
            onFinish={async (vals) => {
              await createOrder(vals as Record<string, unknown>);
              message.success('已创建');
              actionRef.current?.reload();
              return true;
            }}
          >
            <ProFormText name="platform" label="平台" placeholder="manual" extra="手工订单可填 manual 或留空" />
            <Alert
              showIcon
              type="info"
              style={{ marginBottom: 12 }}
              message="商品明细可在创建后进入订单详情「商品明细」Tab 添加；成本/毛利估算依赖明细行。"
            />
            <ProFormSelect
              name="shopId"
              label="关联店铺（可选）"
              options={shopOptions}
              fieldProps={{ allowClear: true, showSearch: true }}
            />
            <ProFormText name="orderNo" label="订单号" rules={[{ required: true }]} />
            <ProFormText name="customerName" label="客户名称" rules={[{ required: true }]} />
            <ProFormText name="customerEmail" label="邮箱" />
            <ProFormText name="customerPhone" label="电话" />
            <ProFormSelect name="status" label="订单状态" options={ORDER_STATUS_OPTS} initialValue="pending" />
            <ProFormSelect name="paymentStatus" label="支付状态" options={PAY_OPTS} initialValue="unpaid" />
            <ProFormSelect name="fulfillmentStatus" label="履约状态" options={FULL_OPTS} initialValue="unfulfilled" />
            <ProFormText name="currency" label="币种" initialValue="USD" />
            <ProFormDigit name="totalAmount" label="订单总额" min={0} fieldProps={{ precision: 2 }} initialValue={0} />
            <ProFormSwitch
              name="deductInventory"
              label="创建后扣减本地库存"
              tooltip="与「设置 → 库存 / 订单 → 手工订单默认扣库存」并联"
            />
            <ProFormSwitch
              name="syncInventory"
              label="扣减后触发平台出库同步队列"
              tooltip="需在策略中放行并具备刊登出库路由"
            />
          </ModalForm>,
          <Button key="import" onClick={() => setImportOpen(true)}>
            批量导入订单
          </Button>,
          <Button key="batch-ship" onClick={() => setBatchShipOpen(true)}>
            批量发货
          </Button>,
        ] : [])}
        request={async () => {
          // 筛选条件一律以 URL query 为准（单一来源）；表单提交通过 onSubmit 写回 URL 后再触发查询
          const qp = {
            page: parsePositiveInt(urlState.page, 1),
            pageSize: parsePositiveInt(urlState.pageSize, 20),
            platform: urlState.platform?.trim(),
            shopId: urlState.shopId?.trim(),
            keyword: prepareKeyword(urlState.keyword),
            paymentStatus: urlState.payStatus?.trim(),
            skuMatchStatus: urlState.skuStatus?.trim(),
            inventoryDeductStatus: urlState.inventoryStatus?.trim(),
            status: urlState.status?.trim(),
            fulfillmentStatus: urlState.fulfillmentStatus?.trim(),
            hasException: urlState.hasException === 'true' ? true : undefined,
            hasPurchase: ['0', '1'].includes(String(urlState.hasPurchase ?? ''))
              ? (urlState.hasPurchase as '0' | '1')
              : undefined,
            start: urlState.start,
            end: urlState.end,
          };
          const res = await queryOrders({
            page: qp.page,
            pageSize: qp.pageSize,
            platform: qp.platform,
            shopId: qp.shopId,
            keyword: qp.keyword,
            status: qp.status,
            paymentStatus: qp.paymentStatus,
            fulfillmentStatus: qp.fulfillmentStatus,
            skuMatchStatus: qp.skuMatchStatus,
            inventoryDeductStatus: qp.inventoryDeductStatus,
            hasException: qp.hasException,
            hasPurchase: qp.hasPurchase,
            start: qp.start,
            end: qp.end,
          });
          setTableRows(res.list);
          const ids = res.list.map((r) => r.id).filter(Boolean);
          if (ids.length > 0) {
            void fetchOrderCostEstimateBatch(ids.slice(0, 50))
              .then((out) => setCostMap((prev) => ({ ...prev, ...(out?.items ?? {}) })))
              .catch(() => {
                /* 估算失败不阻塞列表 */
              });
          }
          return { data: res.list, total: res.pagination.total, success: true };
        }}
        pagination={{
          current: tablePage,
          pageSize: tablePageSize,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            setTablePage(page);
            setTablePageSize(pageSize);
            setUrlState({
              page: page > 1 ? page : undefined,
              pageSize: pageSize !== 20 ? pageSize : undefined,
            });
          },
        }}
      />

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

      <Drawer
        title={detail ? `订单 ${detail.orderNo}` : '订单详情'}
        width={720}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDetail(null);
          setInvEffectRows([]);
        }}
        destroyOnHidden
      >
        {detail && (
          <>
            <Space wrap style={{ marginBottom: 12 }}>
              <Badge status="processing" text={`平台 ${platformLabel(detail.platform)}`} />
              {detail.shopSummary ? (
                <Badge
                  status="default"
                  text={`店铺 ${detail.shopSummary.shopName} (${platformLabel(detail.shopSummary.platform)})`}
                />
              ) : null}
              <Popconfirm
                title="软删除此订单？"
                onConfirm={async () => {
                  await deleteOrder(detail.id);
                  message.success('已删除');
                  setDrawerOpen(false);
                  actionRef.current?.reload();
                }}
              >
                <Button danger size="small">
                  删除
                </Button>
              </Popconfirm>
            </Space>
            <Tabs
              onChange={(k) => {
                if (k === 'inv') void loadInvEffects(detail.id);
              }}
              items={[
                {
                  key: 'b',
                  label: '基础',
                  children: (
                    <Form
                      layout="vertical"
                      form={editForm}
                      onFinish={async (v) => {
                        const payload: Record<string, unknown> = {
                          customerName: v.customerName,
                          customerEmail: v.customerEmail ?? undefined,
                          customerPhone: v.customerPhone ?? undefined,
                          status: v.status,
                          paymentStatus: v.paymentStatus,
                          fulfillmentStatus: v.fulfillmentStatus,
                          currency: v.currency,
                          totalAmount: v.totalAmount,
                        };
                        const sid = v.shopId as string | undefined;
                        if (sid === undefined || sid === null || sid === '') {
                          payload.setShopIdNil = true;
                        } else {
                          payload.shopId = sid;
                        }
                        await updateOrder(detail.id, payload);
                        message.success('已保存');
                        await refreshDetail();
                        actionRef.current?.reload();
                      }}
                    >
                      <Form.Item name="customerName" label="客户名称" rules={[{ required: true }]}>
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerEmail" label="邮箱">
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerPhone" label="电话">
                        <Input />
                      </Form.Item>
                      <Form.Item name="shopId" label="关联店铺">
                        <Select
                          allowClear
                          showSearch
                          optionFilterProp="label"
                          placeholder="可选"
                          options={shopOptions}
                        />
                      </Form.Item>
                      <Form.Item name="status" label="订单状态" rules={[{ required: true }]}>
                        <Select options={ORDER_STATUS_OPTS} />
                      </Form.Item>
                      <Form.Item name="paymentStatus" label="支付" rules={[{ required: true }]}>
                        <Select options={PAY_OPTS} />
                      </Form.Item>
                      <Form.Item name="fulfillmentStatus" label="履约" rules={[{ required: true }]}>
                        <Select options={FULL_OPTS} />
                      </Form.Item>
                      <Form.Item name="currency" label="币种" rules={[{ required: true }]}>
                        <Input style={{ width: 120 }} />
                      </Form.Item>
                      <Form.Item name="totalAmount" label="总额" rules={[{ required: true }]}>
                        <InputNumber style={{ width: '100%' }} min={0} />
                      </Form.Item>
                      <Button type="primary" htmlType="submit">
                        保存
                      </Button>
                    </Form>
                  ),
                },
                {
                  key: 'i',
                  label: '商品明细',
                  children: (
                    <>
                      <Button type="primary" style={{ marginBottom: 8 }} onClick={() => openItemModal()}>
                        添加明细
                      </Button>
                      <Table<OrderItemRow> rowKey="id" columns={itemColumns as never} dataSource={detail.items} pagination={false} />
                    </>
                  ),
                },
                {
                  key: 's',
                  label: '物流',
                  children: (
                    <>
                      <Button type="primary" style={{ marginBottom: 8 }} onClick={() => openShipModal()}>
                        添加物流
                      </Button>
                      <Table<OrderShipmentRow> rowKey="id" columns={shipColumns as never} dataSource={detail.shipments} pagination={false} />
                    </>
                  ),
                },
                {
                  key: 'inv',
                  label: '库存',
                  children: (
                    <>
                      {detail && invEffectFailures.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          style={{ marginBottom: 12 }}
                          message="存在失败的库存扣减或恢复记录"
                          description={
                            <span>
                              请在异常工作台查看是否需要重新绑定 SKU、补库存或重试扣减。{' '}
                              <Typography.Link
                                onClick={() =>
                                  history.push(
                                    `/orders/exceptions?orderId=${encodeURIComponent(detail.id)}`,
                                  )
                                }
                              >
                                打开异常工作台
                              </Typography.Link>
                            </span>
                          }
                        />
                      ) : null}
                      <Space wrap style={{ marginBottom: 12 }}>
                        {detail.inventorySummary ? (
                          <>
                            <Tag color={detail.inventorySummary.hasDeductionSuccess ? 'success' : 'default'}>
                              扣库存{detail.inventorySummary.hasDeductionSuccess ? '：已有成功记录' : '：尚未成功'}
                            </Tag>
                            <Tag color={detail.inventorySummary.hasRestoreSuccess ? 'processing' : 'default'}>
                              回滚{detail.inventorySummary.hasRestoreSuccess ? '：有过成功记录' : '：未记录'}
                            </Tag>
                          </>
                        ) : (
                          <Tag>库存摘要不可用</Tag>
                        )}
                        <Popconfirm
                          title="扣减绑定 SKU 的本地库存（幂等；见错误提示）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, { syncInventory: false });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryDeduction as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
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
                          title="扣库存并触发平台出库同步队列（仍需刊登与 outbound 就绪）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, { syncInventory: true });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryDeduction as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" loading={invActionLoading}>
                            扣库存 + 推平台任务
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title='回滚本订单已成功扣掉的库存（需尚未被标记为「已完全对冲」等特殊状态）'
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await restoreOrderInventory(detail.id, {
                                syncInventory: false,
                                reason: 'manual_ui',
                              });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryRestoration as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
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
                      </Space>
                      <Space wrap style={{ marginBottom: 8 }}>
                        <Typography.Link href={`/inventory/effects?orderId=${encodeURIComponent(detail.id)}`}>
                          全局影响流水
                        </Typography.Link>
                        <Typography.Link href={`/inventory/logs?orderId=${encodeURIComponent(detail.id)}`}>
                          全局库存变更
                        </Typography.Link>
                      </Space>
                      <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
                        策略见「设置 → 库存 / 订单」。平台同步失败不参与本地数据库事务。
                      </Typography.Paragraph>
                      <Table<OrderInventoryEffectRow>
                        rowKey="id"
                        size="small"
                        columns={inventoryEffectCols as never}
                        dataSource={invEffectRows}
                        pagination={{ pageSize: 8 }}
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
                      onRefreshOrder={async () => {
                        await refreshDetail();
                        await loadInvEffects(detail.id);
                      }}
                    />
                  ),
                },
              ]}
            />
          </>
        )}
      </Drawer>

      <Modal
        title={itemModal.row ? '编辑明细' : '新增明细'}
        open={itemModal.open}
        onCancel={() => setItemModal({ open: false })}
        forceRender
        onOk={async () => {
          const v = await itemForm.validateFields();
          if (!detail) return;
          if (itemModal.row) await updateOrderItem(detail.id, itemModal.row.id, v as Record<string, unknown>);
          else await createOrderItem(detail.id, v as Record<string, unknown>);
          message.success('已保存');
          setItemModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={itemForm} layout="vertical">
          <Form.Item name="productTitle" label="标题" rules={[{ required: true, message: '必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="skuCode" label="规格编码">
            <Input />
          </Form.Item>
          <Form.Item name="skuName" label="规格名称">
            <Input />
          </Form.Item>
          <Form.Item name="quantity" label="数量" initialValue={1} rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="unitPrice" label="单价">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="totalPrice" label="小计">
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
          const matched = matchCarrier(carriers, v.carrier as string);
          const payload = { ...v, carrierCode: matched?.code } as Record<string, unknown>;
          if (shipModal.row) await updateOrderShipment(detail.id, shipModal.row.id, payload);
          else await createOrderShipment(detail.id, payload);
          message.success('已保存');
          setShipModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={shipForm} layout="vertical">
          <Form.Item name="carrier" label="物流商" rules={[{ required: true, message: '请选择或填写物流商' }]}>
            <CarrierSelect carriers={carriers} loading={carriersLoading} />
          </Form.Item>
          <Form.Item name="trackingNo" label="运单号">
            <Input />
          </Form.Item>
          <Form.Item name="trackingUrl" label="追踪 URL">
            <Input />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]} initialValue="pending">
            <Select options={SHIP_OPTS} />
          </Form.Item>
        </Form>
      </Modal>
      <ImportOrdersModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onDone={() => actionRef.current?.reload()}
        shopOptions={shopOptions}
      />
      <BatchShipModal
        open={batchShipOpen}
        onClose={() => setBatchShipOpen(false)}
        onDone={() => actionRef.current?.reload()}
      />
    </TmPageContainer>
  );
}

import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { Tag, Typography } from 'antd';
import { INVENTORY_CHANGE_REASON, INVENTORY_CHANGE_TYPE } from '@/constants/inventoryLabels';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useMemo, useRef } from 'react';
import { Link } from '@umijs/max';
import { useLocation } from '@umijs/renderer-react';
import type { InventoryChangeLogRow } from '@/services/inventory';
import { queryGlobalInventoryLogs } from '@/services/inventory';
import WarehouseSelect from '@/components/inventory/WarehouseSelect';

function renderDelta(delta: number) {
  const color = delta > 0 ? 'green' : delta < 0 ? 'red' : 'default';
  const prefix = delta > 0 ? '+' : '';
  return <Tag color={color}>{`${prefix}${delta}`}</Tag>;
}

export default function InventoryLogsPage() {
  const actionRef = useRef<ActionType>();
  const { search } = useLocation();
  const emptyLocale = useListEmptyLocale('inventoryLogs', { permissionScoped: true });

  const initialSearch = useMemo(() => {
    const q = new URLSearchParams(search);
    const productId = (q.get('productId') || '').trim();
    const productSkuId = (q.get('productSkuId') || '').trim();
    const orderId = (q.get('orderId') || '').trim();
    const out: Record<string, string> = {};
    if (productId) out.productId = productId;
    if (productSkuId) out.productSkuId = productSkuId;
    if (orderId) out.orderId = orderId;
    return out;
  }, [search]);

  const columns: ProColumns<InventoryChangeLogRow>[] = useMemo(
    () => [
      {
        title: '时间范围',
        dataIndex: 'timeRange',
        hideInTable: true,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      { title: '商品 ID', dataIndex: 'productId', hideInTable: true },
      { title: '规格编号', dataIndex: 'productSkuId', hideInTable: true },
      { title: '订单 ID', dataIndex: 'orderId', hideInTable: true },
      {
        title: '仓库',
        dataIndex: 'warehouseId',
        hideInTable: true,
        renderFormItem: () => <WarehouseSelect includeAll includeDisabled placeholder="全部仓库" />,
      },
      {
        title: '仓库',
        dataIndex: 'warehouseName',
        width: 100,
        ellipsis: true,
        search: false,
        render: (_, r) => r.warehouseName || '默认仓',
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 180,
        ellipsis: true,
        search: false,
        render: (_, r) =>
          r.productId ? (
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
          ) : (
            r.productTitle || '—'
          ),
      },
      {
        title: '商品规格',
        dataIndex: 'skuCode',
        width: 140,
        ellipsis: true,
        search: false,
        render: (_, r) => (
          <span>
            {r.skuCode || '—'}
            {r.skuName ? (
              <Typography.Text type="secondary" style={{ display: 'block', fontSize: 12 }}>
                {r.skuName}
              </Typography.Text>
            ) : null}
          </span>
        ),
      },
      {
        title: '变更类型',
        dataIndex: 'changeType',
        width: 132,
        ellipsis: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(INVENTORY_CHANGE_TYPE).map(([k, v]) => [k, { text: v }]),
        ),
        render: (_, r) => INVENTORY_CHANGE_TYPE[r.changeType] ?? r.changeType,
      },
      {
        title: '变更前',
        dataIndex: 'beforeStock',
        width: 88,
        search: false,
      },
      {
        title: '变更后',
        dataIndex: 'afterStock',
        width: 88,
        search: false,
      },
      {
        title: '变化',
        dataIndex: 'delta',
        width: 88,
        search: false,
        render: (_, r) => renderDelta(r.delta),
      },
      {
        title: '原因',
        dataIndex: 'reason',
        width: 140,
        ellipsis: true,
        search: false,
        render: (_, r) => (r.reason ? INVENTORY_CHANGE_REASON[r.reason] ?? r.reason : '—'),
      },
      {
        title: '备注',
        dataIndex: 'remark',
        ellipsis: true,
        search: false,
        render: (_, r) => r.remark || '—',
      },
      {
        title: '关联订单',
        dataIndex: 'refOrderId',
        width: 160,
        ellipsis: true,
        search: false,
        render: (_, r) =>
          r.refOrderId ? (
            <Link to={`/orders/${r.refOrderId}?tab=inventory`}>
              {r.refOrderNo || r.refOrderId.slice(0, 8)}
            </Link>
          ) : (
            '—'
          ),
      },
      {
        title: '关联订单行',
        dataIndex: 'refOrderItemId',
        width: 140,
        ellipsis: true,
        copyable: true,
        search: false,
        render: (_, r) => r.refOrderItemId || '—',
      },
    ],
    [],
  );

  return (
    <TmPageContainer title="库存流水" subTitle="查看本地库存变更记录，便于核对扣减、恢复与手动调整。">
      <ProTable<InventoryChangeLogRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1400 }}
        form={{ initialValues: initialSearch }}
        search={{}}
        pagination={{ pageSize: 20 }}
        locale={emptyLocale}
        request={async (params) => {
          const res = await queryGlobalInventoryLogs({
            page: params.current,
            pageSize: params.pageSize,
            productId: (params.productId as string)?.trim() || undefined,
            productSkuId: (params.productSkuId as string)?.trim() || undefined,
            orderId: (params.orderId as string)?.trim() || undefined,
            changeType: (params.changeType as string)?.trim() || undefined,
            warehouseId: (params.warehouseId as string)?.trim() || undefined,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />
    </TmPageContainer>
  );
}

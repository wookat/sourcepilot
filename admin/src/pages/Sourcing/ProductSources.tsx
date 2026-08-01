import { TmPageContainer } from '@/components/ui';
import {
  fetchProductDetail,
  fetchProducts,
  type ProductDetail,
  type ProductListRow,
} from '@/services/products';
import {
  adoptSwitchSuggestion,
  bindProductSource,
  deleteSkuMapping,
  fetchPriceHistory,
  fetchProductSources,
  fetchSourceAlerts,
  fetchSwitchEvents,
  ignoreSwitchSuggestion,
  refreshProductSources,
  saveSkuMappings,
  setPrimarySource,
  updateProductSource,
  type ProductSource,
  type ProductSourceSKU,
  type RefreshAlert,
  type SourceAlertRow,
  type SourcePriceHistoryRow,
  type SourceSwitchEvent,
} from '@/services/sourcing';
import {
  Alert,
  Button,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { isReadonly } from '@/utils/permission';
import { useLocation, useModel } from '@umijs/max';
import { useCallback, useEffect, useMemo, useState } from 'react';

const SOURCE_STATUS_TAG: Record<string, { text: string; color: string }> = {
  active: { text: '正常', color: 'green' },
  out_of_stock: { text: '断货', color: 'red' },
  price_alert: { text: '涨价预警', color: 'orange' },
  disabled: { text: '停用', color: 'default' },
};

const SWITCH_REASON: Record<string, string> = {
  out_of_stock: '断货切换',
  price_increase: '涨价切换',
  manual: '人工切换',
};

const SWITCH_MODE: Record<string, string> = {
  auto: '自动',
  manual: '人工',
  suggested: '建议',
};

function renderRefreshAlert(a: RefreshAlert): string {
  const supplier = a.supplierName || '未知供应商';
  const reason = SWITCH_REASON[a.reason || ''] || '';
  switch (a.code) {
    case 'fetch_failed':
      return `货源「${supplier}」报价拉取失败，请稍后重试`;
    case 'price_increase':
      return `货源「${supplier}」进货价涨幅超过 ${a.thresholdPercent ?? ''}%，已标记涨价预警`;
    case 'primary_locked':
      return `主供应商「${supplier}」${a.reason === 'out_of_stock' ? '断货' : '涨价预警'}，但已锁定，需人工处理`;
    case 'no_backup':
      return `主供应商「${supplier}」${a.reason === 'out_of_stock' ? '断货' : '涨价预警'}，且无可用备选货源`;
    case 'switch_suggested':
      return `建议将主供应商切换到「${supplier}」（${reason || '建议切换'}），可在下方切换审计中采纳或忽略`;
    case 'auto_switched':
      return `主供应商已自动切换到「${supplier}」（${reason || '自动切换'}）`;
    default:
      return `货源「${supplier}」出现预警（${a.code}）`;
  }
}

const SUGGESTION_STATUS_TAG: Record<string, { text: string; color: string }> = {
  open: { text: '待处理', color: 'orange' },
  adopted: { text: '已采纳', color: 'green' },
  ignored: { text: '已忽略', color: 'default' },
};

export default function ProductSourcesPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: { role?: string } };
  };
  const writable = !isReadonly(initialState?.currentUser?.role);
  const location = useLocation();
  const initialProductId = useMemo(() => {
    const v = new URLSearchParams(location.search).get('productId')?.trim();
    return v || undefined;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [productId, setProductId] = useState<string | undefined>(initialProductId);
  const [productDetail, setProductDetail] = useState<ProductDetail | null>(null);
  const [sources, setSources] = useState<ProductSource[]>([]);
  const [events, setEvents] = useState<SourceSwitchEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [alerts, setAlerts] = useState<RefreshAlert[]>([]);

  const [bindOpen, setBindOpen] = useState(false);
  const [bindForm] = Form.useForm();

  const [mappingSource, setMappingSource] = useState<ProductSource | null>(null);
  const [mappingRows, setMappingRows] = useState<
    {
      localSkuId: string;
      skuName: string;
      mappingId?: string;
      externalSkuId?: string;
      currentPrice?: number;
      currentStock?: number;
    }[]
  >([]);

  const [historySku, setHistorySku] = useState<ProductSourceSKU | null>(null);
  const [historyRows, setHistoryRows] = useState<SourcePriceHistoryRow[]>([]);
  const [alertRows, setAlertRows] = useState<SourceAlertRow[]>([]);
  const [alertsLoading, setAlertsLoading] = useState(false);

  useEffect(() => {
    fetchProducts({ page: 1, pageSize: 100 })
      .then((res) => setProducts(res.list || []))
      .catch(() => message.error('加载商品列表失败'));
  }, []);

  const load = useCallback(async () => {
    if (!productId) return;
    setLoading(true);
    try {
      const [src, ev, detail] = await Promise.all([
        fetchProductSources(productId),
        fetchSwitchEvents({ productId, page: 1, pageSize: 20 }),
        fetchProductDetail(productId),
      ]);
      setSources(src.items || []);
      setEvents(ev.items || []);
      setProductDetail(detail);
    } catch (e) {
      message.error((e as Error).message || '加载货源失败');
    } finally {
      setLoading(false);
    }
  }, [productId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (productId) return;
    setAlertsLoading(true);
    fetchSourceAlerts()
      .then((res) => setAlertRows(res.items || []))
      .catch(() => message.error('加载预警货源失败'))
      .finally(() => setAlertsLoading(false));
  }, [productId]);

  const localSkus = useMemo(() => productDetail?.skus || [], [productDetail]);

  const sourceLabel = useCallback(
    (id: string) => {
      const s = sources.find((x) => x.id === id);
      return s?.supplier?.name || id.slice(0, 8);
    },
    [sources],
  );

  const openMapping = (src: ProductSource) => {
    setMappingSource(src);
    const existing = new Map((src.skus || []).map((m) => [m.localSkuId, m]));
    setMappingRows(
      localSkus.map((sku) => {
        const m = existing.get(sku.id);
        return {
          localSkuId: sku.id,
          skuName: sku.skuName || sku.skuCode,
          mappingId: m?.id,
          externalSkuId: m?.externalSkuId,
          currentPrice: m?.currentPrice,
          currentStock: m?.currentStock,
        };
      }),
    );
  };

  const submitMappings = async () => {
    if (!mappingSource) return;
    const rows = mappingRows.filter((r) => r.externalSkuId || r.currentPrice !== undefined);
    if (rows.length === 0) {
      message.warning('请至少填写一行外部SKU或参考价');
      return;
    }
    try {
      await saveSkuMappings(mappingSource.id, rows);
      message.success('SKU 映射已保存');
      setMappingSource(null);
      void load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    }
  };

  const openHistory = async (sku: ProductSourceSKU) => {
    setHistorySku(sku);
    try {
      const res = await fetchPriceHistory(sku.id, 90);
      setHistoryRows(res.items || []);
    } catch (e) {
      message.error((e as Error).message || '加载历史进价失败');
    }
  };

  return (
    <TmPageContainer title="商品货源档案" subTitle="一品多源：主供应商、备用供应商与 SKU 映射">
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          showSearch
          placeholder="选择商品"
          style={{ width: 360 }}
          value={productId}
          optionFilterProp="label"
          onChange={(v) => setProductId(v)}
          options={products.map((p) => ({ value: p.id, label: p.title }))}
        />
        {writable && (
        <Button type="primary" disabled={!productId} onClick={() => setBindOpen(true)}>
          绑定货源
        </Button>
        )}
        {writable && (
        <Button
          disabled={!productId || sources.length === 0}
          loading={refreshing}
          onClick={async () => {
            if (!productId) return;
            setRefreshing(true);
            try {
              const res = await refreshProductSources(productId);
              setAlerts(res.alerts || []);
              message.success(`已刷新 ${res.refreshed} 个规格的报价（模拟报价服务）`);
              void load();
            } catch (e) {
              message.error((e as Error).message || '刷新失败');
            } finally {
              setRefreshing(false);
            }
          }}
        >
          刷新价格/库存（mock）
        </Button>
        )}
      </Space>
      {alerts.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="切换规则提示"
          description={alerts.map((a, i) => (
            <div key={i}>{renderRefreshAlert(a)}</div>
          ))}
          closable
          onClose={() => setAlerts([])}
        />
      )}
      {!productId ? (
        alertRows.length === 0 && !alertsLoading ? (
          <Empty description="暂无涨价/断货预警，请选择商品查看货源档案" />
        ) : (
          <>
            <Typography.Title level={5}>预警货源（涨价 / 断货）</Typography.Title>
            <Table<SourceAlertRow>
              rowKey="sourceId"
              size="small"
              loading={alertsLoading}
              dataSource={alertRows}
              pagination={false}
              scroll={{ x: 800 }}
              columns={[
                { title: '商品', dataIndex: 'productTitle', ellipsis: true },
                { title: '供应商', dataIndex: 'supplierName', width: 160, render: (v) => v || '-' },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 110,
                  render: (v: string) => {
                    const cfg = SOURCE_STATUS_TAG[v] || { text: v, color: 'default' };
                    return <Tag color={cfg.color}>{cfg.text}</Tag>;
                  },
                },
                {
                  title: '主供应商',
                  dataIndex: 'isPrimary',
                  width: 100,
                  render: (v: boolean) => (v ? <Tag color="blue">主</Tag> : '-'),
                },
                {
                  title: '待处理建议',
                  dataIndex: 'openSuggestions',
                  width: 110,
                  render: (v: number) => (v > 0 ? <Tag color="orange">{v}</Tag> : '-'),
                },
                { title: '最近检查', dataIndex: 'lastCheckedAt', width: 180, render: (v) => v || '-' },
                {
                  title: '操作',
                  width: 110,
                  render: (_, row) => (
                    <a onClick={() => setProductId(row.productId)}>查看档案</a>
                  ),
                },
              ]}
            />
          </>
        )
      ) : (
        <>
          <Table<ProductSource>
            rowKey="id"
            loading={loading}
            dataSource={sources}
            pagination={false}
            scroll={{ x: 1100 }}
            expandable={{
              expandedRowRender: (src) => (
                <Table<ProductSourceSKU>
                  rowKey="id"
                  size="small"
                  dataSource={src.skus || []}
                  pagination={false}
                  columns={[
                    {
                      title: '本地规格',
                      dataIndex: 'localSkuId',
                      render: (v: string) =>
                        localSkus.find((s) => s.id === v)?.skuName || v.slice(0, 8),
                    },
                    { title: '货源规格', dataIndex: 'externalSkuId', render: (v) => v || '-' },
                    {
                      title: '当前进价',
                      dataIndex: 'currentPrice',
                      render: (v, row) => (v !== undefined && v !== null ? `${v} ${row.currency}` : '-'),
                    },
                    { title: '库存', dataIndex: 'currentStock', render: (v) => v ?? '-' },
                    {
                      title: '操作',
                      render: (_, row) => <a onClick={() => void openHistory(row)}>历史进价</a>,
                    },
                  ]}
                />
              ),
            }}
            columns={[
              {
                title: '供应商',
                render: (_, row) => row.supplier?.name || row.supplierId.slice(0, 8),
                width: 180,
              },
              {
                title: '货源链接',
                dataIndex: 'sourceUrl',
                ellipsis: true,
                render: (v: string) =>
                  v ? (
                    <Typography.Link href={v} target="_blank">
                      {v}
                    </Typography.Link>
                  ) : (
                    '-'
                  ),
              },
              { title: '优先级', dataIndex: 'priority', width: 90, sorter: (a, b) => a.priority - b.priority },
              {
                title: '主供应商',
                dataIndex: 'isPrimary',
                width: 100,
                render: (v: boolean) => (v ? <Tag color="blue">主</Tag> : '-'),
              },
              {
                title: '锁定',
                dataIndex: 'locked',
                width: 90,
                render: (v: boolean, row) => (
                  <Switch
                    size="small"
                    disabled={!writable}
                    checked={v}
                    onChange={async (checked) => {
                      try {
                        await updateProductSource(row.id, { locked: checked });
                        message.success(checked ? '已锁定（不参与自动切换）' : '已解锁');
                        void load();
                      } catch (e) {
                        message.error((e as Error).message || '操作失败');
                      }
                    }}
                  />
                ),
              },
              {
                title: '状态',
                dataIndex: 'status',
                width: 110,
                render: (v: string) => {
                  const cfg = SOURCE_STATUS_TAG[v] || { text: v, color: 'default' };
                  return <Tag color={cfg.color}>{cfg.text}</Tag>;
                },
              },
              {
                title: '操作',
                width: 220,
                render: (_, row) =>
                  !writable ? (
                    '-'
                  ) : (
                  <Space>
                    {!row.isPrimary && (
                      <a
                        onClick={async () => {
                          try {
                            await setPrimarySource(row.id);
                            message.success('已切换主供应商');
                            void load();
                          } catch (e) {
                            message.error((e as Error).message || '切换失败');
                          }
                        }}
                      >
                        设为主供应商
                      </a>
                    )}
                    <a onClick={() => openMapping(row)}>SKU映射</a>
                  </Space>
                  ),
              },
            ]}
          />
          <Typography.Title level={5} style={{ marginTop: 24 }}>
            切换审计
          </Typography.Title>
          <Table<SourceSwitchEvent>
            rowKey="id"
            size="small"
            dataSource={events}
            pagination={false}
            columns={[
              { title: '时间', dataIndex: 'createdAt', width: 180 },
              {
                title: '原因',
                dataIndex: 'reason',
                width: 120,
                render: (v: string) => SWITCH_REASON[v] || v,
              },
              {
                title: '方式',
                dataIndex: 'mode',
                width: 90,
                render: (v: string) => SWITCH_MODE[v] || v,
              },
              {
                title: '原货源',
                dataIndex: 'fromSourceId',
                render: (v?: string) => (v ? sourceLabel(v) : '-'),
              },
              { title: '新货源', dataIndex: 'toSourceId', render: (v: string) => sourceLabel(v) },
              {
                title: '处理状态',
                dataIndex: 'status',
                width: 100,
                render: (v: string | undefined, row) => {
                  if (row.mode !== 'suggested') return '-';
                  const cfg = SUGGESTION_STATUS_TAG[v || ''] || { text: v || '-', color: 'default' };
                  return <Tag color={cfg.color}>{cfg.text}</Tag>;
                },
              },
              {
                title: '操作',
                width: 160,
                render: (_, row) =>
                  writable && row.mode === 'suggested' && row.status === 'open' ? (
                    <Space>
                      <Popconfirm
                        title="采纳建议并把主供应商切换为该备选货源？"
                        onConfirm={async () => {
                          try {
                            await adoptSwitchSuggestion(row.id);
                            message.success('已采纳建议并切换主供应商');
                            void load();
                          } catch (e) {
                            message.error((e as Error).message || '采纳失败');
                          }
                        }}
                      >
                        <a>采纳建议</a>
                      </Popconfirm>
                      <a
                        onClick={async () => {
                          try {
                            await ignoreSwitchSuggestion(row.id);
                            message.success('已忽略该建议');
                            void load();
                          } catch (e) {
                            message.error((e as Error).message || '操作失败');
                          }
                        }}
                      >
                        忽略
                      </a>
                    </Space>
                  ) : (
                    '-'
                  ),
              },
            ]}
          />
        </>
      )}

      <Modal
        title="绑定货源"
        open={bindOpen}
        destroyOnClose
        onCancel={() => setBindOpen(false)}
        onOk={async () => {
          const values = await bindForm.validateFields();
          if (!productId) return;
          try {
            await bindProductSource(productId, values);
            message.success('货源已绑定');
            setBindOpen(false);
            bindForm.resetFields();
            void load();
          } catch (e) {
            message.error((e as Error).message || '绑定失败');
          }
        }}
      >
        <Form form={bindForm} layout="vertical" initialValues={{ priority: 100 }}>
          <Form.Item
            name="supplierName"
            label="供应商名称"
            rules={[{ required: true, message: '请输入供应商名称' }]}
          >
            <Input placeholder="不存在时自动创建" />
          </Form.Item>
          <Form.Item
            name="sourceUrl"
            label="1688 商品链接"
            rules={[{ required: true, message: '请输入 1688 商品链接' }]}
          >
            <Input placeholder="https://detail.1688.com/offer/xxxx.html" />
          </Form.Item>
          <Form.Item name="priority" label="优先级（数字越小越优先）">
            <InputNumber min={1} max={999} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="moq" label="起订量 MOQ（可选）">
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="leadTimeDays" label="备货周期（天，可选）">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={`SKU 映射 - ${mappingSource?.supplier?.name || ''}`}
        width={720}
        open={!!mappingSource}
        onClose={() => setMappingSource(null)}
        extra={
          <Button type="primary" onClick={() => void submitMappings()}>
            保存
          </Button>
        }
      >
        {localSkus.length === 0 ? (
          <Empty description="该商品没有本地 SKU" />
        ) : (
          <Table
            rowKey="localSkuId"
            size="small"
            dataSource={mappingRows}
            pagination={false}
            columns={[
              { title: '本地规格', dataIndex: 'skuName', width: 180 },
              {
                title: '货源规格 ID',
                dataIndex: 'externalSkuId',
                render: (_, row, idx) => (
                  <Input
                    value={row.externalSkuId}
                    placeholder="1688 skuId"
                    onChange={(e) => {
                      const next = [...mappingRows];
                      next[idx] = { ...row, externalSkuId: e.target.value };
                      setMappingRows(next);
                    }}
                  />
                ),
              },
              {
                title: '参考进价(CNY)',
                dataIndex: 'currentPrice',
                width: 150,
                render: (_, row, idx) => (
                  <InputNumber
                    value={row.currentPrice}
                    min={0}
                    step={0.01}
                    style={{ width: '100%' }}
                    onChange={(v) => {
                      const next = [...mappingRows];
                      next[idx] = { ...row, currentPrice: v ?? undefined };
                      setMappingRows(next);
                    }}
                  />
                ),
              },
              {
                title: '库存',
                dataIndex: 'currentStock',
                width: 110,
                render: (_, row, idx) => (
                  <InputNumber
                    value={row.currentStock}
                    min={0}
                    style={{ width: '100%' }}
                    onChange={(v) => {
                      const next = [...mappingRows];
                      next[idx] = { ...row, currentStock: v ?? undefined };
                      setMappingRows(next);
                    }}
                  />
                ),
              },
              {
                title: '操作',
                width: 90,
                render: (_, row, idx) =>
                  row.mappingId ? (
                    <Popconfirm
                      title="删除该 SKU 映射？删除后订单将无法按该映射生成采购单"
                      onConfirm={async () => {
                        try {
                          await deleteSkuMapping(row.mappingId!);
                          message.success('映射已删除');
                          const next = [...mappingRows];
                          next[idx] = {
                            localSkuId: row.localSkuId,
                            skuName: row.skuName,
                          };
                          setMappingRows(next);
                          void load();
                        } catch (e) {
                          message.error((e as Error).message || '删除失败');
                        }
                      }}
                    >
                      <Typography.Link type="danger">删除映射</Typography.Link>
                    </Popconfirm>
                  ) : (
                    '-'
                  ),
              },
            ]}
          />
        )}
      </Drawer>

      <Modal
        title="历史进价（近 90 天）"
        open={!!historySku}
        footer={null}
        onCancel={() => setHistorySku(null)}
        width={640}
      >
        <Table<SourcePriceHistoryRow>
          rowKey="id"
          size="small"
          dataSource={historyRows}
          pagination={{ pageSize: 10 }}
          columns={[
            { title: '时间', dataIndex: 'capturedAt', width: 200 },
            { title: '价格(CNY)', dataIndex: 'price', width: 120 },
            { title: '库存', dataIndex: 'stock', width: 100, render: (v) => v ?? '-' },
            {
              title: '来源',
              dataIndex: 'captureSource',
              render: (v: string) =>
                ({ crawl: '抓取', order: '下单', manual: '人工', api: 'API' } as Record<string, string>)[v] || v,
            },
          ]}
        />
      </Modal>
    </TmPageContainer>
  );
}

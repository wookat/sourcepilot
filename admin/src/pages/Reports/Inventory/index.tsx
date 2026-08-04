import { ProCard } from '@ant-design/pro-components';
import { Alert, Col, Row, Segmented, Skeleton, Space, Statistic, Table, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from '@umijs/max';
import { mergeQueryState } from '@/utils/urlState';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import { fetchInventoryReport, type InventoryReportDTO, type InventorySKURow } from '@/services/reports';

const SLOW_DAY_OPTIONS = [
  { label: '30 天无出库', value: 30 },
  { label: '60 天无出库', value: 60 },
  { label: '90 天无出库', value: 90 },
];

const DEFAULT_SLOW_DAYS = 30;

export function normalizeSlowDays(raw: string | null): number {
  const n = Number(raw);
  return SLOW_DAY_OPTIONS.some((o) => o.value === n) ? n : DEFAULT_SLOW_DAYS;
}

function formatDateTime(v?: string): string {
  if (!v) {
    return '从未出库';
  }
  return v.slice(0, 10);
}

/** 库存报表：库存价值（参考进价估）、周转天数、滞销预警与低库存清单 */
export default function InventoryReport() {
  const [searchParams] = useSearchParams();
  const slowDays = normalizeSlowDays(searchParams.get('slowDays'));
  const setSlowDays = useCallback((next: number) => {
    mergeQueryState({ slowDays: next === DEFAULT_SLOW_DAYS ? undefined : next }, { replace: true });
  }, []);

  const [data, setData] = useState<InventoryReportDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchInventoryReport(slowDays)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, [slowDays]);

  useEffect(() => {
    load();
  }, [load]);

  const summary = data?.summary;
  const slowMoving = data?.slowMoving ?? [];
  const lowStock = data?.lowStock ?? [];
  const hasSkus = (summary?.skuCount ?? 0) > 0;

  const skuColumns = useMemo<ColumnsType<InventorySKURow>>(
    () => [
      {
        title: '商品与规格',
        dataIndex: 'title',
        ellipsis: true,
        render: (v: string, r) => (
          <Tooltip title={`${v}${r.skuName ? ` / ${r.skuName}` : ''}`}>
            <span>
              {v}
              {r.skuName ? <Typography.Text type="secondary">（{r.skuName}）</Typography.Text> : null}
            </span>
          </Tooltip>
        ),
      },
      { title: 'SKU 编码', dataIndex: 'skuCode', width: 120, ellipsis: true },
      { title: '库存', dataIndex: 'stock', width: 80, align: 'right' },
      { title: '预警阈值', dataIndex: 'warningStock', width: 90, align: 'right' },
      {
        title: '参考进价(CNY)',
        dataIndex: 'unitCostCny',
        width: 120,
        align: 'right',
        render: (v?: number) => (v == null ? '缺进价' : <span style={tabularNumsStyle}>{formatAmount(v)}</span>),
      },
      {
        title: '库存价值(CNY)',
        dataIndex: 'stockValueCny',
        width: 120,
        align: 'right',
        render: (v?: number) => (v == null ? '-' : <span style={tabularNumsStyle}>{formatAmount(v)}</span>),
      },
      {
        title: '最近出库',
        dataIndex: 'lastOutboundAt',
        width: 110,
        render: (v?: string) => formatDateTime(v),
      },
    ],
    [],
  );

  return (
    <TmPageContainer
      title="库存报表"
      subTitle="库存价值按参考进价估算（CNY），周转天数按近 30 天出库速度"
      extra={
        <Space wrap className="tm-page-header-extra">
          <Segmented
            options={SLOW_DAY_OPTIONS}
            value={slowDays}
            onChange={(v) => setSlowDays(v as number)}
            aria-label="滞销判定天数"
          />
        </Space>
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          message="报表加载失败"
          description={error}
          action={<Typography.Link onClick={load}>重试</Typography.Link>}
          style={{ marginBottom: 16 }}
        />
      ) : null}

      {!loading && (summary?.unvaluedSkuCount ?? 0) > 0 ? (
        <Alert
          type="info"
          showIcon
          message={`${summary?.unvaluedSkuCount} 个 SKU 缺少参考进价，未计入库存价值合计`}
          style={{ marginBottom: 16 }}
        />
      ) : null}

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="合计">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="SKU 数" value={summary?.skuCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="总库存（件）" value={summary?.totalStock ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="库存价值（CNY，参考进价估）"
                value={formatAmount(summary?.stockValueCny ?? 0)}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title="周转天数（近 30 天出库速度）"
                value={summary?.turnoverDays == null ? '暂无出库记录' : summary.turnoverDays}
                valueStyle={tabularNumsStyle}
              />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="低库存 SKU" value={summary?.lowStockCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title="缺货 SKU" value={summary?.outOfStockCount ?? 0} valueStyle={tabularNumsStyle} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic
                title={`滞销 SKU（${slowDays} 天无出库）`}
                value={summary?.slowMovingCount ?? 0}
                valueStyle={tabularNumsStyle}
              />
            </Col>
          </Row>
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title={`滞销预警（${slowDays} 天无出库，有库存）`}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : slowMoving.length > 0 ? (
          <Table<InventorySKURow>
            rowKey="skuId"
            size="small"
            columns={skuColumns}
            dataSource={slowMoving}
            scroll={{ x: 760 }}
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        ) : (
          <EmptyState
            compact
            title="暂无滞销 SKU"
            description={hasSkus ? `所有有库存 SKU 在 ${slowDays} 天内均有出库` : '暂无本地 SKU 数据'}
          />
        )}
      </ProCard>

      <ProCard variant="outlined" style={{ marginBottom: 16 }} title="低库存清单（库存 ≤ 预警阈值，与库存预警口径一致）">
        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : lowStock.length > 0 ? (
          <Table<InventorySKURow>
            rowKey="skuId"
            size="small"
            columns={skuColumns}
            dataSource={lowStock}
            scroll={{ x: 760 }}
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        ) : (
          <EmptyState
            compact
            title="暂无低库存 SKU"
            description="库存低于预警阈值时会出现在这里，可在库存预警页调整阈值"
            actionLabel="去库存预警"
            actionPath="/inventory/alerts"
          />
        )}
      </ProCard>
    </TmPageContainer>
  );
}

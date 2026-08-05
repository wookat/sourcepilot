import { EmptyState } from '@/components/ui';
import { Alert, Descriptions, Drawer, Skeleton, Space, Tag, Typography, message } from 'antd';
import { Suspense, lazy, useEffect, useState } from 'react';
import { extractApiErrorMessage } from '@/services/request';
import { formatDateTime } from '@/utils/formatTime';
import {
  fetchCandidateInsights,
  fetchCandidatePriceTrend,
  type CandidateInsights,
  type PriceTrend,
} from '@/services/selection';

const Line = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Line })));

const NOT_COLLECTED = <Typography.Text type="secondary">未采集</Typography.Text>;

function num(v: number | undefined, suffix = ''): React.ReactNode {
  if (v === undefined || v === null) return NOT_COLLECTED;
  return `${v}${suffix}`;
}

function money(v: number | undefined, currency?: string): React.ReactNode {
  if (v === undefined || v === null) return NOT_COLLECTED;
  return `${v.toFixed(2)} ${currency || ''}`.trim();
}

type AIReasons = {
  summary?: string;
  risks?: string[];
  sellingPoints?: string[];
  fallback?: boolean;
};

function parseReasons(raw: unknown): AIReasons {
  if (!raw) return {};
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as AIReasons;
    } catch {
      return { summary: raw };
    }
  }
  return raw as AIReasons;
}

export type InsightsDrawerProps = {
  candidateId?: string;
  title?: string;
  open: boolean;
  onClose: () => void;
};

/** 候选商品数据面板：采集数据、站内同类目经营、AI 评分拆解、采集价格走势、外部数据源状态。 */
export default function InsightsDrawer({ candidateId, title, open, onClose }: InsightsDrawerProps) {
  const [data, setData] = useState<CandidateInsights | null>(null);
  const [trend, setTrend] = useState<PriceTrend | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open || !candidateId) return;
    let cancelled = false;
    setLoading(true);
    setData(null);
    setTrend(null);
    void (async () => {
      try {
        const [insights, trendRes] = await Promise.all([
          fetchCandidateInsights(candidateId),
          fetchCandidatePriceTrend(candidateId),
        ]);
        if (cancelled) return;
        setData(insights);
        setTrend(trendRes);
      } catch (e) {
        if (!cancelled) message.error(extractApiErrorMessage(e, '数据面板加载失败'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, candidateId]);

  const ev = data?.evaluation;
  const reasons = parseReasons(ev?.aiReasons);
  const collected = data?.collected;
  const benchmark = data?.benchmark;
  const points = trend?.points ?? [];

  return (
    <Drawer
      title={title ? `数据面板：${title}` : '候选商品数据面板'}
      open={open}
      onClose={onClose}
      width="min(720px, 100vw)"
      destroyOnClose
    >
      {loading ? (
        <Skeleton active paragraph={{ rows: 10 }} />
      ) : !data ? (
        <EmptyState title="数据面板加载失败" description="请关闭后重试。" compact />
      ) : (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions title="采集数据" size="small" column={{ xs: 1, sm: 2 }} bordered>
            <Descriptions.Item label="海外在售价">
              {money(collected?.marketPrice, collected?.marketCurrency)}
            </Descriptions.Item>
            <Descriptions.Item label="海外 30 天销量">{num(collected?.marketSales30d)}</Descriptions.Item>
            <Descriptions.Item label="海外评价数">{num(collected?.marketReviewCount)}</Descriptions.Item>
            <Descriptions.Item label="货源采集价">
              {money(collected?.sourcePrice, collected?.sourceCurrency || 'CNY')}
            </Descriptions.Item>
            <Descriptions.Item label="货源销量">{num(collected?.sourceSales)}</Descriptions.Item>
            <Descriptions.Item label="货源评价数">{num(collected?.sourceReviewCount)}</Descriptions.Item>
            <Descriptions.Item label="最近采集时间">
              {collected?.sourceCapturedAt ? formatDateTime(collected.sourceCapturedAt) : NOT_COLLECTED}
            </Descriptions.Item>
            <Descriptions.Item label="采集次数">{collected?.collectCount ?? 0}</Descriptions.Item>
          </Descriptions>

          <div>
            <Typography.Title level={5}>采集价格走势</Typography.Title>
            {points.length >= 2 ? (
              <Suspense fallback={<Skeleton active paragraph={{ rows: 4 }} />}>
                <Line
                  height={220}
                  data={points.map((p) => ({
                    time: formatDateTime(p.capturedAt),
                    price: p.price,
                  }))}
                  xField="time"
                  yField="price"
                  axis={{ y: { title: `价格（${trend?.currency || 'CNY'}）` } }}
                  point={{ sizeField: 3 }}
                />
              </Suspense>
            ) : (
              <EmptyState
                title="暂无价格走势"
                description="同一来源链接需要至少 2 次成功采集才能绘制价格走势，可在采集任务中对该链接再次发起采集。"
                actionLabel="前往采集任务"
                actionPath="/collect/tasks"
                compact
              />
            )}
          </div>

          <Descriptions
            title={`站内同类目经营${benchmark ? `（${benchmark.category}，近 ${benchmark.windowDays} 天）` : ''}`}
            size="small"
            column={{ xs: 1, sm: 2 }}
            bordered
          >
            {benchmark ? (
              <>
                <Descriptions.Item label="同类目商品数">{benchmark.productCount}</Descriptions.Item>
                <Descriptions.Item label="草稿平均毛利率">
                  {benchmark.avgDraftMarginPercent !== undefined
                    ? `${benchmark.avgDraftMarginPercent.toFixed(1)}%`
                    : '暂无成本数据'}
                </Descriptions.Item>
                <Descriptions.Item label="动销订单数">{benchmark.orderCount}</Descriptions.Item>
                <Descriptions.Item label="动销件数">{benchmark.soldQty}</Descriptions.Item>
                <Descriptions.Item label="销售额">
                  {benchmark.revenue !== undefined ? benchmark.revenue.toFixed(2) : '暂无订单数据'}
                </Descriptions.Item>
                <Descriptions.Item label="毛利 / 毛利率">
                  {benchmark.grossProfit !== undefined
                    ? `${benchmark.grossProfit.toFixed(2)}${
                        benchmark.grossMarginPercent !== undefined
                          ? ` / ${benchmark.grossMarginPercent.toFixed(1)}%`
                          : ''
                      }`
                    : '暂无成本数据'}
                </Descriptions.Item>
              </>
            ) : (
              <Descriptions.Item label="同类目数据">候选未填写类目，无法关联站内经营数据</Descriptions.Item>
            )}
          </Descriptions>

          <Descriptions title="AI 评分明细" size="small" column={{ xs: 1, sm: 2 }} bordered>
            {ev ? (
              <>
                <Descriptions.Item label="AI 评分">
                  {ev.aiScore !== undefined ? (
                    <Space size={4}>
                      <Tag color={ev.aiScore >= 70 ? 'green' : ev.aiScore >= 50 ? 'orange' : 'red'}>
                        {ev.aiScore.toFixed(0)} 分
                      </Tag>
                      {reasons.fallback ? <Tag color="warning">规则兜底</Tag> : null}
                    </Space>
                  ) : (
                    '未评分'
                  )}
                </Descriptions.Item>
                <Descriptions.Item label="模型">{ev.aiModel || '-'}</Descriptions.Item>
                <Descriptions.Item label="采购成本">{money(ev.purchaseCost, 'CNY')}</Descriptions.Item>
                <Descriptions.Item label="物流成本">{money(ev.shippingCost, 'CNY')}</Descriptions.Item>
                <Descriptions.Item label="平台佣金">{money(ev.commissionFee)}</Descriptions.Item>
                <Descriptions.Item label="到手成本">{money(ev.landedCost)}</Descriptions.Item>
                <Descriptions.Item label="预期利润">{money(ev.estProfit)}</Descriptions.Item>
                <Descriptions.Item label="预期利润率">
                  {ev.estMarginPercent !== undefined ? `${ev.estMarginPercent.toFixed(1)}%` : '-'}
                </Descriptions.Item>
                {reasons.summary ? (
                  <Descriptions.Item label="评分摘要">
                    {reasons.summary}
                  </Descriptions.Item>
                ) : null}
                {reasons.sellingPoints?.length ? (
                  <Descriptions.Item label="卖点">
                    {reasons.sellingPoints.join('；')}
                  </Descriptions.Item>
                ) : null}
                {reasons.risks?.length ? (
                  <Descriptions.Item label="风险">
                    {reasons.risks.join('；')}
                  </Descriptions.Item>
                ) : null}
              </>
            ) : (
              <Descriptions.Item label="评分状态">候选尚未完成评分</Descriptions.Item>
            )}
          </Descriptions>

          <div>
            <Typography.Title level={5}>外部数据源</Typography.Title>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              {(data.external || []).map((src) => (
                <Alert
                  key={src.name}
                  type={src.configured ? 'success' : 'info'}
                  showIcon
                  message={
                    <Space size={8}>
                      {src.displayName}
                      {src.configured ? <Tag color="green">已接入</Tag> : <Tag>未接入</Tag>}
                    </Space>
                  }
                  description={src.configured ? undefined : src.message || '未配置平台开放接口凭证，接入后可补充热销榜与趋势数据。'}
                />
              ))}
              {(data.external || []).length === 0 ? (
                <Typography.Text type="secondary">暂无可用外部数据源</Typography.Text>
              ) : null}
            </Space>
          </div>
        </Space>
      )}
    </Drawer>
  );
}

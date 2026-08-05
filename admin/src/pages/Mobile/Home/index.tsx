import {
  BellOutlined,
  ReloadOutlined,
  RightOutlined,
  SendOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { history } from '@umijs/max';
import { Button, Skeleton, Space, Tag, Typography } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { formatAmount, formatCount, tabularNumsStyle } from '@/constants/chartTokens';
import { usePermission } from '@/hooks/usePermission';
import { usePullToRefresh } from '@/hooks/usePullToRefresh';
import { canAccessPath } from '@/utils/menuAccess';
import {
  queryProductOperationDashboard,
  type DashboardSummary,
  type DashboardTodo,
} from '@/services/dashboard';
import { fetchOrderSalesStats, type SalesStatsDTO, type SalesWindowStats } from '@/services/orders';
import { fetchProfitReport, type ProfitSummary } from '@/services/reports';

/** 移动首页收纳的关键待办（顺序即展示顺序） */
const MOBILE_TODO_KEYS = [
  'order_await_payment',
  'order_await_procurement',
  'procurement_await_receipt',
  'order_await_shipment',
  'order_exceptions',
] as const;

type MetricWindow = {
  label: string;
  orderCount?: number;
  paidAmountBase?: number;
  grossProfitBase?: number;
  missingCostLines?: number;
};

function pickWindow(stats: SalesStatsDTO | undefined, key: string): SalesWindowStats | undefined {
  return stats?.windows?.find((w) => w.key === key);
}

function MetricGroupCard({
  win,
  baseCurrency,
  loading,
}: {
  win: MetricWindow;
  baseCurrency: string;
  loading: boolean;
}) {
  return (
    <div className="tm-mobile-metric-card" data-testid={`tm-mobile-metric-${win.label}`}>
      <Typography.Text type="secondary" style={{ fontSize: 13 }}>
        {win.label}
      </Typography.Text>
      {loading ? (
        <Skeleton active paragraph={{ rows: 1 }} title={false} style={{ marginTop: 8 }} />
      ) : (
        <div className="tm-mobile-metric-card__body">
          <div className="tm-mobile-metric-card__item">
            <span className="tm-mobile-metric-card__label">订单</span>
            <span className="tm-mobile-metric-card__value" style={tabularNumsStyle}>
              {formatCount(win.orderCount ?? 0)}
            </span>
          </div>
          <div className="tm-mobile-metric-card__item">
            <span className="tm-mobile-metric-card__label">销售额</span>
            <span className="tm-mobile-metric-card__value" style={tabularNumsStyle}>
              {formatAmount(win.paidAmountBase ?? 0, baseCurrency)}
            </span>
          </div>
          <div className="tm-mobile-metric-card__item">
            <span className="tm-mobile-metric-card__label">毛利</span>
            <span className="tm-mobile-metric-card__value" style={tabularNumsStyle}>
              {win.grossProfitBase == null ? '—' : formatAmount(win.grossProfitBase, baseCurrency)}
            </span>
          </div>
        </div>
      )}
      {!loading && (win.missingCostLines ?? 0) > 0 ? (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          毛利含 {win.missingCostLines} 行缺进价，仅供参考
        </Typography.Text>
      ) : null}
    </div>
  );
}

function TodoRow({ todo }: { todo: DashboardTodo }) {
  return (
    <button
      type="button"
      className="tm-mobile-list-row"
      onClick={() => history.push(todo.link)}
      data-testid={`tm-mobile-todo-${todo.key}`}
    >
      <span className="tm-mobile-list-row__title">{todo.title}</span>
      <span className="tm-mobile-list-row__extra">
        <Tag
          color={todo.count > 0 ? 'warning' : 'default'}
          style={{ marginInlineEnd: 0, ...tabularNumsStyle }}
        >
          {formatCount(todo.count ?? 0)}
        </Tag>
        <RightOutlined className="tm-mobile-list-row__arrow" />
      </span>
    </button>
  );
}

/** 移动工作台首页：今日/7 日核心指标 + 关键待办 + 告警摘要（<768px 主入口） */
export default function MobileHome() {
  const { user, role, permissions } = usePermission();
  const [loading, setLoading] = useState(true);
  const [salesStats, setSalesStats] = useState<SalesStatsDTO>();
  const [todos, setTodos] = useState<DashboardTodo[]>([]);
  const [summary, setSummary] = useState<Partial<DashboardSummary>>({});
  const [profitToday, setProfitToday] = useState<ProfitSummary>();
  const [profit7d, setProfit7d] = useState<ProfitSummary>();
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    const [sales, dashboard, p1, p7] = await Promise.allSettled([
      fetchOrderSalesStats(),
      queryProductOperationDashboard(),
      fetchProfitReport('order', { days: 1 }),
      fetchProfitReport('order', { days: 7 }),
    ]);
    if (sales.status === 'fulfilled') setSalesStats(sales.value);
    if (dashboard.status === 'fulfilled') {
      setTodos(dashboard.value?.todos ?? []);
      setSummary(dashboard.value?.summary ?? {});
    }
    if (p1.status === 'fulfilled') setProfitToday(p1.value.summary);
    if (p7.status === 'fulfilled') setProfit7d(p7.value.summary);
    if (sales.status === 'rejected' && dashboard.status === 'rejected') {
      setError('经营数据加载失败，请下拉或点击刷新重试');
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const { containerRef, pulling, ready, refreshing } = usePullToRefresh(load);

  const baseCurrency = salesStats?.baseCurrency || 'CNY';
  const todayWin = pickWindow(salesStats, 'today');
  const weekWin = pickWindow(salesStats, '7d') ?? pickWindow(salesStats, 'last7d');

  const metricWindows: MetricWindow[] = [
    {
      label: '今日',
      orderCount: todayWin?.orderCount,
      paidAmountBase: todayWin?.paidAmountBase,
      grossProfitBase: profitToday?.grossProfitBase,
      missingCostLines: profitToday?.missingCostLines,
    },
    {
      label: '近 7 日',
      orderCount: weekWin?.orderCount,
      paidAmountBase: weekWin?.paidAmountBase,
      grossProfitBase: profit7d?.grossProfitBase,
      missingCostLines: profit7d?.missingCostLines,
    },
  ];

  const mobileTodos = useMemo(() => {
    const byKey = new Map(todos.map((t) => [t.key, t]));
    return MOBILE_TODO_KEYS.map((key) => byKey.get(key))
      .filter((t): t is DashboardTodo => !!t)
      .filter((t) => canAccessPath(t.link.split('?')[0], role, permissions, user?.tenantId));
  }, [todos, role, permissions, user?.tenantId]);

  const alertCount = (summary.criticalAlertCount ?? 0) + (summary.openAlertCount ?? 0);
  const canSeeAlerts = canAccessPath('/ops/task-center/alerts', role, permissions, user?.tenantId);
  const canShip = canAccessPath('/orders/list', role, permissions, user?.tenantId);

  const pullHint = refreshing ? '刷新中…' : ready ? '松开立即刷新' : '下拉刷新';

  return (
    <div className="tm-mobile-home" ref={containerRef} data-testid="tm-mobile-home">
      {pulling || refreshing ? (
        <div className="tm-mobile-home__pull-hint">{pullHint}</div>
      ) : null}
      <div className="tm-mobile-home__header">
        <div>
          <Typography.Title level={4} style={{ margin: 0 }}>
            移动工作台
          </Typography.Title>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {user?.displayName || user?.username || '管理员'}，随时掌握经营与待办
          </Typography.Text>
        </div>
        <Button
          icon={<ReloadOutlined />}
          onClick={() => void load()}
          loading={loading}
          className="tm-mobile-touch-btn"
        >
          刷新
        </Button>
      </div>

      {error ? (
        <Typography.Text type="danger" style={{ display: 'block', marginBottom: 12 }}>
          {error}
        </Typography.Text>
      ) : null}

      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        {metricWindows.map((win) => (
          <MetricGroupCard key={win.label} win={win} baseCurrency={baseCurrency} loading={loading} />
        ))}
      </Space>

      <div className="tm-mobile-section">
        <Typography.Text strong className="tm-mobile-section__title">
          关键待办
        </Typography.Text>
        {loading ? (
          <Skeleton active paragraph={{ rows: 3 }} title={false} />
        ) : mobileTodos.length ? (
          <div className="tm-mobile-list">
            {mobileTodos.map((todo) => (
              <TodoRow key={todo.key} todo={todo} />
            ))}
          </div>
        ) : (
          <Typography.Text type="secondary">暂无关键待办</Typography.Text>
        )}
      </div>

      {canSeeAlerts || canShip ? (
        <div className="tm-mobile-section">
          <Typography.Text strong className="tm-mobile-section__title">
            快捷入口
          </Typography.Text>
          <div className="tm-mobile-list">
            {canSeeAlerts ? (
              <button
                type="button"
                className="tm-mobile-list-row"
                onClick={() => history.push('/ops/task-center/alerts')}
                data-testid="tm-mobile-alerts-entry"
              >
                <span className="tm-mobile-list-row__title">
                  <BellOutlined style={{ marginInlineEnd: 8 }} />
                  告警中心
                </span>
                <span className="tm-mobile-list-row__extra">
                  {alertCount > 0 ? (
                    <Tag color="error" icon={<WarningOutlined />} style={{ marginInlineEnd: 0 }}>
                      {formatCount(alertCount)} 条待处理
                    </Tag>
                  ) : (
                    <Tag style={{ marginInlineEnd: 0 }}>暂无告警</Tag>
                  )}
                  <RightOutlined className="tm-mobile-list-row__arrow" />
                </span>
              </button>
            ) : null}
            {canShip ? (
              <button
                type="button"
                className="tm-mobile-list-row"
                onClick={() =>
                  history.push('/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled')
                }
                data-testid="tm-mobile-batch-ship-entry"
              >
                <span className="tm-mobile-list-row__title">
                  <SendOutlined style={{ marginInlineEnd: 8 }} />
                  批量发货
                </span>
                <span className="tm-mobile-list-row__extra">
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    待发货订单列表
                  </Typography.Text>
                  <RightOutlined className="tm-mobile-list-row__arrow" />
                </span>
              </button>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

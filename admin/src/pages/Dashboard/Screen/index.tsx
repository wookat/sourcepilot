import {
  FullscreenExitOutlined,
  FullscreenOutlined,
  ReloadOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Badge,
  Col,
  ConfigProvider,
  Empty,
  Row,
  Segmented,
  Skeleton,
  Space,
  Tag,
  Tooltip,
  Typography,
  theme,
} from 'antd';
import { Suspense, lazy, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from '@umijs/max';
import {
  chartAxisXTickCount,
  formatAmount,
  formatCount,
  makeCategoryLabelFilter,
  tabularNumsStyle,
} from '@/constants/chartTokens';
import { formatDateTime } from '@/utils/formatTime';
import { queryDashboardScreen, type DashboardScreenDTO } from '@/services/dashboard';

const Line = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Line })));
const Column = lazy(() => import('@ant-design/plots').then((m) => ({ default: m.Column })));

const { Title, Text } = Typography;

/** 自动刷新间隔（秒）选项：投屏场景轮询即可，无需 WebSocket */
const REFRESH_OPTIONS = [
  { label: '15 秒', value: 15 },
  { label: '30 秒', value: 30 },
  { label: '60 秒', value: 60 },
  { label: '暂停', value: 0 },
];

const THEME_OPTIONS = [
  { label: '深色', value: 'dark' },
  { label: '浅色', value: 'light' },
];

const REFRESH_STORAGE_KEY = 'tm_dashboard_screen_refresh';
const THEME_STORAGE_KEY = 'tm_dashboard_screen_theme';

export function readStoredNumber(key: string, fallback: number, allowed: number[]): number {
  const raw = localStorage.getItem(key);
  if (raw == null || raw.trim() === '') return fallback;
  const value = Number(raw);
  return allowed.includes(value) ? value : fallback;
}

const SEVERITY_COLOR: Record<string, string> = {
  critical: 'red',
  high: 'red',
  medium: 'orange',
  low: 'blue',
};

const ALERT_TYPE_LABEL: Record<string, string> = {
  task_alert: '任务告警',
  low_stock: '低库存',
  out_of_stock: '断货',
};

export function formatHourTick(value: unknown): string {
  const text = String(value ?? '');
  const d = new Date(text);
  if (Number.isNaN(d.getTime())) return text;
  return `${String(d.getHours()).padStart(2, '0')}:00`;
}

function DashboardScreenBody({
  themeMode,
  setThemeMode,
}: {
  themeMode: string;
  setThemeMode: (v: string) => void;
}) {
  const [data, setData] = useState<DashboardScreenDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshSec, setRefreshSec] = useState(() =>
    readStoredNumber(REFRESH_STORAGE_KEY, 30, [0, 15, 30, 60]),
  );
  const [isFullscreen, setIsFullscreen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const dark = themeMode === 'dark';
  const { token } = theme.useToken();

  const load = useCallback(async () => {
    try {
      const res = await queryDashboardScreen();
      setData(res);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : '经营大屏数据加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    localStorage.setItem(REFRESH_STORAGE_KEY, String(refreshSec));
    if (!refreshSec) return undefined;
    const timer = window.setInterval(load, refreshSec * 1000);
    return () => window.clearInterval(timer);
  }, [refreshSec, load]);

  useEffect(() => {
    const onChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  const toggleFullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else if (rootRef.current) {
      void rootRef.current.requestFullscreen();
    }
  }, []);

  const bg = dark ? '#0b1220' : token.colorBgLayout;
  const cardBg = dark ? '#131c2e' : token.colorBgContainer;
  const border = dark ? '1px solid #24304a' : `1px solid ${token.colorBorderSecondary}`;

  const cardStyle: React.CSSProperties = {
    background: cardBg,
    border,
    borderRadius: 12,
    padding: 16,
    height: '100%',
  };

  const trendData = useMemo(() => {
    const items = data?.trend ?? [];
    return items.flatMap((p) => [
      { hour: p.hour, value: p.orderCount, type: '订单数' },
      { hour: p.hour, value: p.paidCount, type: '已付款' },
    ]);
  }, [data?.trend]);

  const funnelData = useMemo(
    () => (data?.funnel ?? []).map((s) => ({ stage: s.title, count: s.count })),
    [data?.funnel],
  );

  const alerts = data?.alerts ?? [];
  const marqueeActive = alerts.length > 6;

  return (
    <div
      ref={rootRef}
      data-testid="dashboard-screen-root"
      style={{
        minHeight: '100vh',
        background: bg,
        padding: 'clamp(12px, 1.5vw, 24px)',
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
        overflowX: 'hidden',
      }}
    >
      <style>{`
        @keyframes tmScreenTicker { 0% { transform: translateY(0); } 100% { transform: translateY(-50%); } }
        .tm-screen-ticker-active { animation: tmScreenTicker 30s linear infinite; }
        .tm-screen-ticker-active:hover { animation-play-state: paused; }
      `}</style>

      <Row align="middle" justify="space-between" gutter={[12, 12]} wrap>
        <Col>
          <Space align="baseline" size={12} wrap>
            <Title level={3} style={{ margin: 0 }}>
              经营大屏
            </Title>
            <Text type="secondary">
              {data ? `更新于 ${formatDateTime(data.generatedAt)}` : '实时经营数据'}
            </Text>
          </Space>
        </Col>
        <Col>
          <Space size={12} wrap>
            <Segmented
              options={THEME_OPTIONS}
              value={themeMode}
              onChange={(v) => setThemeMode(String(v))}
              aria-label="大屏主题"
            />
            <Segmented
              options={REFRESH_OPTIONS}
              value={refreshSec}
              onChange={(v) => setRefreshSec(Number(v))}
              aria-label="自动刷新间隔"
            />
            <Tooltip title="立即刷新">
              <ReloadOutlined
                role="button"
                aria-label="立即刷新"
                onClick={() => {
                  setLoading(true);
                  void load();
                }}
                style={{ fontSize: 18, cursor: 'pointer' }}
              />
            </Tooltip>
            <Tooltip title={isFullscreen ? '退出全屏' : '进入全屏'}>
              {isFullscreen ? (
                <FullscreenExitOutlined
                  role="button"
                  aria-label="退出全屏"
                  onClick={toggleFullscreen}
                  style={{ fontSize: 18, cursor: 'pointer' }}
                />
              ) : (
                <FullscreenOutlined
                  role="button"
                  aria-label="进入全屏"
                  onClick={toggleFullscreen}
                  style={{ fontSize: 18, cursor: 'pointer' }}
                />
              )}
            </Tooltip>
          </Space>
        </Col>
      </Row>

      {error ? (
        <Alert type="error" showIcon message="经营大屏数据加载失败" description={error} />
      ) : null}

      {loading && !data ? (
        <Skeleton active paragraph={{ rows: 10 }} />
      ) : (
        <>
          <Row gutter={[16, 16]}>
            <Col xs={24} sm={12} xl={6}>
              <div style={cardStyle} data-testid="screen-kpi-orders">
                <Text type="secondary">今日订单</Text>
                <div style={{ fontSize: 'clamp(28px, 2.5vw, 44px)', fontWeight: 600, ...tabularNumsStyle }}>
                  {formatCount(data?.today.orderCount ?? 0)}
                </div>
                <Text type="secondary">已付款 {formatCount(data?.today.paidOrderCount ?? 0)} 单</Text>
              </div>
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <div style={cardStyle} data-testid="screen-kpi-sales">
                <Text type="secondary">今日销售额（{data?.today.baseCurrency || '基准币'}）</Text>
                <div style={{ fontSize: 'clamp(28px, 2.5vw, 44px)', fontWeight: 600, ...tabularNumsStyle }}>
                  {formatAmount(data?.today.salesBase ?? 0)}
                </div>
                {data?.today.unconvertedCurrencies?.length ? (
                  <Text type="warning">未折算：{data.today.unconvertedCurrencies.join('、')}</Text>
                ) : (
                  <Text type="secondary">按已付款订单口径</Text>
                )}
              </div>
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <div style={cardStyle} data-testid="screen-kpi-profit">
                <Text type="secondary">今日毛利（{data?.today.baseCurrency || '基准币'}）</Text>
                <div style={{ fontSize: 'clamp(28px, 2.5vw, 44px)', fontWeight: 600, ...tabularNumsStyle }}>
                  {data?.today.grossProfitBase != null ? formatAmount(data.today.grossProfitBase) : '—'}
                </div>
                <Text type="secondary">
                  {data?.today.marginPercent != null
                    ? `毛利率 ${data.today.marginPercent.toFixed(1)}%`
                    : '缺少汇率或成本，暂无法计算'}
                </Text>
              </div>
            </Col>
            <Col xs={24} sm={12} xl={6}>
              <div style={cardStyle} data-testid="screen-kpi-alerts">
                <Text type="secondary">当前告警</Text>
                <div style={{ fontSize: 'clamp(28px, 2.5vw, 44px)', fontWeight: 600, ...tabularNumsStyle }}>
                  {formatCount(alerts.length)}
                </div>
                <Text type="secondary">任务告警与库存预警合计</Text>
              </div>
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            {(data?.todos ?? []).map((t) => (
              <Col flex="1 1 160px" key={t.key}>
                <Link to={t.link} style={{ display: 'block', height: '100%' }}>
                  <div style={cardStyle} data-testid={`screen-todo-${t.key}`}>
                    <Space size={8}>
                      <Text type="secondary">{t.title}</Text>
                      <Tag color={t.priority === 'P0' ? 'red' : t.priority === 'P1' ? 'orange' : 'blue'}>
                        {t.priority}
                      </Tag>
                    </Space>
                    <div style={{ fontSize: 'clamp(22px, 1.8vw, 32px)', fontWeight: 600, ...tabularNumsStyle }}>
                      {formatCount(t.count)}
                    </div>
                  </div>
                </Link>
              </Col>
            ))}
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} xl={10}>
              <div style={cardStyle} data-testid="screen-funnel">
                <Title level={5} style={{ marginTop: 0 }}>
                  订单状态流转漏斗（近 {data?.funnelDays ?? 7} 天）
                </Title>
                {funnelData.every((d) => d.count === 0) ? (
                  <Empty description="近期暂无订单" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                ) : (
                  <Suspense fallback={<Skeleton active paragraph={{ rows: 5 }} />}>
                    <Column
                      data={funnelData}
                      xField="stage"
                      yField="count"
                      height={260}
                      maxColumnWidth={56}
                      theme={dark ? 'classicDark' : 'classic'}
                      label={{ text: 'count', position: 'top' }}
                    />
                  </Suspense>
                )}
              </div>
            </Col>
            <Col xs={24} xl={14}>
              <div style={cardStyle} data-testid="screen-trend">
                <Title level={5} style={{ marginTop: 0 }}>
                  近 {data?.trendHours ?? 24} 小时订单趋势
                </Title>
                <Suspense fallback={<Skeleton active paragraph={{ rows: 5 }} />}>
                  <Line
                    data={trendData}
                    xField="hour"
                    yField="value"
                    colorField="type"
                    height={260}
                    theme={dark ? 'classicDark' : 'classic'}
                    axis={{
                      x: {
                        labelFormatter: formatHourTick,
                        labelFilter: makeCategoryLabelFilter(
                          data?.trend.length ?? 24,
                          chartAxisXTickCount.compact,
                        ),
                      },
                    }}
                  />
                </Suspense>
              </div>
            </Col>
          </Row>

          <div style={{ ...cardStyle, height: 'auto' }} data-testid="screen-alerts">
            <Space align="baseline" size={8}>
              <WarningOutlined style={{ color: token.colorWarning }} />
              <Title level={5} style={{ marginTop: 0 }}>
                异常 / 低库存告警
              </Title>
            </Space>
            {alerts.length === 0 ? (
              <Empty description="当前没有待处理告警" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <div style={{ maxHeight: 220, overflow: 'hidden' }}>
                <div className={marqueeActive ? 'tm-screen-ticker-active' : undefined}>
                  {(marqueeActive ? [...alerts, ...alerts] : alerts).map((a, idx) => (
                    <Link
                      to={a.link}
                      key={`${a.type}-${a.title}-${idx}`}
                      style={{
                        display: 'flex',
                        alignItems: 'baseline',
                        gap: 8,
                        padding: '6px 0',
                        borderBottom: border,
                        color: 'inherit',
                      }}
                    >
                      <Badge color={SEVERITY_COLOR[a.severity] || 'orange'} />
                      <Tag>{ALERT_TYPE_LABEL[a.type] || a.type}</Tag>
                      <Text ellipsis style={{ flex: 1 }}>
                        {a.title}
                        {a.detail ? <Text type="secondary">（{a.detail}）</Text> : null}
                      </Text>
                      {a.occurredAt ? (
                        <Text type="secondary" style={{ whiteSpace: 'nowrap' }}>
                          {formatDateTime(a.occurredAt)}
                        </Text>
                      ) : null}
                    </Link>
                  ))}
                </div>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

export default function DashboardScreenPage() {
  const [themeMode, setThemeMode] = useState<string>(
    () => (localStorage.getItem(THEME_STORAGE_KEY) === 'light' ? 'light' : 'dark'),
  );

  useEffect(() => {
    localStorage.setItem(THEME_STORAGE_KEY, themeMode);
  }, [themeMode]);

  return (
    <ConfigProvider
      theme={{ algorithm: themeMode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm }}
    >
      <DashboardScreenBody themeMode={themeMode} setThemeMode={setThemeMode} />
    </ConfigProvider>
  );
}

import {
  CarOutlined,
  CheckCircleFilled,
  CloseOutlined,
  CloudUploadOutlined,
  FileTextOutlined,
  SettingOutlined,
  ShoppingCartOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Progress, Row, Tooltip, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { fetchCollectTasks } from '@/services/collectTasks';
import { queryOrders } from '@/services/orders';
import { fetchPurchaseOrders } from '@/services/procurement';
import { fetchProducts } from '@/services/products';
import { fetchSettingsList } from '@/services/settings';
import { fetchSuppliers } from '@/services/sourcing';
import { usePermission } from '@/hooks/usePermission';
import { pickGroup } from '@/utils/settingsForm';
import { appendSourceToUrl } from '@/utils/urlState';

export const ONBOARDING_DISMISSED_KEY = 'tm_dashboard_onboarding_dismissed_v1';

type StepKey = 'settings' | 'collect' | 'draft' | 'orders' | 'procurement' | 'ship';

type StepMeta = {
  key: StepKey;
  title: string;
  description: string;
  link: string;
  icon: ReactNode;
  color: string;
  bg: string;
};

const STEPS: StepMeta[] = [
  {
    key: 'settings',
    title: '配置 AI 与存储',
    description: '填写 AI 服务密钥，本地存储默认可用',
    link: '/settings/ai',
    icon: <SettingOutlined />,
    color: '#6366f1',
    bg: '#eef2ff',
  },
  {
    key: 'collect',
    title: '采集商品',
    description: '粘贴 1688 / 拼多多商品链接开始采集',
    link: '/collect/hub',
    icon: <CloudUploadOutlined />,
    color: '#2563eb',
    bg: '#eff6ff',
  },
  {
    key: 'draft',
    title: '完善草稿与货源',
    description: '编辑商品草稿并绑定供应商货源',
    link: '/product/drafts',
    icon: <FileTextOutlined />,
    color: '#7c3aed',
    bg: '#f5f3ff',
  },
  {
    key: 'orders',
    title: '导入订单',
    description: '批量导入或手工创建销售订单',
    link: '/orders/list',
    icon: <UnorderedListOutlined />,
    color: '#0891b2',
    bg: '#ecfeff',
  },
  {
    key: 'procurement',
    title: '生成采购单',
    description: '按已付款订单一键生成采购单',
    link: '/procurement/orders',
    icon: <ShoppingCartOutlined />,
    color: '#ea580c',
    bg: '#fff7ed',
  },
  {
    key: 'ship',
    title: '发货',
    description: '录入运单完成订单发货履约',
    link: '/orders/list',
    icon: <CarOutlined />,
    color: '#059669',
    bg: '#ecfdf5',
  },
];

function readDismissed(): boolean {
  try {
    return localStorage.getItem(ONBOARDING_DISMISSED_KEY) === '1';
  } catch {
    return false;
  }
}

async function detectDone(canReadSettings: boolean): Promise<Partial<Record<StepKey, boolean>>> {
  const one = { page: 1, pageSize: 1 };
  const [settings, collect, products, suppliers, orders, purchase, fulfilled, partial] =
    await Promise.allSettled([
      canReadSettings ? fetchSettingsList() : Promise.reject(new Error('no settings permission')),
      fetchCollectTasks(one),
      fetchProducts(one),
      fetchSuppliers(one),
      queryOrders(one),
      fetchPurchaseOrders(one),
      queryOrders({ ...one, fulfillmentStatus: 'fulfilled' }),
      queryOrders({ ...one, fulfillmentStatus: 'partial' }),
    ]);

  const total = (r: PromiseSettledResult<{ pagination: { total: number } }>) =>
    r.status === 'fulfilled' ? r.value.pagination.total : 0;
  const flatTotal = (r: PromiseSettledResult<{ total: number }>) =>
    r.status === 'fulfilled' ? r.value.total : 0;

  let settingsDone = false;
  if (settings.status === 'fulfilled') {
    const ai = pickGroup(settings.value.items, 'ai');
    const provider = (ai.provider || 'openai_compatible').trim();
    settingsDone = Boolean((ai[`${provider}_api_key`] || ai.api_key || '').trim());
  }

  return {
    settings: settingsDone,
    collect: total(collect) > 0,
    draft: total(products) > 0 && flatTotal(suppliers) > 0,
    orders: total(orders) > 0,
    procurement: flatTotal(purchase) > 0,
    ship: total(fulfilled) > 0 || total(partial) > 0,
  };
}

function StepCard({ step, index, done }: { step: StepMeta; index: number; done: boolean }) {
  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`第 ${index + 1} 步：${step.title}${done ? '（已完成）' : ''}`}
      onClick={() => history.push(appendSourceToUrl(step.link))}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') history.push(appendSourceToUrl(step.link));
      }}
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 12,
        height: '100%',
        minHeight: 88,
        padding: '14px 14px',
        borderRadius: 10,
        border: `1px solid ${done ? '#b7eb8f' : 'var(--ant-color-border-secondary, #f0f0f0)'}`,
        background: done ? '#f6ffed' : '#fff',
        cursor: 'pointer',
        transition: 'border-color 0.2s, box-shadow 0.2s',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = step.color;
        e.currentTarget.style.boxShadow = `0 2px 8px ${step.color}18`;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = done ? '#b7eb8f' : 'var(--ant-color-border-secondary, #f0f0f0)';
        e.currentTarget.style.boxShadow = 'none';
      }}
    >
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: 10,
          background: done ? '#f6ffed' : step.bg,
          color: done ? '#52c41a' : step.color,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 17,
          flexShrink: 0,
        }}
      >
        {done ? <CheckCircleFilled /> : step.icon}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <Typography.Text strong style={{ display: 'block', fontSize: 13 }}>
          {index + 1}. {step.title}
        </Typography.Text>
        <Typography.Text type="secondary" style={{ display: 'block', fontSize: 12, marginTop: 4 }}>
          {done ? '已完成，点击可再次进入' : step.description}
        </Typography.Text>
      </div>
    </div>
  );
}

/**
 * 首页新手入门引导卡：按业务闭环列出入门步骤，已有数据的步骤自动标记完成；
 * 可关闭，关闭状态保存在本地浏览器。
 */
export default function OnboardingGuide() {
  const [dismissed, setDismissed] = useState<boolean>(() => readDismissed());
  const [done, setDone] = useState<Partial<Record<StepKey, boolean>>>({});
  const { canManageSettings } = usePermission();

  useEffect(() => {
    if (dismissed) return;
    let cancelled = false;
    void detectDone(canManageSettings).then((res) => {
      if (!cancelled) setDone(res);
    });
    return () => {
      cancelled = true;
    };
  }, [dismissed, canManageSettings]);

  const doneCount = useMemo(() => STEPS.filter((s) => done[s.key]).length, [done]);

  if (dismissed) return null;

  return (
    <ProCard
      title="新手入门"
      variant="outlined"
      style={{ marginBottom: 16 }}
      bodyStyle={{ padding: '16px 20px 20px' }}
      extra={
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            已完成 {doneCount}/{STEPS.length}
          </Typography.Text>
          <Tooltip title="关闭引导（可在浏览器清除本地数据后恢复）">
            <Button
              type="text"
              size="small"
              aria-label="关闭新手入门引导"
              icon={<CloseOutlined />}
              onClick={() => {
                try {
                  localStorage.setItem(ONBOARDING_DISMISSED_KEY, '1');
                } catch {
                  /* 本地存储不可用时仅本次会话隐藏 */
                }
                setDismissed(true);
              }}
            />
          </Tooltip>
        </div>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12, fontSize: 13 }}>
        按下面的顺序完成配置与首单流转，即可跑通「采集 → 草稿 / 货源 → 订单 → 采购 → 发货」业务闭环。
      </Typography.Paragraph>
      <Progress
        percent={Math.round((doneCount / STEPS.length) * 100)}
        size="small"
        style={{ marginBottom: 16, maxWidth: 320 }}
      />
      <Row gutter={[12, 12]}>
        {STEPS.map((step, index) => (
          <Col xs={24} sm={12} md={8} lg={8} xl={4} key={step.key}>
            <StepCard step={step} index={index} done={Boolean(done[step.key])} />
          </Col>
        ))}
      </Row>
    </ProCard>
  );
}

import { TmPageContainer } from '@/components/ui';
import {
  AUTOMATION_ACTION_LABELS,
  AUTOMATION_EVENT_ACTIONS,
  AUTOMATION_EVENT_LABELS,
  createOrderAutomationRule,
  deleteOrderAutomationRule,
  dryRunOrderAutomationRule,
  listOrderAutomationRules,
  SHIPPING_APPLY_MODE_LABELS,
  updateOrderAutomationRule,
  WAREHOUSE_STRATEGY_LABELS,
  type AutomationAction,
  type AutomationTriggerEvent,
  type ShippingApplyMode,
  type WarehouseStrategy,
  type OrderAutomationDryRunResult,
  type OrderAutomationRuleBody,
  type OrderAutomationRuleRow,
} from '@/services/orderAutomation';
import { listOrderTags, type OrderTagRow } from '@/services/orderTags';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
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
  Tooltip,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

const PLATFORM_OPTIONS = [
  { value: 'douyin_shop', label: '抖店' },
  { value: 'manual', label: '手工渠道' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'shopee', label: 'Shopee' },
  { value: 'lazada', label: 'Lazada' },
  { value: 'shopify', label: 'Shopify' },
  { value: 'amazon', label: 'Amazon' },
];

const EVENT_OPTIONS = (
  Object.keys(AUTOMATION_EVENT_LABELS) as AutomationTriggerEvent[]
).map((v) => ({ value: v, label: AUTOMATION_EVENT_LABELS[v] }));

const ACTION_COLORS: Record<AutomationAction, string> = {
  confirm_payment: 'blue',
  generate_procurement: 'purple',
  mark_printed: 'cyan',
  notify_shipping: 'green',
  apply_shipping_rule: 'geekblue',
  assign_warehouse: 'orange',
  add_tag: 'magenta',
};

const SHIPPING_MODE_OPTIONS = (
  Object.keys(SHIPPING_APPLY_MODE_LABELS) as ShippingApplyMode[]
).map((v) => ({ value: v, label: SHIPPING_APPLY_MODE_LABELS[v] }));

const WAREHOUSE_STRATEGY_OPTIONS = (
  Object.keys(WAREHOUSE_STRATEGY_LABELS) as WarehouseStrategy[]
).map((v) => ({ value: v, label: WAREHOUSE_STRATEGY_LABELS[v] }));

function fmtRange(min?: number, max?: number, unit = '') {
  if (min == null && max == null) return '';
  if (min != null && max != null) return `${min}~${max}${unit}`;
  if (min != null) return `≥${min}${unit}`;
  return `≤${max}${unit}`;
}

function formToBody(v: Record<string, any>): OrderAutomationRuleBody {
  return {
    name: v.name,
    priority: v.priority ?? 0,
    triggerEvent: v.triggerEvent,
    action: v.action,
    minAmount: v.minAmount ?? undefined,
    maxAmount: v.maxAmount ?? undefined,
    clearMinAmount: v.minAmount == null,
    clearMaxAmount: v.maxAmount == null,
    platforms: v.platforms || [],
    requireReviewPassed: !!v.requireReviewPassed,
    shippingApplyMode: v.action === 'apply_shipping_rule' ? v.shippingApplyMode : undefined,
    warehouseStrategy: v.action === 'assign_warehouse' ? v.warehouseStrategy : undefined,
    tagIds: v.action === 'add_tag' ? v.tagIds || [] : undefined,
  };
}

export default function OrderAutomationRulesPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<OrderAutomationRuleRow[]>([]);
  const [tags, setTags] = useState<OrderTagRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [togglingId, setTogglingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: OrderAutomationRuleRow | null }>({
    open: false,
  });
  const [saving, setSaving] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRun, setDryRun] = useState<OrderAutomationDryRunResult | null>(null);
  const [form] = Form.useForm();
  const triggerEvent = Form.useWatch('triggerEvent', form) as AutomationTriggerEvent | undefined;

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listOrderAutomationRules());
      setTags(await listOrderTags());
    } catch (e) {
      setLoadError((e as Error).message || '加载自动化规则失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: OrderAutomationRuleRow) => {
    setModal({ open: true, row });
    setDryRun(null);
    form.setFieldsValue(
      row
        ? {
            name: row.name,
            priority: row.priority,
            triggerEvent: row.triggerEvent,
            action: row.action,
            minAmount: row.minAmount,
            maxAmount: row.maxAmount,
            platforms: row.platforms || [],
            requireReviewPassed: row.requireReviewPassed,
            shippingApplyMode: row.shippingApplyMode || 'recommend',
            warehouseStrategy: row.warehouseStrategy || 'default_warehouse',
            tagIds: row.tagIds || [],
          }
        : {
            name: '',
            priority: 10,
            triggerEvent: 'order_created',
            action: undefined,
            minAmount: undefined,
            maxAmount: undefined,
            platforms: [],
            requireReviewPassed: false,
            shippingApplyMode: 'recommend',
            warehouseStrategy: 'default_warehouse',
            tagIds: [],
          },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (modal.row) await updateOrderAutomationRule(modal.row.id, formToBody(v));
      else await createOrderAutomationRule(formToBody(v));
      message.success('已保存');
      setModal({ open: false });
      await load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const runDryRun = async () => {
    const v = await form.validateFields();
    setDryRunning(true);
    setDryRun(null);
    try {
      setDryRun(await dryRunOrderAutomationRule(formToBody(v)));
    } catch (e) {
      message.error((e as Error).message || '测试跑失败');
    } finally {
      setDryRunning(false);
    }
  };

  const toggleEnabled = async (row: OrderAutomationRuleRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateOrderAutomationRule(row.id, { enabled });
      message.success(enabled ? `已启用「${row.name}」` : `已停用「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: OrderAutomationRuleRow) => {
    try {
      await deleteOrderAutomationRule(row.id);
      message.success(`已删除「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  const conditionTags = (row: OrderAutomationRuleRow) => {
    const tags: string[] = [];
    const a = fmtRange(row.minAmount, row.maxAmount, ' 元');
    if (a) tags.push(`金额：${a}`);
    if (row.platforms?.length)
      tags.push(
        `平台：${row.platforms
          .map((p) => PLATFORM_OPTIONS.find((o) => o.value === p)?.label || p)
          .join('、')}`,
      );
    if (row.requireReviewPassed) tags.push('要求审单已通过');
    if (!tags.length) tags.push('无附加条件');
    return tags;
  };

  const action = Form.useWatch('action', form) as AutomationAction | undefined;

  const actionOptions = useMemo(
    () =>
      (triggerEvent ? AUTOMATION_EVENT_ACTIONS[triggerEvent] || [] : []).map((a) => ({
        value: a,
        label: AUTOMATION_ACTION_LABELS[a],
      })),
    [triggerEvent],
  );

  return (
    <TmPageContainer
      title="自动化订单规则"
      subTitle="订单状态事件（创建 / 进入待采购 / 采购签收 / 物流揽收）按优先级从小到大匹配，命中即执行站内自动动作；审单待审 / 挂起的订单一律不自动化，执行结果见「自动化执行日志」"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Tooltip title={readonly ? '只读账号不可新增规则' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => openModal()}>
              新增规则
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载自动化规则失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<OrderAutomationRuleRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 960 }}
          locale={{ emptyText: '暂无自动化规则，点击「新增规则」添加' }}
          columns={[
            { title: '优先级', dataIndex: 'priority', width: 80 },
            { title: '名称', dataIndex: 'name', width: 200 },
            {
              title: '触发时机',
              dataIndex: 'triggerEvent',
              width: 160,
              render: (v: AutomationTriggerEvent) => AUTOMATION_EVENT_LABELS[v] || v,
            },
            {
              title: '条件',
              key: 'conditions',
              render: (_, row) => (
                <Space size={4} wrap>
                  {conditionTags(row).map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </Space>
              ),
            },
            {
              title: '自动动作',
              dataIndex: 'action',
              width: 190,
              render: (v: AutomationAction, row) => (
                <Space size={4} wrap>
                  <Tag color={ACTION_COLORS[v]}>{AUTOMATION_ACTION_LABELS[v] || v}</Tag>
                  {v === 'apply_shipping_rule' && row.shippingApplyMode ? (
                    <Tag>{SHIPPING_APPLY_MODE_LABELS[row.shippingApplyMode]}</Tag>
                  ) : null}
                  {v === 'assign_warehouse' && row.warehouseStrategy ? (
                    <Tag>{WAREHOUSE_STRATEGY_LABELS[row.warehouseStrategy]}</Tag>
                  ) : null}
                  {v === 'add_tag'
                    ? (row.tagIds || []).map((id) => {
                        const t = tags.find((x) => x.id === id);
                        return t ? (
                          <Tag key={id} color={t.color === 'default' ? undefined : t.color}>
                            {t.name}
                          </Tag>
                        ) : null;
                      })
                    : null}
                </Space>
              ),
            },
            {
              title: '启用',
              dataIndex: 'enabled',
              width: 90,
              render: (v: boolean, row) => (
                <Switch
                  checked={v}
                  size="small"
                  disabled={readonly}
                  loading={togglingId === row.id}
                  onChange={(checked) => void toggleEnabled(row, checked)}
                />
              ),
            },
            {
              title: '操作',
              width: 140,
              render: (_, row) => (
                <Space size={4}>
                  <Button
                    size="small"
                    type="link"
                    disabled={readonly}
                    onClick={() => openModal(row)}
                  >
                    编辑
                  </Button>
                  <Popconfirm
                    title={`删除规则「${row.name}」？`}
                    okText="删除"
                    okButtonProps={{ danger: true }}
                    disabled={readonly}
                    onConfirm={() => void remove(row)}
                  >
                    <Button size="small" type="link" danger disabled={readonly}>
                      删除
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        title={modal.row ? `编辑规则：${modal.row.name}` : '新增自动化规则'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        okText="保存"
        forceRender
        footer={(_, { OkBtn, CancelBtn }) => (
          <Space>
            <Button loading={dryRunning} onClick={() => void runDryRun()}>
              测试跑（dry-run）
            </Button>
            <CancelBtn />
            <OkBtn />
          </Space>
        )}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="规则名称"
            rules={[{ required: true, message: '请填写规则名称' }]}
          >
            <Input placeholder="如：低额订单自动确认付款" />
          </Form.Item>
          <Form.Item
            name="priority"
            label="优先级（越小越先执行）"
            rules={[{ required: true, message: '请填写优先级' }]}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="triggerEvent"
            label="触发时机"
            rules={[{ required: true, message: '请选择触发时机' }]}
          >
            <Select
              options={EVENT_OPTIONS}
              placeholder="选择状态事件"
              onChange={() => form.setFieldValue('action', undefined)}
            />
          </Form.Item>
          <Form.Item
            name="action"
            label="自动动作"
            rules={[{ required: true, message: '请选择自动动作' }]}
          >
            <Select options={actionOptions} placeholder="选择命中后的自动动作" />
          </Form.Item>
          {action === 'apply_shipping_rule' ? (
            <Form.Item
              name="shippingApplyMode"
              label="发货规则应用方式（仅推荐 / 直接应用；发货时均可人工改选）"
              rules={[{ required: true, message: '请选择应用方式' }]}
            >
              <Select options={SHIPPING_MODE_OPTIONS} placeholder="选择应用方式" />
            </Form.Item>
          ) : null}
          {action === 'assign_warehouse' ? (
            <Form.Item
              name="warehouseStrategy"
              label="分仓策略（库存不足时执行失败并留痕，可在执行日志重试）"
              rules={[{ required: true, message: '请选择分仓策略' }]}
            >
              <Select options={WAREHOUSE_STRATEGY_OPTIONS} placeholder="选择分仓策略" />
            </Form.Item>
          ) : null}
          {action === 'add_tag' ? (
            <Form.Item
              name="tagIds"
              label="要添加的标签（命中后自动给订单打上；标签在「订单标签」页维护）"
              rules={[{ required: true, message: '请至少选择一个标签' }]}
            >
              <Select
                mode="multiple"
                placeholder="选择标签"
                options={tags.map((t) => ({ value: t.id, label: t.name }))}
                notFoundContent="暂无标签，请先在「系统设置 → 订单标签」创建"
              />
            </Form.Item>
          ) : null}
          <Form.Item label="订单金额区间（自动确认付款必须填上限，作为低风险限定）">
            <Space.Compact block>
              <Form.Item name="minAmount" noStyle>
                <InputNumber min={0} placeholder="下限（可选）" style={{ width: '50%' }} />
              </Form.Item>
              <Form.Item name="maxAmount" noStyle>
                <InputNumber min={0} placeholder="上限" style={{ width: '50%' }} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item name="platforms" label="指定平台（可选，多选；留空表示不限）">
            <Select mode="multiple" options={PLATFORM_OPTIONS} placeholder="选择平台" />
          </Form.Item>
          <Form.Item
            name="requireReviewPassed"
            label="仅审单已通过的订单（待审 / 挂起订单无论如何都不会自动化）"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          {dryRun ? (
            <Alert
              type={dryRun.matched > 0 ? 'warning' : 'success'}
              showIcon
              message={`测试跑结果：扫描最近 ${dryRun.scanned} 单，命中 ${dryRun.matched} 单，其中 ${dryRun.blocked} 单将被安全边界跳过`}
              description={
                dryRun.samples.length ? (
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {dryRun.samples.map((s) => (
                      <li key={s.orderId}>
                        {s.orderNo}（{s.amount}）：{s.reason}
                      </li>
                    ))}
                  </ul>
                ) : (
                  '没有命中样本'
                )
              }
            />
          ) : null}
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

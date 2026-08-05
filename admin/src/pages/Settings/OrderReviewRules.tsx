import { TmPageContainer } from '@/components/ui';
import {
  createOrderReviewRule,
  deleteOrderReviewRule,
  dryRunOrderReviewRule,
  listOrderReviewRules,
  REVIEW_ACTION_LABELS,
  updateOrderReviewRule,
  type OrderReviewAction,
  type OrderReviewDryRunResult,
  type OrderReviewRuleBody,
  type OrderReviewRuleRow,
} from '@/services/orderReview';
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
import { useCallback, useEffect, useState } from 'react';

const PLATFORM_OPTIONS = [
  { value: 'douyin_shop', label: '抖店' },
  { value: 'manual', label: '手工渠道' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'shopee', label: 'Shopee' },
  { value: 'lazada', label: 'Lazada' },
  { value: 'shopify', label: 'Shopify' },
  { value: 'amazon', label: 'Amazon' },
];

const ACTION_OPTIONS: { value: OrderReviewAction; label: string }[] = [
  { value: 'review', label: '打标待人工审核' },
  { value: 'hold', label: '挂起拦截' },
  { value: 'pass', label: '自动通过' },
];

const ACTION_COLORS: Record<OrderReviewAction, string> = {
  pass: 'blue',
  review: 'gold',
  hold: 'red',
};

function fmtRange(min?: number, max?: number, unit = '') {
  if (min == null && max == null) return '';
  if (min != null && max != null) return `${min}~${max}${unit}`;
  if (min != null) return `≥${min}${unit}`;
  return `≤${max}${unit}`;
}

function formToBody(v: Record<string, any>): OrderReviewRuleBody {
  return {
    name: v.name,
    priority: v.priority ?? 0,
    action: v.action,
    minAmount: v.minAmount ?? undefined,
    maxAmount: v.maxAmount ?? undefined,
    clearMinAmount: v.minAmount == null,
    clearMaxAmount: v.maxAmount == null,
    addressKeywords: v.addressKeywords || [],
    remarkKeywords: v.remarkKeywords || [],
    platforms: v.platforms || [],
    maxTotalQuantity: v.maxTotalQuantity ?? undefined,
    clearMaxTotalQuantity: v.maxTotalQuantity == null,
    maxSkuQuantity: v.maxSkuQuantity ?? undefined,
    clearMaxSkuQuantity: v.maxSkuQuantity == null,
    repeatReceiverMinOrders: v.repeatReceiverMinOrders ?? undefined,
    repeatReceiverWindowDays: v.repeatReceiverWindowDays ?? undefined,
    clearRepeatReceiver: v.repeatReceiverMinOrders == null,
  };
}

export default function OrderReviewRulesPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<OrderReviewRuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [togglingId, setTogglingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: OrderReviewRuleRow | null }>({
    open: false,
  });
  const [saving, setSaving] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRun, setDryRun] = useState<OrderReviewDryRunResult | null>(null);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listOrderReviewRules());
    } catch (e) {
      setLoadError((e as Error).message || '加载审单规则失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: OrderReviewRuleRow) => {
    setModal({ open: true, row });
    setDryRun(null);
    form.setFieldsValue(
      row
        ? {
            name: row.name,
            priority: row.priority,
            action: row.action,
            minAmount: row.minAmount,
            maxAmount: row.maxAmount,
            addressKeywords: row.addressKeywords || [],
            remarkKeywords: row.remarkKeywords || [],
            platforms: row.platforms || [],
            maxTotalQuantity: row.maxTotalQuantity,
            maxSkuQuantity: row.maxSkuQuantity,
            repeatReceiverMinOrders: row.repeatReceiverMinOrders,
            repeatReceiverWindowDays: row.repeatReceiverWindowDays,
          }
        : {
            name: '',
            priority: 10,
            action: 'review',
            minAmount: undefined,
            maxAmount: undefined,
            addressKeywords: [],
            remarkKeywords: [],
            platforms: [],
            maxTotalQuantity: undefined,
            maxSkuQuantity: undefined,
            repeatReceiverMinOrders: undefined,
            repeatReceiverWindowDays: undefined,
          },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (modal.row) await updateOrderReviewRule(modal.row.id, formToBody(v));
      else await createOrderReviewRule(formToBody(v));
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
      setDryRun(await dryRunOrderReviewRule(formToBody(v)));
    } catch (e) {
      message.error((e as Error).message || '测试跑失败');
    } finally {
      setDryRunning(false);
    }
  };

  const toggleEnabled = async (row: OrderReviewRuleRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateOrderReviewRule(row.id, { enabled });
      message.success(enabled ? `已启用「${row.name}」` : `已停用「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: OrderReviewRuleRow) => {
    try {
      await deleteOrderReviewRule(row.id);
      message.success(`已删除「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  const conditionTags = (row: OrderReviewRuleRow) => {
    const tags: string[] = [];
    const a = fmtRange(row.minAmount, row.maxAmount, ' 元');
    if (a) tags.push(`金额：${a}`);
    if (row.addressKeywords?.length) tags.push(`地址关键词：${row.addressKeywords.join('、')}`);
    if (row.remarkKeywords?.length) tags.push(`备注关键词：${row.remarkKeywords.join('、')}`);
    if (row.platforms?.length)
      tags.push(
        `平台：${row.platforms
          .map((p) => PLATFORM_OPTIONS.find((o) => o.value === p)?.label || p)
          .join('、')}`,
      );
    if (row.maxTotalQuantity != null) tags.push(`商品总数 >${row.maxTotalQuantity}`);
    if (row.maxSkuQuantity != null) tags.push(`单 SKU 数量 >${row.maxSkuQuantity}`);
    if (row.repeatReceiverMinOrders != null)
      tags.push(
        `同收件人 ${row.repeatReceiverWindowDays || 7} 天内 ≥${row.repeatReceiverMinOrders} 单`,
      );
    return tags;
  };

  return (
    <TmPageContainer
      title="审单规则"
      subTitle="订单进入（导入 / 手工新建 / 批量粘贴）时按优先级从小到大匹配，第一条命中的规则决定动作；待审核 / 挂起的订单在审单工作台处理，放行前不能生成采购单和发货"
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
            message="加载审单规则失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<OrderReviewRuleRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 900 }}
          locale={{ emptyText: '暂无审单规则，点击「新增规则」添加' }}
          columns={[
            { title: '优先级', dataIndex: 'priority', width: 80 },
            { title: '名称', dataIndex: 'name', width: 200 },
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
              title: '动作',
              dataIndex: 'action',
              width: 120,
              render: (v: OrderReviewAction) => (
                <Tag color={ACTION_COLORS[v]}>{REVIEW_ACTION_LABELS[v] || v}</Tag>
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
        title={modal.row ? `编辑规则：${modal.row.name}` : '新增审单规则'}
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
            <Input placeholder="如：大额订单人工审核" />
          </Form.Item>
          <Form.Item
            name="priority"
            label="优先级（越小越先匹配）"
            rules={[{ required: true, message: '请填写优先级' }]}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="action"
            label="命中动作"
            rules={[{ required: true, message: '请选择命中动作' }]}
          >
            <Select options={ACTION_OPTIONS} placeholder="选择命中后的动作" />
          </Form.Item>
          <Form.Item label="订单金额区间（可选）">
            <Space.Compact block>
              <Form.Item name="minAmount" noStyle>
                <InputNumber min={0} placeholder="下限" style={{ width: '50%' }} />
              </Form.Item>
              <Form.Item name="maxAmount" noStyle>
                <InputNumber min={0} placeholder="上限" style={{ width: '50%' }} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item
            name="addressKeywords"
            label="收货地址关键词 / 黑名单地区（可选，命中任一即算）"
          >
            <Select mode="tags" placeholder="如：某某区、偏远地区" tokenSeparators={[',', '，', ' ']} />
          </Form.Item>
          <Form.Item name="remarkKeywords" label="买家备注关键词（可选，命中任一即算）">
            <Select mode="tags" placeholder="如：改地址、加急" tokenSeparators={[',', '，', ' ']} />
          </Form.Item>
          <Form.Item name="platforms" label="指定平台（可选，多选；留空表示不限）">
            <Select mode="multiple" options={PLATFORM_OPTIONS} placeholder="选择平台" />
          </Form.Item>
          <Form.Item name="maxTotalQuantity" label="商品总数量超过（可选）">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="如：10" />
          </Form.Item>
          <Form.Item name="maxSkuQuantity" label="单 SKU 数量超过（可选）">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="如：5" />
          </Form.Item>
          <Form.Item label="重复收件人多单（可选）">
            <Space.Compact block>
              <Form.Item name="repeatReceiverMinOrders" noStyle>
                <InputNumber min={1} placeholder="订单数达到（含本单）" style={{ width: '50%' }} />
              </Form.Item>
              <Form.Item name="repeatReceiverWindowDays" noStyle>
                <InputNumber min={1} placeholder="统计窗口（天，默认 7）" style={{ width: '50%' }} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          {dryRun ? (
            <Alert
              type={dryRun.matched > 0 ? 'warning' : 'success'}
              showIcon
              message={`测试跑结果：扫描最近 ${dryRun.scanned} 单，命中 ${dryRun.matched} 单`}
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

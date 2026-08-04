import { TmPageContainer } from '@/components/ui';
import { listCarriers, type CarrierRow } from '@/services/carriers';
import {
  createShippingRule,
  deleteShippingRule,
  listShippingRules,
  updateShippingRule,
  type ShippingRuleRow,
} from '@/services/waybill';
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

function fmtRange(min?: number, max?: number, unit = '') {
  if (min == null && max == null) return '';
  if (min != null && max != null) return `${min}~${max}${unit}`;
  if (min != null) return `≥${min}${unit}`;
  return `≤${max}${unit}`;
}

export default function ShippingRulesPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<ShippingRuleRow[]>([]);
  const [carriers, setCarriers] = useState<CarrierRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [togglingId, setTogglingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: ShippingRuleRow | null }>({
    open: false,
  });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [ruleRows, carrierRows] = await Promise.all([
        listShippingRules(),
        listCarriers({ enabled: true }),
      ]);
      setRows(ruleRows);
      setCarriers(carrierRows);
    } catch (e) {
      setLoadError((e as Error).message || '加载发货规则失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const carrierName = (code: string) =>
    carriers.find((c) => c.code === code)?.name || code;

  const openModal = (row?: ShippingRuleRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(
      row
        ? {
            name: row.name,
            priority: row.priority,
            provinces: row.provinces || [],
            platforms: row.platforms || [],
            minWeightKg: row.minWeightKg,
            maxWeightKg: row.maxWeightKg,
            minAmount: row.minAmount,
            maxAmount: row.maxAmount,
            carrierCode: row.carrierCode,
          }
        : {
            name: '',
            priority: 10,
            provinces: [],
            platforms: [],
            minWeightKg: undefined,
            maxWeightKg: undefined,
            minAmount: undefined,
            maxAmount: undefined,
            carrierCode: undefined,
          },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    const body = {
      name: v.name,
      priority: v.priority ?? 0,
      provinces: v.provinces || [],
      platforms: v.platforms || [],
      minWeightKg: v.minWeightKg ?? null,
      maxWeightKg: v.maxWeightKg ?? null,
      minAmount: v.minAmount ?? null,
      maxAmount: v.maxAmount ?? null,
      carrierCode: v.carrierCode,
    };
    setSaving(true);
    try {
      if (modal.row) await updateShippingRule(modal.row.id, body);
      else await createShippingRule(body);
      message.success('已保存');
      setModal({ open: false });
      await load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (row: ShippingRuleRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateShippingRule(row.id, { enabled });
      message.success(enabled ? `已启用「${row.name}」` : `已停用「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: ShippingRuleRow) => {
    try {
      await deleteShippingRule(row.id);
      message.success(`已删除「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  const conditionTags = (row: ShippingRuleRow) => {
    const tags: string[] = [];
    if (row.provinces?.length) tags.push(`省份：${row.provinces.join('、')}`);
    if (row.platforms?.length)
      tags.push(
        `平台：${row.platforms
          .map((p) => PLATFORM_OPTIONS.find((o) => o.value === p)?.label || p)
          .join('、')}`,
      );
    const w = fmtRange(row.minWeightKg, row.maxWeightKg, 'kg');
    if (w) tags.push(`重量：${w}`);
    const a = fmtRange(row.minAmount, row.maxAmount, ' 元');
    if (a) tags.push(`金额：${a}`);
    if (!tags.length) tags.push('无条件（兜底）');
    return tags;
  };

  return (
    <TmPageContainer
      title="发货规则"
      subTitle="按目的省份 / 重量段 / 金额段 / 平台自动推荐物流商，按优先级从小到大命中第一条；发货时可手动覆盖，不强制"
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
            message="加载发货规则失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<ShippingRuleRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 900 }}
          locale={{ emptyText: '暂无发货规则，点击「新增规则」添加' }}
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
              title: '推荐物流商',
              dataIndex: 'carrierCode',
              width: 140,
              render: (v: string) => <Tag color="blue">{carrierName(v)}</Tag>,
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
        title={modal.row ? `编辑规则：${modal.row.name}` : '新增发货规则'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="规则名称" rules={[{ required: true, message: '请填写规则名称' }]}>
            <Input placeholder="如：江浙沪标准件走中通" />
          </Form.Item>
          <Form.Item
            name="priority"
            label="优先级（越小越先匹配）"
            rules={[{ required: true, message: '请填写优先级' }]}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="provinces"
            label="目的省份（可选，多选；留空表示不限）"
            extra="订单暂不含收货地址，打单/发货时可手动补充省份参与匹配"
          >
            <Select mode="tags" placeholder="如：上海、江苏、浙江" tokenSeparators={[',', '，', ' ']} />
          </Form.Item>
          <Form.Item name="platforms" label="平台（可选，多选；留空表示不限）">
            <Select mode="multiple" options={PLATFORM_OPTIONS} placeholder="选择平台" />
          </Form.Item>
          <Form.Item label="重量段（kg，可选）">
            <Space.Compact block>
              <Form.Item name="minWeightKg" noStyle>
                <InputNumber min={0} placeholder="下限" style={{ width: '50%' }} />
              </Form.Item>
              <Form.Item name="maxWeightKg" noStyle>
                <InputNumber min={0} placeholder="上限" style={{ width: '50%' }} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item label="金额段（订单金额，可选）">
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
            name="carrierCode"
            label="推荐物流商"
            rules={[{ required: true, message: '请选择推荐物流商' }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={carriers.map((c) => ({ value: c.code, label: `${c.name}（${c.code}）` }))}
              placeholder="选择启用中的物流商"
            />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

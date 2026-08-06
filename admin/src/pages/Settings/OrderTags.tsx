import { TmPageContainer } from '@/components/ui';
import {
  ORDER_TAG_COLORS,
  createOrderTag,
  deleteOrderTag,
  listOrderTags,
  updateOrderTag,
  type OrderTagRow,
} from '@/services/orderTags';
import { formatDateTime } from '@/utils/formatTime';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

const COLOR_LABELS: Record<string, string> = {
  blue: '蓝色',
  green: '绿色',
  red: '红色',
  orange: '橙色',
  gold: '金色',
  purple: '紫色',
  cyan: '青色',
  magenta: '洋红',
  geekblue: '极客蓝',
  volcano: '火山红',
  lime: '青柠',
  default: '默认灰',
};

const COLOR_OPTIONS = ORDER_TAG_COLORS.map((c) => ({
  value: c,
  label: (
    <Space size={4}>
      <Tag color={c === 'default' ? undefined : c}>{COLOR_LABELS[c] || c}</Tag>
    </Space>
  ),
}));

export default function OrderTagsPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<OrderTagRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: OrderTagRow | null }>({ open: false });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listOrderTags());
    } catch (e) {
      setLoadError((e as Error).message || '加载订单标签失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: OrderTagRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(row ? { name: row.name, color: row.color } : { name: '', color: 'blue' });
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (modal.row) await updateOrderTag(modal.row.id, v);
      else await createOrderTag(v);
      message.success('已保存');
      setModal({ open: false });
      await load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const remove = async (row: OrderTagRow) => {
    try {
      await deleteOrderTag(row.id);
      message.success(`已删除「${row.name}」`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  return (
    <TmPageContainer
      title="订单标签"
      subTitle="租户级订单标签（名称 / 颜色）。可在订单列表批量打标、订单详情手工打标 / 去标，或在自动化订单规则中配置「自动打标签」动作"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Tooltip title={readonly ? '只读账号不可新增标签' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => openModal()}>
              新增标签
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载订单标签失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<OrderTagRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 520 }}
          locale={{ emptyText: '暂无订单标签，点击「新增标签」添加' }}
          columns={[
            {
              title: '标签',
              dataIndex: 'name',
              render: (v: string, row) => (
                <Tag color={row.color === 'default' ? undefined : row.color}>{v}</Tag>
              ),
            },
            {
              title: '颜色',
              dataIndex: 'color',
              width: 120,
              render: (v: string) => COLOR_LABELS[v] || v,
            },
            {
              title: '创建时间',
              dataIndex: 'createdAt',
              width: 180,
              render: (v: string) => formatDateTime(v),
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
                    title={`删除标签「${row.name}」？订单上已打的该标签会一并移除`}
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
        title={modal.row ? `编辑标签：${modal.row.name}` : '新增标签'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        okText="保存"
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="标签名称（租户内唯一，最长 32 字）"
            rules={[{ required: true, message: '请填写标签名称' }]}
          >
            <Input placeholder="如：加急、大客户" maxLength={32} />
          </Form.Item>
          <Form.Item
            name="color"
            label="标签颜色"
            rules={[{ required: true, message: '请选择标签颜色' }]}
          >
            <Select options={COLOR_OPTIONS} placeholder="选择颜色" />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

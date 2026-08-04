import { TmPageContainer } from '@/components/ui';
import {
  createCarrier,
  deleteCarrier,
  listCarriers,
  updateCarrier,
  type CarrierRow,
} from '@/services/carriers';
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
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

export default function CarrierSettingsPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<CarrierRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [keyword, setKeyword] = useState('');
  const [togglingId, setTogglingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: CarrierRow | null }>({ open: false });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async (kw?: string) => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listCarriers({ keyword: kw }));
    } catch (e) {
      setLoadError((e as Error).message || '加载物流商失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: CarrierRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(
      row
        ? {
            code: row.code,
            name: row.name,
            trackingUrlTemplate: row.trackingUrlTemplate,
            sortOrder: row.sortOrder,
          }
        : { code: '', name: '', trackingUrlTemplate: '', sortOrder: 0 },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (modal.row) {
        await updateCarrier(modal.row.id, {
          name: v.name,
          trackingUrlTemplate: v.trackingUrlTemplate,
          sortOrder: v.sortOrder,
        });
      } else {
        await createCarrier(v);
      }
      message.success('已保存');
      setModal({ open: false });
      await load(keyword);
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (row: CarrierRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateCarrier(row.id, { enabled });
      message.success(enabled ? `已启用 ${row.name}` : `已停用 ${row.name}`);
      await load(keyword);
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: CarrierRow) => {
    try {
      await deleteCarrier(row.id);
      message.success(`已删除 ${row.name}`);
      await load(keyword);
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  return (
    <TmPageContainer
      title="物流商"
      subTitle="管理发货可选物流商：预置国内常用快递可启停，自定义物流商可增删；发货与批量粘贴发货按此列表选择"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input.Search
            allowClear
            placeholder="按名称 / 代码搜索"
            style={{ width: 260 }}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onSearch={(v) => void load(v)}
          />
          <Tooltip title={readonly ? '只读账号不可新增物流商' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => openModal()}>
              新增自定义物流商
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载物流商失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load(keyword)}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<CarrierRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 720 }}
          locale={{ emptyText: '暂无物流商，点击「新增自定义物流商」添加' }}
          columns={[
            { title: '名称', dataIndex: 'name', width: 160 },
            {
              title: '代码',
              dataIndex: 'code',
              width: 120,
              render: (v: string) => <Tag>{v}</Tag>,
            },
            {
              title: '类型',
              dataIndex: 'isPreset',
              width: 90,
              render: (v: boolean) => (v ? <Tag color="blue">预置</Tag> : <Tag>自定义</Tag>),
            },
            {
              title: '轨迹查询 URL 模板',
              dataIndex: 'trackingUrlTemplate',
              ellipsis: true,
              render: (v: string) =>
                v ? (
                  <Tooltip title={v} placement="topLeft">
                    <span>{v}</span>
                  </Tooltip>
                ) : (
                  '—'
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
                  <Button size="small" type="link" disabled={readonly} onClick={() => openModal(row)}>
                    编辑
                  </Button>
                  {row.isPreset ? (
                    <Tooltip title="预置物流商不可删除，可停用">
                      <Button size="small" type="link" disabled>
                        删除
                      </Button>
                    </Tooltip>
                  ) : (
                    <Popconfirm
                      title={`删除物流商「${row.name}」？`}
                      okText="删除"
                      okButtonProps={{ danger: true }}
                      disabled={readonly}
                      onConfirm={() => void remove(row)}
                    >
                      <Button size="small" type="link" danger disabled={readonly}>
                        删除
                      </Button>
                    </Popconfirm>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        title={modal.row ? `编辑物流商：${modal.row.name}` : '新增自定义物流商'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="code"
            label="代码"
            rules={[
              { required: true, message: '请填写代码' },
              { pattern: /^[a-z0-9_-]{1,64}$/, message: '仅支持小写字母、数字、-、_' },
            ]}
            extra="批量粘贴发货时可直接用代码指定物流商（如 sf / zto）"
          >
            <Input disabled={!!modal.row} placeholder="如：yunexpress" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="如：云途物流" />
          </Form.Item>
          <Form.Item
            name="trackingUrlTemplate"
            label="轨迹查询 URL 模板（可选）"
            extra="用 {trackingNo} 占位运单号，例如 https://example.com/track?no={trackingNo}"
          >
            <Input placeholder="https://example.com/track?no={trackingNo}" />
          </Form.Item>
          <Form.Item name="sortOrder" label="排序（越小越靠前）" initialValue={0}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

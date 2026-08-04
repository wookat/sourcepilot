import { TmPageContainer } from '@/components/ui';
import {
  createWaybillTemplate,
  deleteWaybillTemplate,
  listWaybillTemplates,
  updateWaybillTemplate,
  WAYBILL_SIZE_LABELS,
  type WaybillTemplateRow,
} from '@/services/waybill';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  InputNumber,
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

const SIZE_OPTIONS = (Object.keys(WAYBILL_SIZE_LABELS) as WaybillTemplateRow['sizeCode'][]).map(
  (v) => ({ value: v, label: WAYBILL_SIZE_LABELS[v] }),
);

export default function WaybillTemplatesPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<WaybillTemplateRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [settingDefaultId, setSettingDefaultId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: WaybillTemplateRow | null }>({
    open: false,
  });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listWaybillTemplates());
    } catch (e) {
      setLoadError((e as Error).message || '加载面单模板失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: WaybillTemplateRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(
      row
        ? {
            name: row.name,
            sizeCode: row.sizeCode,
            sections: [
              ...(row.showRecipient ? ['recipient'] : []),
              ...(row.showSender ? ['sender'] : []),
              ...(row.showItems ? ['items'] : []),
              ...(row.showRemark ? ['remark'] : []),
              ...(row.showCarrierLogo ? ['logo'] : []),
            ],
            headerText: row.headerText,
            footerText: row.footerText,
            sortOrder: row.sortOrder,
          }
        : {
            name: '',
            sizeCode: '100x180',
            sections: ['recipient', 'sender', 'items', 'remark'],
            headerText: '',
            footerText: '',
            sortOrder: 0,
          },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    const sections: string[] = v.sections || [];
    const body = {
      name: v.name,
      sizeCode: v.sizeCode,
      showRecipient: sections.includes('recipient'),
      showSender: sections.includes('sender'),
      showItems: sections.includes('items'),
      showRemark: sections.includes('remark'),
      showCarrierLogo: sections.includes('logo'),
      headerText: v.headerText || '',
      footerText: v.footerText || '',
      sortOrder: v.sortOrder ?? 0,
    };
    setSaving(true);
    try {
      if (modal.row) await updateWaybillTemplate(modal.row.id, body);
      else await createWaybillTemplate(body);
      message.success('已保存');
      setModal({ open: false });
      await load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const setDefault = async (row: WaybillTemplateRow) => {
    setSettingDefaultId(row.id);
    try {
      await updateWaybillTemplate(row.id, { isDefault: true });
      message.success(`已将「${row.name}」设为默认模板`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setSettingDefaultId('');
    }
  };

  const remove = async (row: WaybillTemplateRow) => {
    try {
      await deleteWaybillTemplate(row.id);
      message.success(`已删除 ${row.name}`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  const sectionTags = (row: WaybillTemplateRow) => {
    const tags: string[] = [];
    if (row.showRecipient) tags.push('收件人');
    if (row.showSender) tags.push('发件人');
    if (row.showItems) tags.push('商品明细');
    if (row.showRemark) tags.push('备注');
    if (row.showCarrierLogo) tags.push('物流商 logo 位');
    return tags;
  };

  return (
    <TmPageContainer
      title="面单模板"
      subTitle="自定义打印模板（非电子面单）：配置纸张尺寸、显示字段与页眉页脚，打印拣货/发货单时按所选模板渲染"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Tooltip title={readonly ? '只读账号不可新增模板' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => openModal()}>
              新增模板
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载面单模板失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<WaybillTemplateRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 860 }}
          locale={{ emptyText: '暂无面单模板，点击「新增模板」添加' }}
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              width: 200,
              render: (v: string, row) => (
                <Space size={4}>
                  <span>{v}</span>
                  {row.isDefault ? <Tag color="green">默认</Tag> : null}
                </Space>
              ),
            },
            {
              title: '尺寸',
              dataIndex: 'sizeCode',
              width: 170,
              render: (v: WaybillTemplateRow['sizeCode']) => (
                <Tag>{WAYBILL_SIZE_LABELS[v] || v}</Tag>
              ),
            },
            {
              title: '类型',
              dataIndex: 'isPreset',
              width: 90,
              render: (v: boolean) => (v ? <Tag color="blue">预置</Tag> : <Tag>自定义</Tag>),
            },
            {
              title: '显示字段',
              key: 'sections',
              render: (_, row) => (
                <Space size={4} wrap>
                  {sectionTags(row).map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </Space>
              ),
            },
            {
              title: '操作',
              width: 210,
              render: (_, row) => (
                <Space size={4}>
                  <Button
                    size="small"
                    type="link"
                    disabled={readonly || row.isDefault}
                    loading={settingDefaultId === row.id}
                    onClick={() => void setDefault(row)}
                  >
                    设为默认
                  </Button>
                  <Button
                    size="small"
                    type="link"
                    disabled={readonly}
                    onClick={() => openModal(row)}
                  >
                    编辑
                  </Button>
                  {row.isPreset ? (
                    <Tooltip title="预置模板不可删除，可修改字段配置">
                      <Button size="small" type="link" disabled>
                        删除
                      </Button>
                    </Tooltip>
                  ) : (
                    <Popconfirm
                      title={`删除模板「${row.name}」？`}
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
        title={modal.row ? `编辑模板：${modal.row.name}` : '新增面单模板'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="模板名称" rules={[{ required: true, message: '请填写模板名称' }]}>
            <Input placeholder="如：默认发货面单" />
          </Form.Item>
          <Form.Item name="sizeCode" label="纸张尺寸" rules={[{ required: true, message: '请选择纸张尺寸' }]}>
            <Select options={SIZE_OPTIONS} />
          </Form.Item>
          <Form.Item name="sections" label="显示字段">
            <Checkbox.Group
              options={[
                { value: 'recipient', label: '收件人' },
                { value: 'sender', label: '发件人' },
                { value: 'items', label: '商品明细' },
                { value: 'remark', label: '备注' },
                { value: 'logo', label: '物流商 logo 位' },
              ]}
            />
          </Form.Item>
          <Form.Item name="headerText" label="页眉文本（可选）">
            <Input placeholder="如：XX 旗舰店 发货单" maxLength={200} />
          </Form.Item>
          <Form.Item name="footerText" label="页脚文本（可选）">
            <Input placeholder="如：感谢惠顾，售后问题请联系在线客服" maxLength={200} />
          </Form.Item>
          <Form.Item name="sortOrder" label="排序（越小越靠前）" initialValue={0}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

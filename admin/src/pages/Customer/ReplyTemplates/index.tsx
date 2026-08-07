import { TmPageContainer } from '@/components/ui';
import {
  createReplyTemplate,
  deleteReplyTemplate,
  queryReplyTemplates,
  reorderReplyTemplates,
  updateReplyTemplate,
  TEMPLATE_LANGUAGES,
  templateLanguageLabel,
  type ReplyTemplateGroupKey,
  type ReplyTemplateRow,
} from '@/services/customer';
import { extractErrorMessage } from '@/utils/httpErrorCopy';
import {
  REPLY_TEMPLATE_GROUPS,
  REPLY_TEMPLATE_VAR_KEYS,
  replyTemplateGroupLabel,
} from '@/utils/replyTemplateVars';
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
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
  Switch,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

export default function ReplyTemplatesPage() {
  const [rows, setRows] = useState<ReplyTemplateRow[]>([]);
  const [canWrite, setCanWrite] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [group, setGroup] = useState<'all' | ReplyTemplateGroupKey>('all');
  const [keyword, setKeyword] = useState('');
  const [togglingId, setTogglingId] = useState('');
  const [movingId, setMovingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: ReplyTemplateRow | null }>({
    open: false,
  });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(
    async (g: 'all' | ReplyTemplateGroupKey, kw: string) => {
      setLoading(true);
      setLoadError('');
      try {
        const res = await queryReplyTemplates({
          group: g === 'all' ? undefined : g,
          keyword: kw || undefined,
        });
        setRows(res.list || []);
        setCanWrite(res.canWrite !== false);
      } catch (e) {
        setLoadError(extractErrorMessage(e, '加载话术模板失败'));
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    void load(group, keyword);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [group]);

  const openModal = (row?: ReplyTemplateRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(
      row
        ? {
            groupKey: row.groupKey,
            name: row.name,
            content: row.content,
            defaultLanguage: row.defaultLanguage || 'zh-CN',
            variants: row.variants || [],
          }
        : {
            groupKey: group === 'all' ? 'presale' : group,
            name: '',
            content: '',
            defaultLanguage: 'zh-CN',
            variants: [],
          },
    );
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      const body = { ...v, variants: v.variants || [] };
      if (modal.row) {
        await updateReplyTemplate(modal.row.id, body);
      } else {
        await createReplyTemplate(body);
      }
      message.success('已保存');
      setModal({ open: false });
      await load(group, keyword);
    } catch (e) {
      message.error(extractErrorMessage(e, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (row: ReplyTemplateRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateReplyTemplate(row.id, { enabled });
      message.success(enabled ? `已启用「${row.name}」` : `已停用「${row.name}」`);
      await load(group, keyword);
    } catch (e) {
      message.error(extractErrorMessage(e, '操作失败'));
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: ReplyTemplateRow) => {
    try {
      await deleteReplyTemplate(row.id);
      message.success(`已删除「${row.name}」`);
      await load(group, keyword);
    } catch (e) {
      message.error(extractErrorMessage(e, '删除失败'));
    }
  };

  const move = async (row: ReplyTemplateRow, dir: -1 | 1) => {
    const groupRows = rows
      .filter((r) => r.groupKey === row.groupKey)
      .sort((a, b) => a.sortOrder - b.sortOrder);
    const idx = groupRows.findIndex((r) => r.id === row.id);
    const target = idx + dir;
    if (idx < 0 || target < 0 || target >= groupRows.length) return;
    const ids = groupRows.map((r) => r.id);
    [ids[idx], ids[target]] = [ids[target], ids[idx]];
    setMovingId(row.id);
    try {
      await reorderReplyTemplates({ groupKey: row.groupKey, ids });
      await load(group, keyword);
    } catch (e) {
      message.error(extractErrorMessage(e, '排序失败'));
    } finally {
      setMovingId('');
    }
  };

  return (
    <TmPageContainer
      title="话术模板"
      subTitle="维护客服快捷回复话术：按售前 / 售后 / 物流 / 退款分组管理，正文可用 {订单号}、{买家昵称} 等变量占位，会话回复框可一键插入并自动填充"
    >
      <Card>
        <Tabs
          activeKey={group}
          onChange={(k) => setGroup(k as 'all' | ReplyTemplateGroupKey)}
          items={[
            { key: 'all', label: '全部' },
            ...REPLY_TEMPLATE_GROUPS.map((g) => ({ key: g.key, label: g.label })),
          ]}
        />
        <Space style={{ marginBottom: 16 }} wrap>
          <Input.Search
            allowClear
            placeholder="按名称 / 内容搜索"
            style={{ width: 260 }}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onSearch={() => void load(group, keyword)}
          />
          <Tooltip title={canWrite ? '' : '当前账号无客服操作权限，不可新增话术模板'}>
            <Button type="primary" disabled={!canWrite} onClick={() => openModal()}>
              新增话术模板
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载话术模板失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load(group, keyword)}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<ReplyTemplateRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={false}
          scroll={{ x: 760 }}
          locale={{ emptyText: '暂无话术模板，点击「新增话术模板」添加' }}
          columns={[
            { title: '名称', dataIndex: 'name', width: 200, ellipsis: true },
            {
              title: '分组',
              dataIndex: 'groupKey',
              width: 90,
              render: (v: string) => <Tag>{replyTemplateGroupLabel(v)}</Tag>,
            },
            {
              title: '内容',
              dataIndex: 'content',
              ellipsis: true,
              render: (v: string) => (
                <Tooltip title={v} placement="topLeft">
                  <span>{v}</span>
                </Tooltip>
              ),
            },
            {
              title: '语言',
              width: 170,
              render: (_, row) => (
                <Space size={4} wrap>
                  <Tag color="blue">{templateLanguageLabel(row.defaultLanguage || 'zh-CN')}</Tag>
                  {(row.variants || []).map((v) => (
                    <Tag key={v.language}>{templateLanguageLabel(v.language)}</Tag>
                  ))}
                </Space>
              ),
            },
            {
              title: '排序',
              width: 100,
              render: (_, row) => (
                <Space size={0}>
                  <Button
                    size="small"
                    type="text"
                    aria-label={`上移「${row.name}」`}
                    icon={<ArrowUpOutlined />}
                    disabled={!canWrite || movingId !== ''}
                    onClick={() => void move(row, -1)}
                  />
                  <Button
                    size="small"
                    type="text"
                    aria-label={`下移「${row.name}」`}
                    icon={<ArrowDownOutlined />}
                    disabled={!canWrite || movingId !== ''}
                    onClick={() => void move(row, 1)}
                  />
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
                  disabled={!canWrite}
                  loading={togglingId === row.id}
                  onChange={(checked) => void toggleEnabled(row, checked)}
                />
              ),
            },
            {
              title: '操作',
              width: 130,
              render: (_, row) => (
                <Space size={4}>
                  <Button size="small" type="link" disabled={!canWrite} onClick={() => openModal(row)}>
                    编辑
                  </Button>
                  <Popconfirm
                    title={`删除话术模板「${row.name}」？`}
                    okText="删除"
                    okButtonProps={{ danger: true }}
                    disabled={!canWrite}
                    onConfirm={() => void remove(row)}
                  >
                    <Button size="small" type="link" danger disabled={!canWrite}>
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
        title={modal.row ? `编辑话术模板：${modal.row.name}` : '新增话术模板'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item name="groupKey" label="分组" rules={[{ required: true, message: '请选择分组' }]}>
            <Select
              options={REPLY_TEMPLATE_GROUPS.map((g) => ({ value: g.key, label: g.label }))}
              placeholder="请选择分组"
            />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="如：物流-查询进度" />
          </Form.Item>
          <Form.Item
            name="defaultLanguage"
            label="默认语言"
            tooltip="下方「内容」为默认语言正文；无法推断买家语言时回退到默认语言"
            rules={[{ required: true, message: '请选择默认语言' }]}
          >
            <Select
              options={TEMPLATE_LANGUAGES.map((l) => ({ value: l.key, label: l.label }))}
              placeholder="请选择默认语言"
            />
          </Form.Item>
          <Form.Item
            name="content"
            label="内容（默认语言）"
            rules={[{ required: true, message: '请填写内容' }]}
            extra={`支持变量占位：${REPLY_TEMPLATE_VAR_KEYS.map((k) => `{${k}}`).join('、')}，插入时按会话上下文自动填充`}
          >
            <Input.TextArea rows={5} placeholder="如：您好{买家昵称}，您的订单 {订单号} 已发货…" />
          </Form.Item>
          <Form.List name="variants">
            {(fields, { add, remove: removeField }) => (
              <>
                <Form.Item label="语言变体" style={{ marginBottom: 8 }} tooltip="同一模板按语言维护多份内容，变量占位符口径不变；正文可为外语">
                  {fields.length === 0 ? (
                    <Typography.Text type="secondary">暂无语言变体，点击下方添加</Typography.Text>
                  ) : null}
                  {fields.map((field) => (
                    <div
                      key={field.key}
                      style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginBottom: 8 }}
                    >
                      <Form.Item
                        name={[field.name, 'language']}
                        rules={[{ required: true, message: '请选择语言' }]}
                        style={{ marginBottom: 0, width: 140, flexShrink: 0 }}
                      >
                        <Select
                          placeholder="语言"
                          options={TEMPLATE_LANGUAGES.map((l) => ({ value: l.key, label: l.label }))}
                        />
                      </Form.Item>
                      <Form.Item
                        name={[field.name, 'content']}
                        rules={[{ required: true, message: '请填写该语言内容' }]}
                        style={{ marginBottom: 0, flex: 1 }}
                      >
                        <Input.TextArea rows={3} placeholder="该语言的模板正文，变量占位符与默认内容一致，如 {订单号}" />
                      </Form.Item>
                      <Button
                        type="text"
                        danger
                        aria-label="删除该语言变体"
                        icon={<DeleteOutlined />}
                        onClick={() => removeField(field.name)}
                      />
                    </div>
                  ))}
                  <Button
                    type="dashed"
                    block
                    icon={<PlusOutlined />}
                    onClick={() => add({ language: undefined, content: '' })}
                  >
                    添加语言变体
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

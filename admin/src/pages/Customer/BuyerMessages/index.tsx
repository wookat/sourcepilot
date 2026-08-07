import { TmPageContainer } from '@/components/ui';
import {
  BUYER_MSG_DRAFT_STATUSES,
  BUYER_MSG_NODES,
  batchMarkBuyerMsgDraftsSent,
  buyerMsgDraftStatusLabel,
  buyerMsgNodeLabel,
  createBuyerMsgRule,
  deleteBuyerMsgRule,
  estimateBuyerMsgBackfill,
  generateBuyerMsgDrafts,
  ignoreBuyerMsgDraft,
  markBuyerMsgDraftSent,
  queryBuyerMsgDrafts,
  queryBuyerMsgRules,
  queryReplyTemplates,
  regenerateBuyerMsgDraft,
  TEMPLATE_LANGUAGES,
  templateLanguageLabel,
  updateBuyerMsgDraft,
  updateBuyerMsgRule,
  type BuyerMsgDraftRow,
  type BuyerMsgNode,
  type BuyerMsgRuleBody,
  type BuyerMsgRuleRow,
  type ReplyTemplateRow,
} from '@/services/customer';
import { queryShops, type ShopListRow } from '@/services/shops';
import { extractErrorMessage } from '@/utils/httpErrorCopy';
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
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from '@umijs/max';

const NODE_TAG_COLORS: Record<string, string> = {
  paid: 'blue',
  shipped: 'geekblue',
  delivered: 'green',
  logistics_exception: 'orange',
  refunded: 'red',
};

const STATUS_TAG_COLORS: Record<string, string> = {
  pending: 'processing',
  sent: 'success',
  ignored: 'default',
};

/** 目标语言来源标注（与后端 langSource 口径一致） */
const LANG_SOURCE_LABELS: Record<string, string> = {
  order_country: '按收货地推断',
  shop_language: '按店铺语言配置',
  platform: '按店铺平台推断',
  fallback: '无法推断，已回退默认语言',
  no_variant: '缺该语言变体，已回退默认语言',
  manual: '人工切换',
};

const LANG_FALLBACK_SOURCES = ['fallback', 'no_variant'];

function DraftsTab({ shops }: { shops: ShopListRow[] }) {
  const [rows, setRows] = useState<BuyerMsgDraftRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [canWrite, setCanWrite] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [node, setNode] = useState<string>('');
  const [status, setStatus] = useState<string>('pending');
  const [shopId, setShopId] = useState<string>('');
  const [keyword, setKeyword] = useState('');
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [actingId, setActingId] = useState('');
  const [batching, setBatching] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [editModal, setEditModal] = useState<{ open: boolean; row?: BuyerMsgDraftRow | null }>({
    open: false,
  });
  const [savingEdit, setSavingEdit] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const [regenLang, setRegenLang] = useState('');
  const [editForm] = Form.useForm();

  const load = useCallback(
    async (p = page, ps = pageSize) => {
      setLoading(true);
      setLoadError('');
      try {
        const res = await queryBuyerMsgDrafts({
          page: p,
          pageSize: ps,
          node: node || undefined,
          status: status || undefined,
          shopId: shopId || undefined,
          keyword: keyword || undefined,
        });
        setRows(res.list || []);
        setTotal(res.total || 0);
        setCanWrite(res.canWrite !== false);
        setSelectedIds([]);
      } catch (e) {
        setLoadError(extractErrorMessage(e, '加载待发消息失败'));
      } finally {
        setLoading(false);
      }
    },
    [page, pageSize, node, status, shopId, keyword],
  );

  useEffect(() => {
    void load(1, pageSize);
    setPage(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node, status, shopId]);

  const markSent = async (row: BuyerMsgDraftRow) => {
    setActingId(row.id);
    try {
      await markBuyerMsgDraftSent(row.id);
      message.success(`订单 ${row.orderNo} 的消息已标记为已发送`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '标记失败'));
    } finally {
      setActingId('');
    }
  };

  const ignore = async (row: BuyerMsgDraftRow) => {
    setActingId(row.id);
    try {
      await ignoreBuyerMsgDraft(row.id);
      message.success(`订单 ${row.orderNo} 的消息草稿已忽略`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '忽略失败'));
    } finally {
      setActingId('');
    }
  };

  const batchMarkSent = async () => {
    setBatching(true);
    try {
      const res = await batchMarkBuyerMsgDraftsSent(selectedIds);
      message.success(`已批量标记 ${res.updated} 条为已发送${res.skipped ? `，跳过 ${res.skipped} 条` : ''}`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '批量标记失败'));
    } finally {
      setBatching(false);
    }
  };

  const generateNow = async () => {
    setGenerating(true);
    try {
      const res = await generateBuyerMsgDrafts();
      message.success(res.created > 0 ? `本次扫描新生成 ${res.created} 条待发草稿` : '本次扫描没有新的待发草稿');
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '生成草稿失败'));
    } finally {
      setGenerating(false);
    }
  };

  const openEdit = (row: BuyerMsgDraftRow) => {
    setEditModal({ open: true, row });
    setRegenLang(row.language || 'zh-CN');
    editForm.setFieldsValue({ content: row.content });
  };

  const regenerate = async () => {
    const row = editModal.row;
    if (!row || !regenLang) return;
    setRegenerating(true);
    try {
      const updated = await regenerateBuyerMsgDraft(row.id, regenLang);
      editForm.setFieldsValue({ content: updated.content });
      setEditModal({ open: true, row: updated });
      message.success(`已按「${templateLanguageLabel(regenLang)}」重新生成草稿内容`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '重新生成失败'));
    } finally {
      setRegenerating(false);
    }
  };

  const submitEdit = async () => {
    const v = await editForm.validateFields().catch(() => null);
    if (!v || !editModal.row) return;
    setSavingEdit(true);
    try {
      await updateBuyerMsgDraft(editModal.row.id, v.content);
      message.success('草稿内容已保存');
      await load();
      setEditModal({ open: false });
    } catch (e) {
      message.error(extractErrorMessage(e, '保存失败'));
    } finally {
      setSavingEdit(false);
    }
  };

  const pendingSelected = selectedIds.length > 0;

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          allowClear
          placeholder="按节点筛选"
          style={{ width: 140 }}
          value={node || undefined}
          onChange={(v) => setNode(v || '')}
          options={BUYER_MSG_NODES.map((n) => ({ value: n.key, label: n.label }))}
        />
        <Select
          allowClear
          placeholder="按状态筛选"
          style={{ width: 140 }}
          value={status || undefined}
          onChange={(v) => setStatus(v || '')}
          options={BUYER_MSG_DRAFT_STATUSES.map((s) => ({ value: s.key, label: s.label }))}
        />
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="按店铺筛选"
          style={{ width: 200 }}
          value={shopId || undefined}
          onChange={(v) => setShopId(v || '')}
          options={shops.map((s) => ({ value: s.id, label: s.shopName }))}
        />
        <Input.Search
          allowClear
          placeholder="按订单号 / 买家 / 内容搜索"
          style={{ width: 240 }}
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          onSearch={() => void load(1, pageSize)}
        />
        <Tooltip title={canWrite ? '' : '当前账号无客服操作权限，不可批量标记'}>
          <Button
            type="primary"
            disabled={!canWrite || !pendingSelected}
            loading={batching}
            onClick={() => void batchMarkSent()}
          >
            批量标记已发送{pendingSelected ? `（${selectedIds.length}）` : ''}
          </Button>
        </Tooltip>
        <Tooltip title={canWrite ? '按启用规则立即扫描订单节点并生成缺失草稿' : '当前账号无客服操作权限，不可触发生成'}>
          <Button disabled={!canWrite} loading={generating} onClick={() => void generateNow()}>
            立即生成草稿
          </Button>
        </Tooltip>
      </Space>
      {loadError ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="加载待发消息失败"
          description={loadError}
          action={
            <Button size="small" onClick={() => void load()}>
              重试
            </Button>
          }
        />
      ) : null}
      <Table<BuyerMsgDraftRow>
        rowKey="id"
        size="middle"
        loading={loading}
        dataSource={rows}
        scroll={{ x: 980 }}
        locale={{ emptyText: '暂无待发消息草稿：启用节点规则后，系统会按订单节点自动生成' }}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys) => setSelectedIds(keys as string[]),
          getCheckboxProps: (row) => ({
            disabled: !canWrite || row.status !== 'pending',
            'aria-label': `选择订单 ${row.orderNo} 的草稿`,
          }),
        }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
            void load(p, ps);
          },
        }}
        columns={[
          {
            title: '订单',
            dataIndex: 'orderNo',
            width: 170,
            render: (v: string, row) => (
              <Space direction="vertical" size={0}>
                <Link to={`/orders/${row.orderId}`}>{v}</Link>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {row.customerName}
                </Typography.Text>
              </Space>
            ),
          },
          {
            title: '节点',
            dataIndex: 'node',
            width: 100,
            render: (v: string) => <Tag color={NODE_TAG_COLORS[v]}>{buyerMsgNodeLabel(v)}</Tag>,
          },
          { title: '店铺', dataIndex: 'shopName', width: 140, ellipsis: true },
          {
            title: '消息内容',
            dataIndex: 'content',
            render: (v: string, row) => (
              <Space direction="vertical" size={0} style={{ maxWidth: '100%' }}>
                <Tooltip title={v} placement="topLeft">
                  <Typography.Text ellipsis style={{ maxWidth: 360 }}>
                    {v}
                  </Typography.Text>
                </Tooltip>
                {row.missingVars?.length ? (
                  <Typography.Text type="warning" style={{ fontSize: 12 }}>
                    缺少变量：{row.missingVars.map((m) => `{${m}}`).join('、')}，发送前请补全
                  </Typography.Text>
                ) : null}
              </Space>
            ),
          },
          {
            title: '语言',
            dataIndex: 'language',
            width: 130,
            render: (v: string, row) => (
              <Space direction="vertical" size={0}>
                <Tooltip title={LANG_SOURCE_LABELS[row.langSource] || ''}>
                  <Tag color={LANG_FALLBACK_SOURCES.includes(row.langSource) ? 'orange' : 'blue'}>
                    {templateLanguageLabel(v || 'zh-CN')}
                  </Tag>
                </Tooltip>
                {LANG_FALLBACK_SOURCES.includes(row.langSource) ? (
                  <Typography.Text type="warning" style={{ fontSize: 12 }}>
                    {LANG_SOURCE_LABELS[row.langSource]}
                  </Typography.Text>
                ) : null}
              </Space>
            ),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (v: string) => <Tag color={STATUS_TAG_COLORS[v]}>{buyerMsgDraftStatusLabel(v)}</Tag>,
          },
          {
            title: '操作',
            width: 220,
            render: (_, row) => (
              <Space size={4} wrap>
                {row.conversationId ? (
                  <Link to={`/customer/conversations/${row.conversationId}`}>会话</Link>
                ) : null}
                <Button
                  size="small"
                  type="link"
                  disabled={!canWrite || row.status !== 'pending'}
                  onClick={() => openEdit(row)}
                >
                  编辑
                </Button>
                <Popconfirm
                  title={`确认已在平台后台向买家发送订单 ${row.orderNo} 的消息？`}
                  okText="已发送"
                  disabled={!canWrite || row.status !== 'pending'}
                  onConfirm={() => void markSent(row)}
                >
                  <Button
                    size="small"
                    type="link"
                    disabled={!canWrite || row.status !== 'pending'}
                    loading={actingId === row.id}
                  >
                    标记已发送
                  </Button>
                </Popconfirm>
                <Popconfirm
                  title={`忽略订单 ${row.orderNo} 的消息草稿？忽略后不会重新生成`}
                  okText="忽略"
                  okButtonProps={{ danger: true }}
                  disabled={!canWrite || row.status !== 'pending'}
                  onConfirm={() => void ignore(row)}
                >
                  <Button size="small" type="link" danger disabled={!canWrite || row.status !== 'pending'}>
                    忽略
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={editModal.row ? `编辑待发消息：订单 ${editModal.row.orderNo}` : '编辑待发消息'}
        open={editModal.open}
        confirmLoading={savingEdit}
        onCancel={() => setEditModal({ open: false })}
        onOk={() => void submitEdit()}
        forceRender
      >
        <Form form={editForm} layout="vertical">
          <Form.Item
            label="语言变体"
            extra="切换语言后点击「重新生成」，会用该语言的模板变体重建草稿内容（仅改草稿，不会向买家发送）"
          >
            <Space wrap>
              <Select
                style={{ width: 160 }}
                value={regenLang || undefined}
                onChange={(v) => setRegenLang(v)}
                options={TEMPLATE_LANGUAGES.map((l) => ({ value: l.key, label: l.label }))}
                placeholder="选择语言"
              />
              <Button loading={regenerating} disabled={!canWrite} onClick={() => void regenerate()}>
                按所选语言重新生成
              </Button>
              {editModal.row?.langSource ? (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  当前：{templateLanguageLabel(editModal.row.language || 'zh-CN')}（
                  {LANG_SOURCE_LABELS[editModal.row.langSource] || editModal.row.langSource}）
                </Typography.Text>
              ) : null}
            </Space>
          </Form.Item>
          <Form.Item
            name="content"
            label="消息内容"
            rules={[{ required: true, message: '请填写消息内容' }]}
            extra="编辑后请在对应平台后台发送给买家，再回到本页标记为已发送"
          >
            <Input.TextArea rows={5} placeholder="待发送给买家的消息内容" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

function RulesTab({ shops }: { shops: ShopListRow[] }) {
  const [rows, setRows] = useState<BuyerMsgRuleRow[]>([]);
  const [canWrite, setCanWrite] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [templates, setTemplates] = useState<ReplyTemplateRow[]>([]);
  const [togglingId, setTogglingId] = useState('');
  const [modal, setModal] = useState<{ open: boolean; row?: BuyerMsgRuleRow | null }>({ open: false });
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const platformOptions = useMemo(() => {
    const set = new Set(shops.map((s) => s.platform));
    return Array.from(set).map((p) => ({ value: p, label: p }));
  }, [shops]);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const [ruleRes, tplRes] = await Promise.all([
        queryBuyerMsgRules(),
        queryReplyTemplates({ enabled: true }),
      ]);
      setRows(ruleRes.list || []);
      setCanWrite(ruleRes.canWrite !== false);
      setTemplates(tplRes.list || []);
    } catch (e) {
      setLoadError(extractErrorMessage(e, '加载自动消息规则失败'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: BuyerMsgRuleRow) => {
    setModal({ open: true, row });
    form.setFieldsValue(
      row
        ? {
            name: row.name,
            node: row.node,
            // 原模板已删除时不预填，强制重新选择可用模板
            templateId: row.templateMissing ? undefined : row.templateId,
            platforms: row.platforms,
            shopIds: row.shopIds,
            backfill: !!row.backfill,
          }
        : { name: '', node: undefined, templateId: undefined, platforms: [], shopIds: [], backfill: false },
    );
  };

  const save = async (v: BuyerMsgRuleBody & { node: BuyerMsgNode }) => {
    setSaving(true);
    try {
      if (modal.row) {
        await updateBuyerMsgRule(modal.row.id, v);
      } else {
        await createBuyerMsgRule(v);
      }
      message.success('已保存');
      setModal({ open: false });
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const turningOnBackfill = v.backfill && !(modal.row && modal.row.backfill);
    if (!turningOnBackfill) {
      await save(v);
      return;
    }
    // 开启回溯存量：先预估将生成的草稿数量，经操作人确认后再保存。
    setSaving(true);
    let estimated = 0;
    try {
      const res = await estimateBuyerMsgBackfill({
        node: v.node,
        platforms: v.platforms,
        shopIds: v.shopIds,
      });
      estimated = res.estimated;
    } catch (e) {
      message.error(extractErrorMessage(e, '预估回溯数量失败'));
      setSaving(false);
      return;
    }
    setSaving(false);
    Modal.confirm({
      title: '确认回溯存量订单？',
      content: `开启后预计将为约 ${estimated} 笔存量订单生成待发草稿（仍需人工确认后才会发送）。不开启则只对规则生效后的新订单事件生成草稿。`,
      okText: `确认回溯（约 ${estimated} 条）`,
      cancelText: '取消',
      onOk: () => save(v),
    });
  };

  const toggleEnabled = async (row: BuyerMsgRuleRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateBuyerMsgRule(row.id, { enabled });
      message.success(enabled ? `已启用「${row.name}」` : `已停用「${row.name}」`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '操作失败'));
    } finally {
      setTogglingId('');
    }
  };

  const remove = async (row: BuyerMsgRuleRow) => {
    try {
      await deleteBuyerMsgRule(row.id);
      message.success(`已删除「${row.name}」`);
      await load();
    } catch (e) {
      message.error(extractErrorMessage(e, '删除失败'));
    }
  };

  const shopName = (id: string) => shops.find((s) => s.id === id)?.shopName || id;

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <Tooltip title={canWrite ? '' : '当前账号无客服操作权限，不可新增规则'}>
          <Button type="primary" disabled={!canWrite} onClick={() => openModal()}>
            新增节点规则
          </Button>
        </Tooltip>
      </Space>
      {loadError ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="加载自动消息规则失败"
          description={loadError}
          action={
            <Button size="small" onClick={() => void load()}>
              重试
            </Button>
          }
        />
      ) : null}
      <Table<BuyerMsgRuleRow>
        rowKey="id"
        size="middle"
        loading={loading}
        dataSource={rows}
        pagination={false}
        scroll={{ x: 860 }}
        locale={{ emptyText: '暂无节点规则，点击「新增节点规则」配置订单节点自动生成买家消息草稿' }}
        columns={[
          { title: '规则名称', dataIndex: 'name', width: 200, ellipsis: true },
          {
            title: '订单节点',
            dataIndex: 'node',
            width: 110,
            render: (v: string) => <Tag color={NODE_TAG_COLORS[v]}>{buyerMsgNodeLabel(v)}</Tag>,
          },
          {
            title: '话术模板',
            dataIndex: 'templateName',
            width: 180,
            ellipsis: true,
            render: (v: string, row) =>
              row.templateMissing ? (
                <Tooltip title="该规则引用的话术模板已被删除，不会再生成草稿；请编辑规则重新选择模板">
                  <Tag color="red">模板已删除</Tag>
                </Tooltip>
              ) : (
                v || '—'
              ),
          },
          {
            title: '平台过滤',
            dataIndex: 'platforms',
            width: 140,
            render: (v: string[]) =>
              v?.length ? v.map((p) => <Tag key={p}>{p}</Tag>) : <Typography.Text type="secondary">全部平台</Typography.Text>,
          },
          {
            title: '店铺过滤',
            dataIndex: 'shopIds',
            width: 180,
            render: (v: string[]) =>
              v?.length ? (
                v.map((id) => (
                  <Tag key={id} style={{ maxWidth: 160 }}>
                    {shopName(id)}
                  </Tag>
                ))
              ) : (
                <Typography.Text type="secondary">全部店铺</Typography.Text>
              ),
          },
          {
            title: '生效范围',
            dataIndex: 'backfill',
            width: 120,
            render: (v: boolean) =>
              v ? (
                <Tooltip title="已开启回溯：对全部存量订单生成草稿">
                  <Tag color="orange">回溯存量</Tag>
                </Tooltip>
              ) : (
                <Tooltip title="仅对规则生效后的新订单节点事件生成草稿，不回溯存量订单">
                  <Tag color="green">仅新订单</Tag>
                </Tooltip>
              ),
          },
          {
            title: '启用',
            dataIndex: 'enabled',
            width: 110,
            render: (v: boolean, row) => (
              <Space size={4}>
                <Tooltip
                  title={
                    row.templateMissing
                      ? v
                        ? '模板已删除，该规则已不再生成草稿；可停用或编辑规则重新选择模板'
                        : '模板已删除，请先编辑规则重新选择模板后再启用'
                      : ''
                  }
                >
                  <Switch
                    checked={v}
                    size="small"
                    disabled={!canWrite || (row.templateMissing && !v)}
                    loading={togglingId === row.id}
                    onChange={(checked) => void toggleEnabled(row, checked)}
                  />
                </Tooltip>
                {row.templateMissing && v ? <Tag color="red">已失效</Tag> : null}
              </Space>
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
                  title={`删除规则「${row.name}」？已生成的草稿不受影响`}
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
      <Modal
        title={modal.row ? `编辑节点规则：${modal.row.name}` : '新增节点规则'}
        open={modal.open}
        confirmLoading={saving}
        onCancel={() => setModal({ open: false })}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          {modal.row?.templateMissing ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="原话术模板已被删除"
              description="该规则当前不会生成草稿，请重新选择一个可用的话术模板后保存"
            />
          ) : null}
          <Form.Item name="name" label="规则名称" rules={[{ required: true, message: '请填写规则名称' }]}>
            <Input placeholder="如：发货后自动通知买家" />
          </Form.Item>
          <Form.Item name="node" label="订单节点" rules={[{ required: true, message: '请选择订单节点' }]}>
            <Select
              placeholder="请选择订单节点"
              options={BUYER_MSG_NODES.map((n) => ({ value: n.key, label: n.label }))}
            />
          </Form.Item>
          <Form.Item
            name="templateId"
            label="话术模板"
            rules={[{ required: true, message: '请选择话术模板' }]}
            extra="复用话术模板变量占位，生成草稿时按订单 / 物流上下文自动填充"
          >
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="请选择话术模板"
              options={templates.map((t) => ({ value: t.id, label: t.name }))}
            />
          </Form.Item>
          <Form.Item name="platforms" label="平台过滤" extra="留空表示全部平台生效">
            <Select mode="multiple" allowClear placeholder="全部平台" options={platformOptions} />
          </Form.Item>
          <Form.Item name="shopIds" label="店铺过滤" extra="留空表示全部店铺生效">
            <Select
              mode="multiple"
              allowClear
              showSearch
              optionFilterProp="label"
              placeholder="全部店铺"
              options={shops.map((s) => ({ value: s.id, label: s.shopName }))}
            />
          </Form.Item>
          <Form.Item
            name="backfill"
            label="回溯存量订单"
            valuePropName="checked"
            extra="默认关闭：规则只对生效后的新订单节点事件生成草稿，不回溯存量订单；开启时会先预估将生成的数量并需确认"
          >
            <Switch data-testid="buyer-msg-backfill-switch" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

export default function BuyerMessagesPage() {
  const [shops, setShops] = useState<ShopListRow[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 100 });
        setShops(res.list || []);
      } catch {
        setShops([]);
      }
    })();
  }, []);

  return (
    <TmPageContainer
      title="买家自动消息"
      subTitle="按订单节点（已付款 / 已发货 / 已签收 / 物流异常 / 退款）自动生成买家消息草稿，人工确认后在平台后台发送"
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="当前不会自动发送到平台"
        description="平台消息通道尚未接入：系统只按节点规则自动生成待发送草稿，绝不自动外发。请复制草稿内容，在对应平台商家后台发送给买家后，再回到本页标记为已发送。通道接入后此流程可无缝切换为自动发送。"
      />
      <Card>
        <Tabs
          defaultActiveKey="drafts"
          items={[
            { key: 'drafts', label: '待发消息', children: <DraftsTab shops={shops} /> },
            { key: 'rules', label: '节点规则', children: <RulesTab shops={shops} /> },
          ]}
        />
      </Card>
    </TmPageContainer>
  );
}

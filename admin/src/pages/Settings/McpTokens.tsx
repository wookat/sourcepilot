import { TmPageContainer } from '@/components/ui';
import {
  createMcpToken,
  createMcpWriteToken,
  listMcpAuditLogs,
  listMcpTokens,
  revokeMcpToken,
  type McpAuditLogRow,
  type McpTokenRow,
} from '@/services/mcpTokens';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { formatDateTime } from '@/utils/formatTime';
import { isReadonly, normalizeRole, ROLES } from '@/utils/permission';
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
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';

const EXPIRY_OPTIONS = [
  { value: 0, label: '不过期' },
  { value: 7, label: '7 天' },
  { value: 30, label: '30 天' },
  { value: 90, label: '90 天' },
  { value: 180, label: '180 天' },
  { value: 365, label: '365 天' },
];

const EXPIRING_SOON_MS = 7 * 24 * 60 * 60 * 1000;

const PURPOSE_OPTIONS = [
  { value: 'mcp', label: 'MCP 只读' },
  { value: 'openapi', label: '开放 API' },
  { value: 'both', label: 'MCP + 开放 API' },
];

const PURPOSE_LABELS: Record<string, string> = {
  mcp: 'MCP 只读',
  openapi: '开放 API',
  both: 'MCP + 开放 API',
};

const MCP_TOOL_OPTIONS = [
  'orders_query',
  'inventory_query',
  'report_summary',
  'exceptions_pending',
  'mcp:auth',
  'openapi:auth',
];

const MCP_WRITE_TOOL_OPTIONS = [
  'orders_add_tag',
  'orders_remove_tag',
  'exceptions_mark',
  'procurement_mark_placed',
  'procurement_fill_logistics',
];

const WRITE_EXPIRY_OPTIONS = [
  { value: 0, label: '默认（30 天）' },
  { value: 7, label: '7 天' },
  { value: 30, label: '30 天' },
  { value: 90, label: '90 天' },
];

const WRITE_MODE_TAGS: Record<string, { color: string; label: string }> = {
  dry_run: { color: 'blue', label: 'dry_run 预览' },
  execute: { color: 'volcano', label: 'execute 执行' },
};

function isWriteScope(scope: string): boolean {
  return scope
    .split(',')
    .map((s) => s.trim())
    .includes('write:ops');
}

const AUDIT_STATUS_TAGS: Record<string, { color: string; label: string }> = {
  success: { color: 'green', label: '成功' },
  error: { color: 'red', label: '失败' },
  auth_failed: { color: 'orange', label: '鉴权失败' },
  rate_limited: { color: 'gold', label: '已限流' },
};

const OPENAPI_ENDPOINT_OPTIONS = [
  'openapi:orders_list',
  'openapi:orders_detail',
  'openapi:inventory_list',
  'openapi:reports_summary',
  'openapi:exceptions_list',
];

function expiryCell(row: McpTokenRow) {
  if (!row.expiresAt) {
    return <Typography.Text type="secondary">不过期</Typography.Text>;
  }
  if (row.expired) {
    return (
      <Space size={4}>
        <Tag color="red">已过期</Tag>
        <Typography.Text type="secondary">{formatDateTime(row.expiresAt)}</Typography.Text>
      </Space>
    );
  }
  const soon = new Date(row.expiresAt).getTime() - Date.now() <= EXPIRING_SOON_MS;
  return (
    <Space size={4}>
      {soon ? <Tag color="orange">即将过期</Tag> : null}
      <Typography.Text>{formatDateTime(row.expiresAt)}</Typography.Text>
    </Space>
  );
}

export default function McpTokensPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);
  const isAdmin = normalizeRole(initialState?.currentUser?.role) === ROLES.ADMIN;

  const [rows, setRows] = useState<McpTokenRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [plaintext, setPlaintext] = useState('');
  const [revokingId, setRevokingId] = useState('');
  const [form] = Form.useForm<{ name: string; purpose: string; expiresInDays: number }>();

  const [auditRows, setAuditRows] = useState<McpAuditLogRow[]>([]);
  const [auditTotal, setAuditTotal] = useState(0);
  const [auditLoading, setAuditLoading] = useState(true);
  const [auditError, setAuditError] = useState('');
  const [auditPage, setAuditPage] = useState(1);
  const [auditPageSize, setAuditPageSize] = useState(20);
  const [auditTool, setAuditTool] = useState<string>();
  const [auditStatus, setAuditStatus] = useState<string>();
  const auditSeqRef = useRef(0);

  const [writeEnabled, setWriteEnabled] = useState(false);
  const [writeGateLoading, setWriteGateLoading] = useState(false);
  const [writeGateSaving, setWriteGateSaving] = useState(false);
  const [writeGateConfirmOpen, setWriteGateConfirmOpen] = useState(false);
  const [writeCreateOpen, setWriteCreateOpen] = useState(false);
  const [writeSaving, setWriteSaving] = useState(false);
  const [writeForm] = Form.useForm<{ name: string; expiresInDays: number }>();

  const loadWriteGate = useCallback(async () => {
    if (!isAdmin) return;
    setWriteGateLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const row = items.find((i) => i.groupKey === 'mcp' && i.itemKey === 'write_enabled');
      setWriteEnabled((row?.itemValue || '').trim().toLowerCase() === 'true');
    } catch (e) {
      message.error((e as Error).message || '加载 MCP 写开关失败');
    } finally {
      setWriteGateLoading(false);
    }
  }, [isAdmin]);

  useEffect(() => {
    void loadWriteGate();
  }, [loadWriteGate]);

  const saveWriteGate = useCallback(
    async (next: boolean) => {
      setWriteGateSaving(true);
      try {
        await saveSettingsItems([
          {
            groupKey: 'mcp',
            itemKey: 'write_enabled',
            itemValue: next ? 'true' : 'false',
            isEncrypted: false,
            remark: 'MCP 写白名单租户开关',
          },
        ]);
        setWriteEnabled(next);
        message.success(next ? '已开启本租户 MCP 写白名单' : '已关闭本租户 MCP 写白名单');
      } catch (e) {
        message.error((e as Error).message || '保存失败');
      } finally {
        setWriteGateSaving(false);
      }
    },
    [],
  );

  const onToggleWriteGate = (next: boolean) => {
    if (!next) {
      void saveWriteGate(false);
      return;
    }
    setWriteGateConfirmOpen(true);
  };

  const submitWriteToken = async () => {
    const v = await writeForm.validateFields();
    setWriteSaving(true);
    try {
      const res = await createMcpWriteToken(v.name.trim(), v.expiresInDays);
      setWriteCreateOpen(false);
      writeForm.resetFields();
      setPlaintext(res.plaintext);
      await load();
    } catch (e) {
      message.error((e as Error).message || '创建失败');
    } finally {
      setWriteSaving(false);
    }
  };

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRows(await listMcpTokens());
    } catch (e) {
      setLoadError((e as Error).message || '加载 MCP token 失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // 并发请求只认最后一次：手动「刷新」与筛选/翻页触发的加载可能重叠，
  // 迟到的旧响应不得覆盖新数据（否则会闪回旧列表甚至「暂无数据」）。
  const loadAudits = useCallback(async () => {
    const seq = ++auditSeqRef.current;
    setAuditLoading(true);
    setAuditError('');
    try {
      const res = await listMcpAuditLogs({
        page: auditPage,
        pageSize: auditPageSize,
        tool: auditTool,
        status: auditStatus,
      });
      if (seq !== auditSeqRef.current) return;
      setAuditRows(res.items || []);
      setAuditTotal(res.total || 0);
    } catch (e) {
      if (seq !== auditSeqRef.current) return;
      setAuditError((e as Error).message || '加载审计日志失败');
    } finally {
      if (seq === auditSeqRef.current) setAuditLoading(false);
    }
  }, [auditPage, auditPageSize, auditTool, auditStatus]);

  useEffect(() => {
    void loadAudits();
  }, [loadAudits]);

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      const res = await createMcpToken(v.name.trim(), v.expiresInDays, v.purpose);
      setCreateOpen(false);
      form.resetFields();
      setPlaintext(res.plaintext);
      await load();
    } catch (e) {
      message.error((e as Error).message || '创建失败');
    } finally {
      setSaving(false);
    }
  };

  const revoke = async (row: McpTokenRow) => {
    setRevokingId(row.id);
    try {
      await revokeMcpToken(row.id);
      message.success(`已吊销 ${row.name}`);
      await load();
    } catch (e) {
      message.error((e as Error).message || '吊销失败');
    } finally {
      setRevokingId('');
    }
  };

  return (
    <TmPageContainer
      title="只读 API 接入（MCP / 开放 API）"
      subTitle="管理只读 API token：供 Claude 等 MCP 客户端与第三方系统（/api/open/v1/*）查询订单、库存、经营摘要与异常待办；token 仅创建时展示一次，只支持只读查询"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Tooltip title={readonly ? '只读账号不可创建 token' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => setCreateOpen(true)}>
              创建只读 token
            </Button>
          </Tooltip>
          <Typography.Text type="secondary">
            配置方法见{' '}
            <a href="/docs/mcp.md" target="_blank" rel="noopener noreferrer">
              docs/mcp.md
            </a>{' '}
            与{' '}
            <a href="/docs/open-api.md" target="_blank" rel="noopener noreferrer">
              docs/open-api.md
            </a>
            ；token 一旦泄露请立即吊销
          </Typography.Text>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载 MCP token 失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<McpTokenRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows.filter((r) => !isWriteScope(r.scope))}
          pagination={false}
          scroll={{ x: 'max-content' }}
          columns={[
            { title: '名称', dataIndex: 'name' },
            {
              title: '访问令牌（脱敏）',
              dataIndex: 'maskedToken',
              render: (v: string) => (
                <Typography.Text code style={{ whiteSpace: 'nowrap' }}>
                  {v}
                </Typography.Text>
              ),
            },
            {
              title: '权限',
              dataIndex: 'scope',
              render: (v: string) => <Tag color="blue">{v === 'readonly' ? '只读' : v}</Tag>,
            },
            {
              title: '用途',
              dataIndex: 'purpose',
              render: (v: string) => (
                <Tag color={v === 'openapi' ? 'purple' : v === 'both' ? 'geekblue' : 'cyan'}>
                  {PURPOSE_LABELS[v] || v}
                </Tag>
              ),
            },
            {
              title: '状态',
              dataIndex: 'revoked',
              render: (revoked: boolean, row: McpTokenRow) => {
                if (revoked) return <Tag color="red">已吊销</Tag>;
                if (row.expired) return <Tag color="red">已过期</Tag>;
                return <Tag color="green">有效</Tag>;
              },
            },
            {
              title: '过期时间',
              dataIndex: 'expiresAt',
              render: (_: unknown, row: McpTokenRow) => expiryCell(row),
            },
            {
              title: '创建时间',
              dataIndex: 'createdAt',
              render: (v?: string) => formatDateTime(v),
            },
            {
              title: '最近使用',
              dataIndex: 'lastUsedAt',
              render: (v?: string) => formatDateTime(v),
            },
            {
              title: '操作',
              key: 'actions',
              render: (_: unknown, row: McpTokenRow) =>
                row.revoked ? null : (
                  <Popconfirm
                    title={`确认吊销 ${row.name}？吊销后该 token 立即失效且不可恢复`}
                    onConfirm={() => void revoke(row)}
                    disabled={readonly}
                  >
                    <Button danger size="small" disabled={readonly} loading={revokingId === row.id}>
                      吊销
                    </Button>
                  </Popconfirm>
                ),
            },
          ]}
        />
      </Card>

      {isAdmin ? (
        <Card title="MCP 写白名单（write:ops，仅管理员）" style={{ marginTop: 16 }}>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="高风险能力：三层闸门默认全关"
            description="写操作需同时满足：① 服务端全局环境闸门 MCP_WRITE_ENABLED=true（默认关，需运维开启）；② 下方本租户开关（默认关）；③ 调用方持有 write:ops 写 token。每次写均强制 dry_run 影响预览 + 一次性确认 token，失败即拒绝的审计与限额兼具；消息发送等对外动作永不开放。写 token 仅管理员可见可管，默认 30 天过期，最长 90 天，不支持不过期。"
          />
          <Space style={{ marginBottom: 16 }} wrap>
            <span>本租户写开关（mcp / write_enabled）：</span>
            <Switch
              checked={writeEnabled}
              loading={writeGateLoading || writeGateSaving}
              checkedChildren="已开启"
              unCheckedChildren="已关闭"
              onChange={onToggleWriteGate}
            />
            <Button type="primary" danger onClick={() => setWriteCreateOpen(true)}>
              创建写 token
            </Button>
          </Space>
          <Table<McpTokenRow>
            rowKey="id"
            size="middle"
            loading={loading}
            dataSource={rows.filter((r) => isWriteScope(r.scope))}
            pagination={false}
            scroll={{ x: 'max-content' }}
            locale={{ emptyText: '暂无写 token' }}
            columns={[
              { title: '名称', dataIndex: 'name' },
              {
                title: '访问令牌（脱敏）',
                dataIndex: 'maskedToken',
                render: (v: string) => (
                  <Typography.Text code style={{ whiteSpace: 'nowrap' }}>
                    {v}
                  </Typography.Text>
                ),
              },
              {
                title: '权限',
                dataIndex: 'scope',
                render: (v: string) => <Tag color="volcano">{v}</Tag>,
              },
              {
                title: '状态',
                dataIndex: 'revoked',
                render: (revoked: boolean, row: McpTokenRow) => {
                  if (revoked) return <Tag color="red">已吊销</Tag>;
                  if (row.expired) return <Tag color="red">已过期</Tag>;
                  return <Tag color="green">有效</Tag>;
                },
              },
              {
                title: '过期时间',
                dataIndex: 'expiresAt',
                render: (_: unknown, row: McpTokenRow) => expiryCell(row),
              },
              {
                title: '创建时间',
                dataIndex: 'createdAt',
                render: (v?: string) => formatDateTime(v),
              },
              {
                title: '最近使用',
                dataIndex: 'lastUsedAt',
                render: (v?: string) => formatDateTime(v),
              },
              {
                title: '操作',
                key: 'actions',
                render: (_: unknown, row: McpTokenRow) =>
                  row.revoked ? null : (
                    <Popconfirm
                      title={`确认吊销写 token ${row.name}？吊销后立即失效且不可恢复`}
                      onConfirm={() => void revoke(row)}
                    >
                      <Button danger size="small" loading={revokingId === row.id}>
                        吊销
                      </Button>
                    </Popconfirm>
                  ),
              },
            ]}
          />
        </Card>
      ) : null}

      <Modal
        title="创建只读 token"
        open={createOpen}
        confirmLoading={saving}
        onOk={() => void submit()}
        onCancel={() => setCreateOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ purpose: 'mcp', expiresInDays: 0 }}>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
            extra="建议按用途命名，如 claude-desktop、mcp-inspector"
          >
            <Input maxLength={64} placeholder="如 claude-desktop" />
          </Form.Item>
          <Form.Item
            name="purpose"
            label="用途"
            extra="MCP 只读供 MCP 客户端使用；开放 API 供第三方系统调用 /api/open/v1/* 只读接口；两者均为只读"
          >
            <Select options={PURPOSE_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="expiresInDays"
            label="有效期"
            extra="到期后 token 自动失效，默认不过期；建议为对外接入设置有效期"
          >
            <Select options={EXPIRY_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="确认开启本租户的 MCP 写白名单？"
        open={writeGateConfirmOpen}
        okText="确认开启"
        okButtonProps={{ danger: true }}
        cancelText="取消"
        confirmLoading={writeGateSaving}
        onOk={async () => {
          await saveWriteGate(true);
          setWriteGateConfirmOpen(false);
        }}
        onCancel={() => setWriteGateConfirmOpen(false)}
      >
        开启后，持有 write:ops 写 token 的 MCP 客户端可对本租户执行白名单写操作（订单打标、异常标记、采购单
        mark-placed、运单号回填）。每次写均需 dry_run 预览 + 一次性确认
        token，并受限额与审计约束；但仍存在被滥用风险，请确保写 token
        妥善保管。另需服务端开启全局环境闸门 MCP_WRITE_ENABLED 才会生效。
      </Modal>

      <Modal
        title="创建写 token（write:ops）"
        open={writeCreateOpen}
        confirmLoading={writeSaving}
        okText="确认创建"
        okButtonProps={{ danger: true }}
        onOk={() => void submitWriteToken()}
        onCancel={() => setWriteCreateOpen(false)}
        destroyOnHidden
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="写 token 可对本租户执行白名单写操作"
          description="请仅为受控的 MCP 客户端创建，并设置尽量短的有效期；泄露请立即吊销。"
        />
        <Form form={writeForm} layout="vertical" initialValues={{ expiresInDays: 0 }}>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
            extra="建议按用途命名，如 claude-write、ops-agent"
          >
            <Input maxLength={64} placeholder="如 claude-write" />
          </Form.Item>
          <Form.Item
            name="expiresInDays"
            label="有效期"
            extra="写 token 必须过期：默认 30 天，最长 90 天，不支持不过期"
          >
            <Select options={WRITE_EXPIRY_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="Token 创建成功"
        open={!!plaintext}
        destroyOnHidden
        onOk={() => setPlaintext('')}
        onCancel={() => setPlaintext('')}
        footer={
          <Button type="primary" onClick={() => setPlaintext('')}>
            我已保存
          </Button>
        }
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="明文 token 仅展示这一次"
          description="请立即复制并妥善保存；关闭后无法再次查看，只能吊销后重新创建"
        />
        <Typography.Paragraph copyable={{ text: plaintext }}>
          <Typography.Text code>{plaintext}</Typography.Text>
        </Typography.Paragraph>
      </Modal>

      <Card title="MCP / 开放 API 调用审计日志" style={{ marginTop: 16 }}>
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            allowClear
            showSearch
            placeholder="工具 / 开放 API 接口"
            style={{ width: 220 }}
            value={auditTool}
            onChange={(v) => {
              setAuditPage(1);
              setAuditTool(v);
            }}
            options={[
              { label: 'MCP 工具', options: MCP_TOOL_OPTIONS.map((t) => ({ value: t, label: t })) },
              {
                label: 'MCP 写工具',
                options: MCP_WRITE_TOOL_OPTIONS.map((t) => ({ value: t, label: t })),
              },
              { label: '开放 API', options: OPENAPI_ENDPOINT_OPTIONS.map((t) => ({ value: t, label: t })) },
            ]}
          />
          <Select
            allowClear
            placeholder="结果状态"
            style={{ width: 140 }}
            value={auditStatus}
            onChange={(v) => {
              setAuditPage(1);
              setAuditStatus(v);
            }}
            options={Object.entries(AUDIT_STATUS_TAGS).map(([value, v]) => ({
              value,
              label: v.label,
            }))}
          />
          <Button onClick={() => void loadAudits()}>刷新</Button>
          <Typography.Text type="secondary">
            每次 MCP 工具调用与开放 API 调用（openapi: 前缀）各记录一条；鉴权失败与限流事件按来源每分钟至多一条；不记录查询参数与查询结果内容
          </Typography.Text>
        </Space>
        {auditError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载审计日志失败"
            description={auditError}
            action={
              <Button size="small" onClick={() => void loadAudits()}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<McpAuditLogRow>
          rowKey="id"
          size="middle"
          loading={auditLoading}
          dataSource={auditRows}
          pagination={{
            current: auditPage,
            pageSize: auditPageSize,
            total: auditTotal,
            showSizeChanger: true,
            onChange: (p, ps) => {
              setAuditPage(p);
              setAuditPageSize(ps);
            },
          }}
          columns={[
            {
              title: '时间',
              dataIndex: 'createdAt',
              render: (v?: string) => formatDateTime(v),
            },
            {
              title: '访问令牌',
              dataIndex: 'tokenName',
              render: (_: unknown, row: McpAuditLogRow) => (
                <Space size={4}>
                  <span>{row.tokenName}</span>
                  <Typography.Text code type="secondary">
                    {row.tokenMasked}
                  </Typography.Text>
                </Space>
              ),
            },
            {
              title: '工具',
              dataIndex: 'tool',
              render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
            },
            {
              title: '结果',
              dataIndex: 'status',
              render: (v: string) => {
                const tag = AUDIT_STATUS_TAGS[v] ?? AUDIT_STATUS_TAGS.error;
                return <Tag color={tag.color}>{tag.label}</Tag>;
              },
            },
            {
              title: '模式',
              dataIndex: 'mode',
              render: (v?: string) => {
                if (!v) return <Typography.Text type="secondary">—</Typography.Text>;
                const tag = WRITE_MODE_TAGS[v];
                return tag ? <Tag color={tag.color}>{tag.label}</Tag> : <Tag>{v}</Tag>;
              },
            },
            {
              title: '参数摘要',
              dataIndex: 'paramsSummary',
              render: (v?: string) =>
                v ? (
                  <Typography.Text code style={{ whiteSpace: 'nowrap' }}>
                    {v}
                  </Typography.Text>
                ) : (
                  <Typography.Text type="secondary">—</Typography.Text>
                ),
            },
            {
              title: '确认哈希',
              dataIndex: 'confirmHash',
              render: (v?: string) =>
                v ? (
                  <Tooltip title={v}>
                    <Typography.Text code>{v.slice(0, 12)}…</Typography.Text>
                  </Tooltip>
                ) : (
                  <Typography.Text type="secondary">—</Typography.Text>
                ),
            },
            { title: '耗时(ms)', dataIndex: 'durationMs' },
          ]}
        />
      </Card>
    </TmPageContainer>
  );
}

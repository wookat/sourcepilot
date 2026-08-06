import { TmPageContainer } from '@/components/ui';
import {
  createMcpToken,
  listMcpTokens,
  revokeMcpToken,
  type McpTokenRow,
} from '@/services/mcpTokens';
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
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

export default function McpTokensPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<McpTokenRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [plaintext, setPlaintext] = useState('');
  const [revokingId, setRevokingId] = useState('');
  const [form] = Form.useForm<{ name: string }>();

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

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      const res = await createMcpToken(v.name.trim());
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
      title="MCP 只读接入"
      subTitle="管理 MCP（Model Context Protocol）只读 API token：供 Claude 等 MCP 客户端查询订单、库存、经营摘要与异常待办；token 仅创建时展示一次，只支持只读查询"
    >
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Tooltip title={readonly ? '只读账号不可创建 token' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => setCreateOpen(true)}>
              创建只读 token
            </Button>
          </Tooltip>
          <Typography.Text type="secondary">
            配置方法见 docs/mcp.md；token 一旦泄露请立即吊销
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
          dataSource={rows}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            {
              title: '访问令牌（脱敏）',
              dataIndex: 'maskedToken',
              render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
            },
            {
              title: '权限',
              dataIndex: 'scope',
              render: (v: string) => <Tag color="blue">{v === 'readonly' ? '只读' : v}</Tag>,
            },
            {
              title: '状态',
              dataIndex: 'revoked',
              render: (revoked: boolean) =>
                revoked ? <Tag color="red">已吊销</Tag> : <Tag color="green">有效</Tag>,
            },
            { title: '创建时间', dataIndex: 'createdAt' },
            { title: '最近使用', dataIndex: 'lastUsedAt', render: (v?: string) => v || '-' },
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

      <Modal
        title="创建 MCP 只读 token"
        open={createOpen}
        confirmLoading={saving}
        onOk={() => void submit()}
        onCancel={() => setCreateOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
            extra="建议按用途命名，如 claude-desktop、mcp-inspector"
          >
            <Input maxLength={64} placeholder="如 claude-desktop" />
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
    </TmPageContainer>
  );
}

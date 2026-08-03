import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import { formatDateTime } from '@/utils/formatTime';
import {
  createPlatformTenant,
  disablePlatformTenant,
  enablePlatformTenant,
  fetchPlatformTenantPurge,
  fetchPlatformTenants,
  purgePlatformTenant,
  renamePlatformTenant,
  type PlatformTenantRow,
} from '@/services/platformTenants';
import { Alert, Button, Form, Input, Modal, Result, Space, Tag, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { isPlatformAdmin } from '@/utils/permission';

const PURGE_POLL_INTERVAL_MS = 3000;

export default function PlatformTenantsPage() {
  const actionRef = useRef<ActionType>();
  const { user, role } = usePermission();
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm();
  const [renameTarget, setRenameTarget] = useState<PlatformTenantRow | null>(null);
  const [renameForm] = Form.useForm();
  const [purgeTarget, setPurgeTarget] = useState<PlatformTenantRow | null>(null);
  const [purgeForm] = Form.useForm();
  const [purgingIds, setPurgingIds] = useState<number[]>([]);
  const [modal, modalContextHolder] = Modal.useModal();
  const emptyLocale = useListEmptyLocale('platformTenants', {
    onAction: () => setCreateOpen(true),
    actionLabel: '新建租户',
  });

  // 清退为后台异步任务：提交后轮询任务状态，完成前列表行显示「清退中」，避免短暂残留被误读
  useEffect(() => {
    if (!purgingIds.length) return undefined;
    const timer = window.setInterval(async () => {
      for (const id of purgingIds) {
        try {
          const task = await fetchPlatformTenantPurge(id);
          if (task.status === 'succeeded') {
            message.success(`租户「${task.tenantName}」清退完成`);
            setPurgingIds((prev) => prev.filter((x) => x !== id));
            actionRef.current?.reload();
          } else if (task.status === 'failed') {
            message.error(`租户「${task.tenantName}」清退失败：${task.error || '请稍后重试'}`);
            setPurgingIds((prev) => prev.filter((x) => x !== id));
            actionRef.current?.reload();
          }
        } catch {
          // 任务查询失败（如租户已删除）时停止轮询并刷新列表
          setPurgingIds((prev) => prev.filter((x) => x !== id));
          actionRef.current?.reload();
        }
      }
    }, PURGE_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [purgingIds]);

  if (!isPlatformAdmin(role, user?.tenantId)) {
    return <Result status="403" title="无权限" subTitle="仅平台管理员可管理平台租户" />;
  }

  const columns: ProColumns<PlatformTenantRow>[] = [
    { title: '租户编号', dataIndex: 'id', width: 100 },
    {
      title: '租户名称',
      dataIndex: 'name',
      ellipsis: true,
      render: (_, row) =>
        row.id === 0 ? (
          <span>
            {row.name} <Tag color="blue">平台</Tag>
          </span>
        ) : (
          row.name
        ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) =>
        purgingIds.includes(row.id) ? (
          <Tag color="processing">清退中</Tag>
        ) : row.status === 'disabled' ? (
          <Tag color="red">已停用</Tag>
        ) : (
          <Tag color="green">启用中</Tag>
        ),
    },
    { title: '账号数', dataIndex: 'adminCount', width: 100 },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 180,
      render: (_, row) => (row.createdAt ? formatDateTime(row.createdAt) : '—'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 180,
      render: (_, row) =>
        row.id === 0 || purgingIds.includes(row.id) ? (
          <span>—</span>
        ) : (
          <Space size="small">
            <Button
              type="link"
              size="small"
              onClick={() => {
                renameForm.setFieldsValue({ name: row.name });
                setRenameTarget(row);
              }}
            >
              改名
            </Button>
            {row.status === 'disabled' ? (
              <Button
                type="link"
                size="small"
                onClick={() =>
                  modal.confirm({
                    title: `启用租户「${row.name}」？`,
                    content: '启用后该租户下的账号可正常登录。',
                    okText: '启用',
                    onOk: async () => {
                      try {
                        await enablePlatformTenant(row.id);
                        message.success('租户已启用');
                        actionRef.current?.reload();
                      } catch (e: unknown) {
                        message.error((e as Error)?.message || '启用失败');
                      }
                    },
                  })
                }
              >
                启用
              </Button>
            ) : null}
            {row.status === 'disabled' ? (
              <Button
                type="link"
                size="small"
                danger
                onClick={() => {
                  purgeForm.resetFields();
                  setPurgeTarget(row);
                }}
              >
                清退删除
              </Button>
            ) : (
              <Button
                type="link"
                size="small"
                danger
                onClick={() =>
                  modal.confirm({
                    title: `停用租户「${row.name}」？`,
                    content: '停用后该租户所有账号将无法登录，已登录会话将在下次请求时失效。',
                    okText: '停用',
                    okButtonProps: { danger: true },
                    onOk: async () => {
                      try {
                        await disablePlatformTenant(row.id);
                        message.success('租户已停用');
                        actionRef.current?.reload();
                      } catch (e: unknown) {
                        message.error((e as Error)?.message || '停用失败');
                      }
                    },
                  })
                }
              >
                停用
              </Button>
            )}
          </Space>
        ),
    },
  ];

  return (
    <TmPageContainer
      title={PAGE_COPY.platformTenants.title}
      subTitle={PAGE_COPY.platformTenants.description}
    >
      <ProTable<PlatformTenantRow>
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        search={false}
        pagination={false}
        locale={emptyLocale}
        toolBarRender={() => [
          <Button key="create" type="primary" onClick={() => setCreateOpen(true)}>
            新建租户
          </Button>,
        ]}
        request={async () => {
          const res = await fetchPlatformTenants();
          return { data: res.list || [], success: true };
        }}
      />

      <Modal
        title="新建租户"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => createForm.submit()}
        destroyOnHidden
      >
        <Form
          form={createForm}
          layout="vertical"
          onFinish={async (v) => {
            try {
              await createPlatformTenant(v);
              message.success('租户已创建，初始管理员可直接登录');
              setCreateOpen(false);
              createForm.resetFields();
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '创建失败');
            }
          }}
        >
          <Form.Item name="name" label="租户名称" rules={[{ required: true, max: 128 }]}>
            <Input placeholder="示例：华东运营中心" />
          </Form.Item>
          <Form.Item
            name="adminEmail"
            label="初始管理员邮箱"
            rules={[{ required: true, type: 'email' }]}
          >
            <Input placeholder="tenant_admin@example.com" />
          </Form.Item>
          <Form.Item
            name="adminPassword"
            label="初始管理员密码"
            rules={[{ required: true, min: 6 }]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="租户改名"
        open={!!renameTarget}
        onCancel={() => setRenameTarget(null)}
        onOk={() => renameForm.submit()}
        destroyOnHidden
      >
        <Form
          form={renameForm}
          layout="vertical"
          onFinish={async (v: { name: string }) => {
            if (!renameTarget) return;
            try {
              await renamePlatformTenant(renameTarget.id, v.name);
              message.success('租户已改名');
              setRenameTarget(null);
              renameForm.resetFields();
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '改名失败');
            }
          }}
        >
          <Form.Item name="name" label="租户名称" rules={[{ required: true, max: 128 }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="清退删除租户"
        open={!!purgeTarget}
        onCancel={() => setPurgeTarget(null)}
        onOk={() => purgeForm.submit()}
        okText="下一步"
        okButtonProps={{ danger: true }}
        destroyOnHidden
      >
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="清退删除不可恢复"
          description="将级联删除该租户全部业务数据（账号、店铺、商品、草稿、货源、订单、采购、库存、客服、选品、采集、发布、批次及业务日志）。平台侧开租/清退审计将保留。"
        />
        <Form
          form={purgeForm}
          layout="vertical"
          onFinish={async (v: { confirmName: string }) => {
            if (!purgeTarget) return;
            const target = purgeTarget;
            modal.confirm({
              title: `确认清退租户「${target.name}」？`,
              content: '此操作不可恢复，将后台执行级联清理并逐表校验零残留。',
              okText: '确认清退',
              okButtonProps: { danger: true },
              onOk: async () => {
                try {
                  await purgePlatformTenant(target.id, v.confirmName);
                  message.success('清退任务已提交，将在后台执行');
                  setPurgeTarget(null);
                  purgeForm.resetFields();
                  setPurgingIds((prev) => (prev.includes(target.id) ? prev : [...prev, target.id]));
                  actionRef.current?.reload();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '清退失败');
                }
              },
            });
          }}
        >
          <Form.Item
            name="confirmName"
            label={`请输入租户名称「${purgeTarget?.name ?? ''}」以确认`}
            rules={[
              { required: true, message: '请输入租户名称' },
              {
                validator: (_, value) =>
                  !value || value === purgeTarget?.name
                    ? Promise.resolve()
                    : Promise.reject(new Error('输入的名称与租户名称不一致')),
              },
            ]}
          >
            <Input placeholder={purgeTarget?.name} />
          </Form.Item>
        </Form>
      </Modal>
      {modalContextHolder}
    </TmPageContainer>
  );
}

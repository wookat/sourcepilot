import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import { formatDateTime } from '@/utils/formatTime';
import {
  createPlatformTenant,
  fetchPlatformTenants,
  type PlatformTenantRow,
} from '@/services/platformTenants';
import { Button, Form, Input, Modal, Result, Tag, message } from 'antd';
import { useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { isPlatformAdmin } from '@/utils/permission';

export default function PlatformTenantsPage() {
  const actionRef = useRef<ActionType>();
  const { user, role } = usePermission();
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm();
  const emptyLocale = useListEmptyLocale('platformTenants', {
    onAction: () => setCreateOpen(true),
    actionLabel: '新建租户',
  });

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
    { title: '账号数', dataIndex: 'adminCount', width: 100 },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 180,
      render: (_, row) => (row.createdAt ? formatDateTime(row.createdAt) : '—'),
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
    </TmPageContainer>
  );
}

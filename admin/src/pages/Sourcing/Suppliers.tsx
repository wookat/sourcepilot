import { TmPageContainer } from '@/components/ui';
import {
  createSupplier,
  deleteSupplier,
  fetchSuppliers,
  updateSupplier,
  type Supplier,
} from '@/services/sourcing';
import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import { useCallback, useEffect, useState } from 'react';

const STATUS_TAG: Record<string, { text: string; color: string }> = {
  active: { text: '启用', color: 'green' },
  disabled: { text: '停用', color: 'default' },
};

export default function SuppliersPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: { role?: string } };
  };
  const writable = !isReadonly(initialState?.currentUser?.role);
  const [rows, setRows] = useState<Supplier[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Supplier | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchSuppliers({ page, pageSize, keyword: keyword || undefined });
      setRows(res.items || []);
      setTotal(res.total || 0);
    } catch (e) {
      message.error((e as Error).message || '加载供应商失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, keyword]);

  useEffect(() => {
    void load();
  }, [load]);

  const openModal = (row?: Supplier) => {
    setEditing(row || null);
    form.setFieldsValue(
      row || { platform: '1688', status: 'active', name: '', externalId: '', remark: '' },
    );
    setModalOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    try {
      if (editing) {
        await updateSupplier(editing.id, values);
        message.success('供应商已更新');
      } else {
        await createSupplier(values);
        message.success('供应商已创建');
      }
      setModalOpen(false);
      void load();
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    }
  };

  return (
    <TmPageContainer title="供应商管理" subTitle="1688 等采购供应商档案">
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="按名称 / 外部ID搜索"
          allowClear
          style={{ width: 260 }}
          onSearch={(v) => {
            setPage(1);
            setKeyword(v.trim());
          }}
        />
        {writable && (
          <Button type="primary" onClick={() => openModal()}>
            新增供应商
          </Button>
        )}
      </Space>
      <Table<Supplier>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        scroll={{ x: 900 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
        columns={[
          { title: '名称', dataIndex: 'name', width: 200 },
          { title: '平台', dataIndex: 'platform', width: 90 },
          { title: '外部ID', dataIndex: 'externalId', width: 140, render: (v) => v || '-' },
          { title: '评分', dataIndex: 'rating', width: 80, render: (v) => v ?? '-' },
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (v: string) => {
              const cfg = STATUS_TAG[v] || { text: v, color: 'default' };
              return <Tag color={cfg.color}>{cfg.text}</Tag>;
            },
          },
          { title: '备注', dataIndex: 'remark', ellipsis: true, render: (v) => v || '-' },
          ...(writable
            ? [{
            title: '操作',
            width: 160,
            render: (_: unknown, row: Supplier) => (
              <Space>
                <a onClick={() => openModal(row)}>编辑</a>
                <Popconfirm
                  title="删除该供应商？"
                  description="已绑定货源的供应商无法删除"
                  onConfirm={async () => {
                    try {
                      await deleteSupplier(row.id);
                      message.success('已删除');
                      void load();
                    } catch (e) {
                      message.error((e as Error).message || '删除失败');
                    }
                  }}
                >
                  <a style={{ color: '#ff4d4f' }}>删除</a>
                </Popconfirm>
              </Space>
            ),
          }]
            : []),
        ]}
      />
      <Modal
        title={editing ? '编辑供应商' : '新增供应商'}
        open={modalOpen}
        onOk={submit}
        onCancel={() => setModalOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="供应商名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如：义乌某某工厂" />
          </Form.Item>
          <Form.Item name="platform" label="平台">
            <Select
              options={[{ value: '1688', label: '1688' }]}
              disabled={!!editing}
            />
          </Form.Item>
          <Form.Item name="externalId" label="平台外部ID（1688 店铺/会员ID，可选）">
            <Input />
          </Form.Item>
          <Form.Item name="rating" label="评分（0-5，可选）">
            <InputNumber min={0} max={5} step={0.1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select
              options={[
                { value: 'active', label: '启用' },
                { value: 'disabled', label: '停用' },
              ]}
            />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

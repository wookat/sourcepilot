import { TmPageContainer } from '@/components/ui';
import {
  createSupplier,
  deleteProductSource,
  deleteSupplier,
  fetchOrphanSources,
  fetchSuppliers,
  updateSupplier,
  type OrphanSourceRow,
  type Supplier,
} from '@/services/sourcing';
import {
  Alert,
  Button,
  Form,
  Grid,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Breakpoint } from 'antd';
import { httpErrorCopy } from '@/constants/errorMessages';
import { formatDateTime } from '@/utils/formatTime';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import { useCallback, useEffect, useState } from 'react';

/** 次要列在 <768px 小屏折叠，只保留名称 / 平台 / 状态 / 操作。 */
const DESKTOP_ONLY: Breakpoint[] = ['md'];

const STATUS_TAG: Record<string, { text: string; color: string }> = {
  active: { text: '启用', color: 'green' },
  disabled: { text: '停用', color: 'default' },
};

export default function SuppliersPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: { role?: string } };
  };
  const writable = !isReadonly(initialState?.currentUser?.role);
  const screens = Grid.useBreakpoint();
  const [wideScreen, setWideScreen] = useState(
    () => typeof window === 'undefined' || window.innerWidth >= 768,
  );
  useEffect(() => {
    if (screens.md !== undefined) setWideScreen(screens.md);
  }, [screens.md]);
  const [rows, setRows] = useState<Supplier[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Supplier | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [orphans, setOrphans] = useState<OrphanSourceRow[]>([]);
  const [orphanLoading, setOrphanLoading] = useState(false);

  const loadOrphans = useCallback(async () => {
    setOrphanLoading(true);
    try {
      const res = await fetchOrphanSources();
      setOrphans(res.items || []);
    } catch (e) {
      message.error((e as Error).message || '加载孤儿货源失败');
    } finally {
      setOrphanLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadOrphans();
  }, [loadOrphans]);

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
        scroll={{ x: wideScreen ? 900 : undefined }}
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
          { title: '外部ID', dataIndex: 'externalId', width: 140, responsive: DESKTOP_ONLY, render: (v) => v || '-' },
          { title: '评分', dataIndex: 'rating', width: 80, responsive: DESKTOP_ONLY, render: (v) => v ?? '-' },
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (v: string) => {
              const cfg = STATUS_TAG[v] || { text: v, color: 'default' };
              return <Tag color={cfg.color}>{cfg.text}</Tag>;
            },
          },
          { title: '备注', dataIndex: 'remark', ellipsis: true, responsive: DESKTOP_ONLY, render: (v) => v || '-' },
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
                      message.error(httpErrorCopy(e, '删除失败'));
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
      {orphans.length > 0 && (
        <>
          <Typography.Title level={5} style={{ marginTop: 24 }}>
            孤儿货源（关联商品已删除）
          </Typography.Title>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="以下货源关联的商品已删除，会阻塞对应供应商的删除；解绑后供应商即可删除。"
          />
          <Table<OrphanSourceRow>
            rowKey="sourceId"
            size="small"
            loading={orphanLoading}
            dataSource={orphans}
            pagination={false}
            columns={[
              {
                title: '商品（已删除）',
                dataIndex: 'productTitle',
                ellipsis: true,
                render: (v: string) => v || '-',
              },
              { title: '供应商', dataIndex: 'supplierName', width: 180, render: (v) => v || '-' },
              {
                title: '主货源',
                dataIndex: 'isPrimary',
                width: 90,
                render: (v: boolean) => (v ? <Tag color="blue">主</Tag> : '-'),
              },
              { title: '规格映射数', dataIndex: 'skuCount', width: 100 },
              {
                title: '绑定时间',
                dataIndex: 'createdAt',
                width: 180,
                render: (v: string) => formatDateTime(v),
              },
              ...(writable
                ? [{
                    title: '操作',
                    width: 100,
                    render: (_: unknown, row: OrphanSourceRow) => (
                      <Popconfirm
                        title="解绑该孤儿货源？"
                        description="软删除该货源及其 SKU 映射，解绑后对应供应商可删除"
                        onConfirm={async () => {
                          try {
                            await deleteProductSource(row.sourceId);
                            message.success('已解绑');
                            void loadOrphans();
                          } catch (e) {
                            message.error((e as Error).message || '解绑失败');
                          }
                        }}
                      >
                        <a style={{ color: '#ff4d4f' }}>解绑</a>
                      </Popconfirm>
                    ),
                  }]
                : []),
            ]}
          />
        </>
      )}
      <Modal
        title={editing ? '编辑供应商' : '新增供应商'}
        open={modalOpen}
        onOk={submit}
        onCancel={() => setModalOpen(false)}
        destroyOnHidden
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

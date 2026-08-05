import { TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createWarehouse,
  deleteWarehouse,
  queryWarehouseMigrationPreview,
  queryWarehouseSummary,
  updateWarehouse,
  type WarehouseMigrationPreview,
  type WarehouseSummaryEntry,
} from '@/services/inventory';
import { ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Skeleton,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';

type WarehouseFormValues = {
  code?: string;
  name: string;
  priority?: number;
  remark?: string;
};

/** 仓库管理：租户级仓库增删改启停 + 存量迁移预检（默认仓不可删除/停用） */
export default function WarehousesPage() {
  const { canWriteInventory } = usePermission();
  const [rows, setRows] = useState<WarehouseSummaryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState<WarehouseMigrationPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [editing, setEditing] = useState<WarehouseSummaryEntry | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm<WarehouseFormValues>();

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    queryWarehouseSummary()
      .then((res) => setRows(res.list ?? []))
      .catch((e: unknown) => setError((e as Error)?.message || '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const loadPreview = useCallback(() => {
    setPreviewLoading(true);
    queryWarehouseMigrationPreview()
      .then((res) => setPreview(res))
      .catch((e: unknown) => message.error((e as Error)?.message || '迁移预检失败'))
      .finally(() => setPreviewLoading(false));
  }, []);

  const openCreate = () => {
    form.resetFields();
    setEditing(null);
    setCreateOpen(true);
  };

  const openEdit = (r: WarehouseSummaryEntry) => {
    form.setFieldsValue({ name: r.warehouseName, priority: r.priority });
    setEditing(r);
    setCreateOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      if (editing) {
        await updateWarehouse(editing.warehouseId, {
          name: values.name,
          priority: values.priority,
          remark: values.remark,
        });
        message.success('仓库已更新');
      } else {
        await createWarehouse({
          code: values.code?.trim() || undefined,
          name: values.name,
          priority: values.priority,
          remark: values.remark,
        });
        message.success('仓库已创建');
      }
      setCreateOpen(false);
      load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  const toggleEnabled = async (r: WarehouseSummaryEntry, enabled: boolean) => {
    try {
      await updateWarehouse(r.warehouseId, { enabled });
      message.success(enabled ? '仓库已启用' : '仓库已停用');
      load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '操作失败');
    }
  };

  const remove = async (r: WarehouseSummaryEntry) => {
    try {
      await deleteWarehouse(r.warehouseId);
      message.success('仓库已删除');
      load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '删除失败');
    }
  };

  const columns: ColumnsType<WarehouseSummaryEntry> = [
    {
      title: '仓库',
      dataIndex: 'warehouseName',
      ellipsis: true,
      render: (v: string, r) => (
        <Space size="small" wrap>
          <span>{v}</span>
          {r.isDefault ? <Tag color="blue">默认仓</Tag> : null}
          {!r.enabled ? <Tag>已停用</Tag> : null}
        </Space>
      ),
    },
    { title: '编码', dataIndex: 'code', width: 120, ellipsis: true },
    { title: '优先级', dataIndex: 'priority', width: 80, align: 'right' },
    { title: '库存合计（件）', dataIndex: 'totalStock', width: 130, align: 'right' },
    { title: '规格数', dataIndex: 'skuCount', width: 90, align: 'right' },
    {
      title: '操作',
      key: 'option',
      width: 220,
      render: (_, r) =>
        canWriteInventory ? (
          <Space size="small" wrap>
            <Typography.Link onClick={() => openEdit(r)}>编辑</Typography.Link>
            {!r.isDefault ? (
              <>
                <Switch
                  size="small"
                  checked={r.enabled}
                  checkedChildren="启用"
                  unCheckedChildren="停用"
                  onChange={(checked) => void toggleEnabled(r, checked)}
                />
                <Popconfirm
                  title="删除该仓库？仓库需先清空库存（调拨到其他仓库）"
                  okText="删除"
                  cancelText="取消"
                  onConfirm={() => void remove(r)}
                >
                  <Typography.Link type="danger">删除</Typography.Link>
                </Popconfirm>
              </>
            ) : (
              <Typography.Text type="secondary">默认仓不可删除/停用</Typography.Text>
            )}
          </Space>
        ) : (
          <Typography.Text type="secondary">只读</Typography.Text>
        ),
    },
  ];

  return (
    <TmPageContainer
      title="仓库管理"
      subTitle="轻量多仓：默认仓承接存量库存，非默认仓通过调拨/入库获得库存"
      extra={
        <Space wrap className="tm-page-header-extra">
          <Button onClick={loadPreview} loading={previewLoading}>
            存量迁移预检
          </Button>
          {canWriteInventory ? (
            <Button type="primary" onClick={openCreate}>
              新建仓库
            </Button>
          ) : null}
        </Space>
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          message="仓库列表加载失败"
          description={error}
          action={<Typography.Link onClick={load}>重试</Typography.Link>}
          style={{ marginBottom: 16 }}
        />
      ) : null}

      {preview ? (
        <ProCard variant="outlined" style={{ marginBottom: 16 }} title="存量迁移预检">
          <Alert
            type={preview.consistent ? 'success' : 'warning'}
            showIcon
            message={
              preview.consistent
                ? '库存口径一致：SKU 总库存 = 默认仓（推导）+ 非默认仓之和，迁移零丢失'
                : '发现口径不一致，请先处理再执行调拨'
            }
            style={{ marginBottom: 12 }}
          />
          <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }} bordered>
            <Descriptions.Item label="默认仓">
              {preview.defaultWarehouseExists ? preview.defaultWarehouseName || '默认仓' : '缺失'}
            </Descriptions.Item>
            <Descriptions.Item label="SKU 数">{preview.skuCount}</Descriptions.Item>
            <Descriptions.Item label="总库存（件）">{preview.totalStock}</Descriptions.Item>
            <Descriptions.Item label="非默认仓库存">{preview.nonDefaultStock}</Descriptions.Item>
            <Descriptions.Item label="默认仓库存（推导）">{preview.defaultDerivedStock}</Descriptions.Item>
            <Descriptions.Item label="孤儿仓库行 / 超发 SKU">
              {preview.orphanWarehouseRows} / {preview.negativeDerivedSkus}
            </Descriptions.Item>
          </Descriptions>
        </ProCard>
      ) : null}

      <ProCard variant="outlined">
        {loading ? (
          <Skeleton active paragraph={{ rows: 4 }} />
        ) : (
          <Table<WarehouseSummaryEntry>
            rowKey="warehouseId"
            size="small"
            columns={columns}
            dataSource={rows}
            scroll={{ x: 720 }}
            pagination={false}
            locale={{ emptyText: '暂无仓库' }}
          />
        )}
      </ProCard>

      <Modal
        title={editing ? '编辑仓库' : '新建仓库'}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void submit()}
        confirmLoading={submitting}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          {!editing ? (
            <Form.Item name="code" label="仓库编码" extra="留空自动生成；default 为默认仓保留编码">
              <Input maxLength={64} placeholder="如 south / WH-2" />
            </Form.Item>
          ) : null}
          <Form.Item name="name" label="仓库名称" rules={[{ required: true, message: '请输入仓库名称' }]}>
            <Input maxLength={128} placeholder="如 华南仓" />
          </Form.Item>
          <Form.Item name="priority" label="扣减优先级" extra="数值越小优先级越高；发货未指定仓库时按此顺序扣减">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={200} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}

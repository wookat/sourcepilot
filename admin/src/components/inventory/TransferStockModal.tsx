import {
  querySkuWarehouseStocks,
  transferWarehouseStock,
  type WarehouseStockEntry,
} from '@/services/inventory';
import { Alert, Form, Input, InputNumber, Modal, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

export type TransferStockModalProps = {
  open: boolean;
  productSkuId?: string;
  skuLabel?: string;
  onClose: () => void;
  onTransferred?: () => void;
};

/** 仓库调拨弹窗：源仓 → 目标仓，原子生成两条流水 */
export default function TransferStockModal({
  open,
  productSkuId,
  skuLabel,
  onClose,
  onTransferred,
}: TransferStockModalProps) {
  const [form] = Form.useForm<{
    fromWarehouseId: string;
    toWarehouseId: string;
    quantity: number;
    remark?: string;
  }>();
  const [stocks, setStocks] = useState<WarehouseStockEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const loadStocks = useCallback(() => {
    if (!productSkuId) return;
    setLoading(true);
    setLoadError(null);
    querySkuWarehouseStocks(productSkuId)
      .then((res) => setStocks(res.list ?? []))
      .catch((e: unknown) => setLoadError((e as Error)?.message || '仓库库存加载失败'))
      .finally(() => setLoading(false));
  }, [productSkuId]);

  useEffect(() => {
    if (open) {
      form.resetFields();
      loadStocks();
    }
  }, [open, form, loadStocks]);

  const optionOf = (e: WarehouseStockEntry, disableDisabled: boolean) => ({
    label: `${e.warehouseName}${e.isDefault ? '（默认）' : ''}${e.enabled ? '' : '（已停用）'} · 库存 ${e.stock}`,
    value: e.warehouseId,
    disabled: disableDisabled && !e.enabled,
  });

  return (
    <Modal
      title="仓库调拨"
      open={open}
      onCancel={onClose}
      confirmLoading={submitting}
      okText="确认调拨"
      cancelText="取消"
      onOk={async () => {
        const values = await form.validateFields();
        if (!productSkuId) return;
        if (values.fromWarehouseId === values.toWarehouseId) {
          message.error('源仓库与目标仓库不能相同');
          return;
        }
        setSubmitting(true);
        try {
          const res = await transferWarehouseStock({
            productSkuId,
            fromWarehouseId: values.fromWarehouseId,
            toWarehouseId: values.toWarehouseId,
            quantity: values.quantity,
            remark: values.remark?.trim() || undefined,
          });
          message.success(
            `调拨成功：${res.fromWarehouseName} → ${res.toWarehouseName}，数量 ${res.quantity}`,
          );
          onTransferred?.();
          onClose();
        } catch (e: unknown) {
          message.error((e as Error)?.message || '调拨失败');
        } finally {
          setSubmitting(false);
        }
      }}
    >
      {skuLabel ? (
        <Typography.Paragraph>
          <Space wrap>
            <span>调拨规格：</span>
            <Tag>{skuLabel}</Tag>
          </Space>
        </Typography.Paragraph>
      ) : null}
      {loadError ? (
        <Alert
          type="error"
          showIcon
          message={loadError}
          action={<Typography.Link onClick={loadStocks}>重试</Typography.Link>}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      <Form form={form} layout="vertical" disabled={loading}>
        <Form.Item
          name="fromWarehouseId"
          label="源仓库"
          rules={[{ required: true, message: '请选择源仓库' }]}
        >
          <Select
            placeholder="选择源仓库"
            loading={loading}
            options={stocks.map((e) => optionOf(e, false))}
          />
        </Form.Item>
        <Form.Item
          name="toWarehouseId"
          label="目标仓库"
          rules={[{ required: true, message: '请选择目标仓库' }]}
        >
          <Select
            placeholder="选择目标仓库"
            loading={loading}
            options={stocks.map((e) => optionOf(e, true))}
          />
        </Form.Item>
        <Form.Item
          name="quantity"
          label="调拨数量"
          rules={[{ required: true, message: '请输入调拨数量' }]}
        >
          <InputNumber min={1} precision={0} style={{ width: '100%' }} placeholder="大于 0 的整数" />
        </Form.Item>
        <Form.Item name="remark" label="备注">
          <Input.TextArea rows={2} maxLength={200} placeholder="调拨原因（可选）" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

import {
  batchCreateOrderShipments,
  type BatchShipmentLineResult,
} from '@/services/orders';
import { listCarriers, type CarrierRow } from '@/services/carriers';
import { Alert, Button, Input, Modal, Select, Space, Table, Tag, Tooltip, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { ORDER_STATUS } from '@/constants/status';

type ParsedLine = {
  line: number;
  raw: string;
  orderNo?: string;
  trackingNo?: string;
  carrier?: string;
  error?: string;
};

function parseShipText(text: string): ParsedLine[] {
  const out: ParsedLine[] = [];
  text.split(/\r?\n/).forEach((raw, i) => {
    const trimmed = raw.trim();
    if (!trimmed) return;
    const parts = trimmed.split(/[\s,，\t]+/).filter(Boolean);
    const item: ParsedLine = { line: i + 1, raw: trimmed };
    if (parts.length < 2) {
      item.error = '格式：订单号 快递单号 [承运商]（空格/逗号/Tab 分隔）';
      out.push(item);
      return;
    }
    item.orderNo = parts[0];
    item.trackingNo = parts[1];
    if (parts.length >= 3) item.carrier = parts.slice(2).join(' ');
    out.push(item);
  });
  return out;
}

export default function BatchShipModal({
  open,
  onClose,
  onDone,
}: {
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}) {
  const [text, setText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<BatchShipmentLineResult[] | null>(null);
  const [carriers, setCarriers] = useState<CarrierRow[]>([]);
  const [carriersLoading, setCarriersLoading] = useState(false);
  const [defaultCarrierCode, setDefaultCarrierCode] = useState<string | undefined>();

  useEffect(() => {
    if (!open) return;
    setText('');
    setResults(null);
    setDefaultCarrierCode(undefined);
    setCarriersLoading(true);
    listCarriers({ enabled: true })
      .then(setCarriers)
      .catch(() => setCarriers([]))
      .finally(() => setCarriersLoading(false));
  }, [open]);

  const parsed = useMemo(() => parseShipText(text), [text]);
  const validCount = parsed.filter((p) => !p.error).length;
  const errorLines = parsed.filter((p) => p.error);

  const submit = async () => {
    if (validCount === 0) {
      message.warning('没有可提交的有效行');
      return;
    }
    setSubmitting(true);
    try {
      const res = await batchCreateOrderShipments(
        parsed
          .filter((p) => !p.error)
          .map((p) => ({
            orderNo: p.orderNo as string,
            trackingNo: p.trackingNo as string,
            carrier: p.carrier,
          })),
        defaultCarrierCode,
      );
      setResults(res.results || []);
      if (res.failed === 0) {
        message.success(`全部成功：${res.succeeded} 条`);
      } else {
        message.warning(`成功 ${res.succeeded} 条，失败 ${res.failed} 条，请查看逐行结果`);
      }
      if (res.succeeded > 0) onDone();
    } catch (e) {
      message.error((e as Error).message || '批量发货失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="批量发货"
      open={open}
      width={720}
      onCancel={onClose}
      footer={
        results ? (
          <Button type="primary" onClick={onClose}>
            完成
          </Button>
        ) : (
          <Space>
            <Button onClick={onClose}>取消</Button>
            <Button
              type="primary"
              loading={submitting}
              disabled={validCount === 0}
              onClick={() => void submit()}
            >
              提交 {validCount > 0 ? `${validCount} 条` : ''}
            </Button>
          </Space>
        )
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="按订单号匹配已付款销售订单并新增「已发货」物流，订单会自动流转为已发货；未付款、已取消订单会逐行提示失败。"
        description="按当前「手工扣库存」策略，发货不会自动扣减本地库存（属预期行为）；结果中标记「未扣库存」的订单，可到订单详情「库存影响」Tab 手工扣减。"
      />
      {!results && (
        <>
          <Space style={{ marginBottom: 12, width: '100%' }} wrap>
            <span>默认物流商（未填承运商列时使用）：</span>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              placeholder="可选，不选则按旧格式处理"
              style={{ minWidth: 220 }}
              loading={carriersLoading}
              value={defaultCarrierCode}
              onChange={setDefaultCarrierCode}
              options={carriers.map((c) => ({ value: c.code, label: c.name }))}
            />
          </Space>
          <Input.TextArea
            rows={8}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={
              '每行一条：订单号 快递单号 [物流商]\n物流商列可填名称或代码（如：顺丰 / sf），缺省时使用上方默认物流商\n例如：\nSO-2026-0001 SF1234567890123 顺丰\nSO-2026-0002 YT9876543210'
            }
          />
          {errorLines.length > 0 && (
            <Alert
              style={{ marginTop: 12 }}
              type="warning"
              showIcon
              message={`${errorLines.length} 行格式有误（提交时会跳过）`}
              description={
                <ul style={{ margin: 0, paddingLeft: 18, maxHeight: 120, overflow: 'auto' }}>
                  {errorLines.slice(0, 20).map((p) => (
                    <li key={p.line}>
                      第 {p.line} 行「{p.raw.slice(0, 40)}」：{p.error}
                    </li>
                  ))}
                </ul>
              }
            />
          )}
        </>
      )}
      {results && (
        <>
          {results.some((r) => r.ok && r.inventoryDeducted === false) ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message="「未扣库存」属于手工扣库存策略下的预期行为"
              description="发货与库存扣减解耦，发货成功不会自动扣减本地库存；请到对应订单详情的「库存影响」Tab 手工扣减，保持库存准确。"
            />
          ) : null}
          <Table<BatchShipmentLineResult>
          rowKey={(r, i) => `${r.key}-${i}`}
          size="small"
          dataSource={results}
          pagination={results.length > 10 ? { pageSize: 10 } : false}
          scroll={{ x: 520 }}
          columns={[
            { title: '订单号', dataIndex: 'key', width: 200, ellipsis: true },
            {
              title: '结果',
              dataIndex: 'ok',
              width: 90,
              render: (ok: boolean) =>
                ok ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
            },
            {
              title: '说明',
              render: (_, r) =>
                r.ok ? (
                  <Space size={4} wrap>
                    {`订单状态：${
                      (ORDER_STATUS as Record<string, { text: string }>)[r.status || '']?.text ||
                      r.status ||
                      '-'
                    }`}
                    {r.inventoryDeducted === false ? (
                      <Tooltip title="预期行为：当前为手工扣库存策略，发货不会自动扣减库存；可到订单详情「库存影响」Tab 手工扣减。">
                        <Tag color="warning">未扣库存（预期）</Tag>
                      </Tooltip>
                    ) : null}
                  </Space>
                ) : (
                  r.message || '-'
                ),
            },
          ]}
          />
        </>
      )}
    </Modal>
  );
}

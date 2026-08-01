import {
  batchFillPurchaseLogistics,
  batchMarkPurchaseOrdersPlaced,
  fetchPurchaseOrders,
  type BatchLineResult,
  type PurchaseOrder,
} from '@/services/procurement';
import { Alert, Button, Input, Modal, Space, Table, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import {
  parseLogisticsText,
  parsePlacedText,
  type ParsedLogisticsLine,
  type ParsedPlacedLine,
} from './batchParse';

export type BatchMode = 'placed' | 'logistics';

const MODE_META: Record<
  BatchMode,
  { title: string; placeholder: string; hint: string; fetchStatus: string }
> = {
  placed: {
    title: '批量回填 1688 订单号',
    placeholder:
      '每行一条：采购单号(完整或前缀) 1688订单号\n例如：\n3f2a91b0 2026072912345678\n8c4d… 2026072987654321',
    hint: '从「下单中(人工)」采购单批量回填 1688 订单号，采购单号支持粘贴 CSV 导出的完整 ID 或唯一前缀。',
    fetchStatus: 'placing',
  },
  logistics: {
    title: '批量回填快递单号',
    placeholder:
      '每行一条：1688订单号 运单号 [承运商]\n例如：\n2026072912345678 SF1234567890 顺丰\n2026072987654321 YT9876543210',
    hint: '按 1688 订单号自动匹配采购单并回填运单号（已下单未付款的会先自动标记付款）。',
    fetchStatus: 'paid',
  },
};

export default function BatchBackfillModal({
  mode,
  open,
  onClose,
  onDone,
}: {
  mode: BatchMode;
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}) {
  const meta = MODE_META[mode];
  const [text, setText] = useState('');
  const [candidates, setCandidates] = useState<PurchaseOrder[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<BatchLineResult[] | null>(null);

  useEffect(() => {
    if (!open) return;
    setText('');
    setResults(null);
    if (mode === 'placed') {
      fetchPurchaseOrders({ page: 1, pageSize: 200, status: 'placing' })
        .then((res) => setCandidates(res.items || []))
        .catch(() => setCandidates([]));
    }
  }, [open, mode]);

  const parsed = useMemo(() => {
    if (mode === 'placed') {
      return parsePlacedText(
        text,
        candidates.map((c) => c.id),
      );
    }
    return parseLogisticsText(text);
  }, [mode, text, candidates]);

  const validCount = parsed.filter((p) => !p.error).length;
  const errorLines = parsed.filter((p) => p.error);

  const submit = async () => {
    if (validCount === 0) {
      message.warning('没有可提交的有效行');
      return;
    }
    setSubmitting(true);
    try {
      let res;
      if (mode === 'placed') {
        res = await batchMarkPurchaseOrdersPlaced(
          (parsed as ParsedPlacedLine[])
            .filter((p) => !p.error)
            .map((p) => ({
              purchaseOrderId: p.purchaseOrderId as string,
              externalOrderId: p.externalOrderId as string,
            })),
        );
      } else {
        res = await batchFillPurchaseLogistics(
          (parsed as ParsedLogisticsLine[])
            .filter((p) => !p.error)
            .map((p) => ({
              externalOrderId: p.externalOrderId as string,
              trackingNo: p.trackingNo as string,
              carrier: p.carrier,
            })),
        );
      }
      setResults(res.results || []);
      if (res.failed === 0) {
        message.success(`全部成功：${res.succeeded} 条`);
      } else {
        message.warning(`成功 ${res.succeeded} 条，失败 ${res.failed} 条，请查看逐行结果`);
      }
      if (res.succeeded > 0) onDone();
    } catch (e) {
      message.error((e as Error).message || '批量提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={meta.title}
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
      <Alert type="info" showIcon style={{ marginBottom: 12 }} message={meta.hint} />
      {!results && (
        <>
          <Input.TextArea
            rows={8}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={meta.placeholder}
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
          {mode === 'placed' && (
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              当前有 {candidates.length} 张「下单中(人工)」采购单可回填。
            </Typography.Paragraph>
          )}
        </>
      )}
      {results && (
        <Table<BatchLineResult>
          rowKey={(r, i) => `${r.key}-${i}`}
          size="small"
          dataSource={results}
          pagination={results.length > 10 ? { pageSize: 10 } : false}
          scroll={{ x: 560 }}
          columns={[
            { title: '行', dataIndex: 'key', width: 180, ellipsis: true },
            {
              title: '供应商',
              dataIndex: 'supplierName',
              width: 140,
              render: (v) => v || '-',
            },
            {
              title: '结果',
              dataIndex: 'ok',
              width: 90,
              render: (ok: boolean) =>
                ok ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
            },
            {
              title: '说明',
              render: (_, r) => (r.ok ? `状态：${r.status || '-'}` : r.message || '-'),
            },
          ]}
        />
      )}
    </Modal>
  );
}

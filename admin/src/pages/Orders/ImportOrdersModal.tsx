import { importOrders, type OrderImportRowResult } from '@/services/orders';
import { Alert, Button, Checkbox, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { groupImportOrders, parseImportText } from './importParse';

const PLACEHOLDER =
  '每行一条明细：订单号,客户名,商品标题,SKU编码,数量,单价[,币种]\n同一订单号的多行会合并为一张订单的多条明细\n例如：\nSO-20260801-01,王小明,蓝牙耳机,BT-001,2,19.9,USD\nSO-20260801-01,王小明,耳机充电仓,BT-001C,1,9.9,USD\nSO-20260801-02,李华,手机支架,PH-100,3,5.5';

const STATUS_TAG: Record<string, { color: string; text: string }> = {
  created: { color: 'green', text: '已创建' },
  skipped_duplicate: { color: 'gold', text: '已存在，跳过' },
  failed: { color: 'red', text: '失败' },
};

export default function ImportOrdersModal({
  open,
  onClose,
  onDone,
  shopOptions,
}: {
  open: boolean;
  onClose: () => void;
  onDone: () => void;
  shopOptions?: { label: string; value: string }[];
}) {
  const [text, setText] = useState('');
  const [matchSkus, setMatchSkus] = useState(true);
  const [shopId, setShopId] = useState<string | undefined>(undefined);
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<OrderImportRowResult[] | null>(null);

  useEffect(() => {
    if (!open) return;
    setText('');
    setResults(null);
    setMatchSkus(true);
    setShopId(undefined);
  }, [open]);

  const parsed = useMemo(() => parseImportText(text), [text]);
  const errorLines = parsed.filter((p) => p.error);
  const orders = useMemo(() => groupImportOrders(parsed), [parsed]);

  const submit = async () => {
    if (orders.length === 0) {
      message.warning('没有可提交的有效订单');
      return;
    }
    setSubmitting(true);
    try {
      const res = await importOrders({
        orders: orders.map((o) => ({
          orderNo: o.orderNo,
          customerName: o.customerName,
          currency: o.currency,
          totalAmount: o.totalAmount,
          items: o.items,
          ...(shopId ? { shopId } : {}),
        })),
        matchSkus,
      });
      setResults(res.results || []);
      if (res.failed === 0 && res.duplicate === 0) {
        message.success(`已导入 ${res.created} 张订单`);
      } else {
        message.warning(
          `导入 ${res.created} 张，已存在跳过 ${res.duplicate} 张，失败 ${res.failed} 张，请查看逐单结果`,
        );
      }
      if (res.created > 0) onDone();
    } catch (e) {
      message.error((e as Error).message || '批量导入失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="批量导入订单"
      open={open}
      width={760}
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
              disabled={orders.length === 0}
              onClick={() => void submit()}
            >
              导入 {orders.length > 0 ? `${orders.length} 张订单` : ''}
            </Button>
          </Space>
        )
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="店铺未接入前，可把平台后台导出的订单粘贴到这里批量建单；订单号已存在的会自动跳过，不会重复建单。"
      />
      {!results && (
        <>
          <Input.TextArea
            rows={8}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={PLACEHOLDER}
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
          <div style={{ marginTop: 12 }}>
            <Space wrap>
              <span>关联店铺（可选，应用到本次导入的全部订单）：</span>
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="不关联店铺"
                style={{ minWidth: 220 }}
                options={shopOptions}
                value={shopId}
                onChange={(v) => setShopId(v)}
              />
            </Space>
          </div>
          <div style={{ marginTop: 12 }}>
            <Checkbox checked={matchSkus} onChange={(e) => setMatchSkus(e.target.checked)}>
              导入后自动按 SKU 编码匹配本地商品规格
            </Checkbox>
          </div>
          {orders.length > 0 && (
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              解析出 {orders.length} 张订单、
              {orders.reduce((n, o) => n + o.items.length, 0)} 条明细。
            </Typography.Paragraph>
          )}
        </>
      )}
      {results && (
        <Table<OrderImportRowResult>
          rowKey={(r, i) => `${r.orderNo}-${i}`}
          size="small"
          dataSource={results}
          pagination={results.length > 10 ? { pageSize: 10 } : false}
          scroll={{ x: 560 }}
          columns={[
            { title: '订单号', dataIndex: 'orderNo', width: 200, ellipsis: true },
            {
              title: '结果',
              dataIndex: 'status',
              width: 130,
              render: (s: string) => {
                const meta = STATUS_TAG[s] || { color: 'default', text: s };
                return <Tag color={meta.color}>{meta.text}</Tag>;
              },
            },
            {
              title: '说明',
              render: (_, r) =>
                r.status === 'created'
                  ? `明细 ${r.itemsTotal} 条${r.itemsMatched > 0 ? `，自动匹配规格 ${r.itemsMatched} 条` : ''}`
                  : r.error || '-',
            },
          ]}
        />
      )}
    </Modal>
  );
}

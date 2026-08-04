import { getOrderPrintSheets, type PrintSheet } from '@/services/orders';
import { ORDER_SHIPMENT_STATUS } from '@/constants/status';
import { formatDateTime } from '@/utils/formatTime';
import { Alert, Button, Empty, Grid, Space, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from '@umijs/max';

const sheetStyles = `
.print-sheet { border: 1px solid #999; border-radius: 4px; padding: 16px 20px; margin: 0 auto 16px; max-width: 720px; background: #fff; color: #000; page-break-after: always; }
.print-sheet h3 { margin: 0 0 4px; font-size: 18px; }
.print-sheet .meta { color: #444; font-size: 12px; margin-bottom: 8px; }
.print-sheet table { width: 100%; border-collapse: collapse; font-size: 13px; margin-top: 8px; }
.print-sheet th, .print-sheet td { border: 1px solid #bbb; padding: 4px 8px; text-align: left; }
.print-sheet .section-title { font-weight: 600; margin-top: 12px; }
.print-sheet .label-box { border: 1px dashed #888; padding: 8px 12px; margin-top: 12px; font-size: 12px; color: #555; }
@media print {
  .print-toolbar { display: none !important; }
  body { background: #fff; }
  .ant-layout-sider, .ant-pro-sider, .ant-pro-global-header, .ant-layout-header,
  .ant-pro-layout-watermark, .ant-back-top { display: none !important; }
  .ant-layout, .ant-pro-layout .ant-layout { margin: 0 !important; }
  .ant-pro-layout-content, .ant-layout-content, .ant-pro-layout-container { margin: 0 !important; padding: 0 !important; background: #fff !important; }
  .print-sheet { border: none; max-width: none; margin: 0; padding: 0; }
  .print-sheet:last-child { page-break-after: auto; }
}
`;

export default function OrderPrintSheetsPage() {
  const [searchParams] = useSearchParams();
  const ids = useMemo(
    () => (searchParams.get('ids') || '').split(',').map((s) => s.trim()).filter(Boolean),
    [searchParams],
  );
  const [sheets, setSheets] = useState<PrintSheet[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const screens = Grid.useBreakpoint();

  useEffect(() => {
    if (ids.length === 0) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    getOrderPrintSheets(ids)
      .then(setSheets)
      .catch((e) => setError((e as Error).message || '加载拣货/发货单失败'))
      .finally(() => setLoading(false));
  }, [ids]);

  return (
    <div style={{ padding: 16 }}>
      <style>{sheetStyles}</style>
      <div className="print-toolbar" style={{ maxWidth: 720, margin: '0 auto 16px' }}>
        {!screens.md && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="当前为小屏设备，建议在桌面端浏览器打印以保证 A4 版式效果"
          />
        )}
        <Space wrap>
          <Button type="primary" disabled={loading || sheets.length === 0} onClick={() => window.print()}>
            打印
          </Button>
          <Button onClick={() => window.history.back()}>返回</Button>
          <span style={{ color: '#888' }}>
            拣货/发货单（人工贴单用，非电子面单），共 {sheets.length} 单
          </span>
        </Space>
      </div>
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin tip="加载拣货/发货单…" />
        </div>
      ) : error ? (
        <Alert type="error" showIcon message="加载失败" description={error} style={{ maxWidth: 720, margin: '0 auto' }} />
      ) : ids.length === 0 ? (
        <Empty description="缺少订单参数：请从订单列表勾选订单后点击「打印拣货单」进入" />
      ) : sheets.length === 0 ? (
        <Empty description="没有可打印的订单" />
      ) : (
        sheets.map((s) => (
          <div className="print-sheet" key={s.orderId}>
            <h3>拣货 / 发货单 · {s.orderNo}</h3>
            <div className="meta">
              平台：{s.platform}
              {s.shopName ? ` ｜ 店铺：${s.shopName}` : ''}
              {s.orderedAt ? ` ｜ 下单时间：${formatDateTime(s.orderedAt)}` : ''}
            </div>
            <div className="section-title">收件人</div>
            <table>
              <tbody>
                <tr>
                  <th style={{ width: 90 }}>姓名</th>
                  <td>{s.customerName || '—'}</td>
                  <th style={{ width: 90 }}>电话</th>
                  <td>{s.customerPhone || '—'}</td>
                </tr>
                <tr>
                  <th>邮箱</th>
                  <td>{s.customerEmail || '—'}</td>
                  <th>备注</th>
                  <td>{s.remark || '—'}</td>
                </tr>
              </tbody>
            </table>
            <div className="section-title">商品明细（拣货）</div>
            <table>
              <thead>
                <tr>
                  <th>商品</th>
                  <th style={{ width: 140 }}>规格</th>
                  <th style={{ width: 110 }}>规格编码</th>
                  <th style={{ width: 60 }}>数量</th>
                </tr>
              </thead>
              <tbody>
                {s.items.length === 0 ? (
                  <tr>
                    <td colSpan={4}>—</td>
                  </tr>
                ) : (
                  s.items.map((it, i) => (
                    <tr key={i}>
                      <td>{it.productTitle}</td>
                      <td>{it.skuName || '—'}</td>
                      <td>{it.skuCode || it.sellerSku || '—'}</td>
                      <td>{it.quantity}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            <div className="section-title">物流（发货）</div>
            <table>
              <thead>
                <tr>
                  <th style={{ width: 160 }}>物流商</th>
                  <th>运单号</th>
                  <th style={{ width: 100 }}>状态</th>
                </tr>
              </thead>
              <tbody>
                {s.shipments.length === 0 ? (
                  <tr>
                    <td colSpan={3}>未发货：发货后可重新打印带运单号版本</td>
                  </tr>
                ) : (
                  s.shipments.map((sh, i) => (
                    <tr key={i}>
                      <td>{sh.carrier || '—'}</td>
                      <td>{sh.trackingNo || '—'}</td>
                      <td>
                        {(ORDER_SHIPMENT_STATUS as Record<string, { text: string }>)[sh.status]?.text ||
                          sh.status}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            <div className="label-box">面单粘贴处：请将快递面单贴于此处（本单为拣货/发货单，非电子面单）</div>
          </div>
        ))
      )}
    </div>
  );
}

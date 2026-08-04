import { getOrderPrintSheetsWithTemplate, type PrintSheet } from '@/services/orders';
import {
  listWaybillTemplates,
  markOrdersPrinted,
  WAYBILL_SIZE_LABELS,
  type WaybillTemplateRow,
} from '@/services/waybill';
import { ORDER_SHIPMENT_STATUS } from '@/constants/status';
import { platformLabel } from '@/constants/userFriendly';
import { isReadonly } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';
import { useModel, useSearchParams } from '@umijs/max';
import { Alert, Button, Empty, Grid, Select, Space, Spin, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

// 尺寸相关样式：100×180 / 100×150 使用毫米宽度模拟标签纸，A4 一联单保持整页宽。
// printWidth = 页面内容宽（纸张宽度 - 左右边距），打印时锁定单据宽度，避免浏览器按视口宽度缩放。
const SIZE_STYLES: Record<
  WaybillTemplateRow['sizeCode'],
  { maxWidth: string; printWidth: string; page: string }
> = {
  '100x180': { maxWidth: '100mm', printWidth: '92mm', page: 'size: 100mm 180mm; margin: 4mm;' },
  '100x150': { maxWidth: '100mm', printWidth: '92mm', page: 'size: 100mm 150mm; margin: 4mm;' },
  a4_list: { maxWidth: '720px', printWidth: '186mm', page: 'size: A4; margin: 12mm;' },
};

const buildStyles = (sizeCode: WaybillTemplateRow['sizeCode']) => `
.print-sheet { border: 1px solid #999; border-radius: 4px; padding: 12px 14px; margin: 0 auto 16px; max-width: ${SIZE_STYLES[sizeCode].maxWidth}; background: #fff; color: #000; page-break-after: always; }
.print-sheet h3 { margin: 0 0 4px; font-size: ${sizeCode === 'a4_list' ? '18px' : '14px'}; }
.print-sheet .meta { color: #444; font-size: 11px; margin-bottom: 6px; }
.print-sheet table { width: 100%; border-collapse: collapse; font-size: ${sizeCode === 'a4_list' ? '13px' : '11px'}; margin-top: 6px; }
.print-sheet th, .print-sheet td { border: 1px solid #bbb; padding: 3px 6px; text-align: left; }
.print-sheet .section-title { font-weight: 600; margin-top: 10px; font-size: ${sizeCode === 'a4_list' ? '14px' : '12px'}; }
.print-sheet .header-text, .print-sheet .footer-text { font-size: 12px; color: #333; text-align: center; margin: 4px 0; }
.print-sheet .logo-box { border: 1px dashed #aaa; padding: 6px 10px; margin-bottom: 6px; font-size: 11px; color: #888; text-align: center; }
.print-sheet .label-box { border: 1px dashed #888; padding: 8px 12px; margin-top: 10px; font-size: 11px; color: #555; }
@media print {
  @page { ${SIZE_STYLES[sizeCode].page} }
  .print-toolbar { display: none !important; }
  html, body { width: auto !important; height: auto !important; overflow: visible !important; background: #fff; }
  #root, .ant-app, .ant-layout, .ant-pro-layout, .ant-layout-content,
  .ant-pro-layout-content, .ant-pro-layout-container {
    display: block !important; overflow: visible !important;
    height: auto !important; min-height: 0 !important; width: auto !important;
  }
  .ant-layout-sider, .ant-pro-sider, .ant-pro-global-header, .ant-layout-header,
  .ant-pro-layout-watermark, .ant-back-top { display: none !important; }
  .ant-layout, .ant-pro-layout .ant-layout { margin: 0 !important; }
  .ant-pro-layout-content, .ant-layout-content, .ant-pro-layout-container { margin: 0 !important; padding: 0 !important; background: #fff !important; }
  .print-sheet {
    border: none; border-radius: 0; padding: 0; margin: 0 auto;
    width: ${SIZE_STYLES[sizeCode].printWidth}; max-width: ${SIZE_STYLES[sizeCode].printWidth};
    page-break-after: always; break-after: page;
    page-break-inside: avoid; break-inside: avoid;
  }
  .print-sheet:last-child { page-break-after: auto; break-after: auto; }
}
`;

export default function OrderPrintSheetsPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);
  const [searchParams, setSearchParams] = useSearchParams();
  const ids = useMemo(
    () => (searchParams.get('ids') || '').split(',').map((s) => s.trim()).filter(Boolean),
    [searchParams],
  );
  const templateIdParam = searchParams.get('templateId') || '';
  const [sheets, setSheets] = useState<PrintSheet[]>([]);
  const [template, setTemplate] = useState<WaybillTemplateRow | null>(null);
  const [templates, setTemplates] = useState<WaybillTemplateRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [marking, setMarking] = useState(false);
  const screens = Grid.useBreakpoint();

  useEffect(() => {
    listWaybillTemplates()
      .then(setTemplates)
      .catch(() => setTemplates([]));
  }, []);

  useEffect(() => {
    if (ids.length === 0) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    getOrderPrintSheetsWithTemplate(ids, templateIdParam || undefined)
      .then((res) => {
        setSheets(res.items);
        setTemplate(res.template || null);
      })
      .catch((e) => setError((e as Error).message || '加载拣货/发货单失败'))
      .finally(() => setLoading(false));
  }, [ids, templateIdParam]);

  const switchTemplate = useCallback(
    (tid: string) => {
      const next = new URLSearchParams(searchParams);
      next.set('templateId', tid);
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const markPrinted = async () => {
    setMarking(true);
    try {
      const res = await markOrdersPrinted(ids);
      message.success(`已标记 ${res.marked} 单为已打单（不影响发货状态）`);
    } catch (e) {
      message.error((e as Error).message || '标记打单状态失败');
    } finally {
      setMarking(false);
    }
  };

  const sizeCode = template?.sizeCode || 'a4_list';
  const show = {
    recipient: template ? template.showRecipient : true,
    sender: template ? template.showSender : false,
    items: template ? template.showItems : true,
    remark: template ? template.showRemark : true,
    logo: template ? template.showCarrierLogo : false,
  };

  return (
    <div style={{ padding: 16 }}>
      <style>{buildStyles(sizeCode)}</style>
      <div className="print-toolbar" style={{ maxWidth: 720, margin: '0 auto 16px' }}>
        {!screens.md && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="当前为小屏设备，建议在桌面端浏览器打印以保证版式效果"
          />
        )}
        <Space wrap>
          <span>面单模板：</span>
          <Select
            style={{ minWidth: 220 }}
            value={template?.id}
            loading={loading && !template}
            options={templates.map((t) => ({
              value: t.id,
              label: `${t.name}（${WAYBILL_SIZE_LABELS[t.sizeCode] || t.sizeCode}）`,
            }))}
            onChange={switchTemplate}
            placeholder="选择打印模板"
          />
          <Button type="primary" disabled={loading || sheets.length === 0} onClick={() => window.print()}>
            打印
          </Button>
          <Button
            disabled={loading || sheets.length === 0 || readonly}
            loading={marking}
            onClick={() => void markPrinted()}
          >
            标记已打单
          </Button>
          <Button onClick={() => window.history.back()}>返回</Button>
          <span style={{ color: '#888' }}>
            拣货/发货单（人工贴单用，非电子面单），共 {sheets.length} 单
          </span>
        </Space>
      </div>
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin tip="加载拣货/发货单…">
            <div style={{ minHeight: 48 }} />
          </Spin>
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
            {show.logo ? <div className="logo-box">物流商 logo 位（接入电子面单后展示）</div> : null}
            {template?.headerText ? <div className="header-text">{template.headerText}</div> : null}
            <h3>拣货 / 发货单 · {s.orderNo}</h3>
            <div className="meta">
              平台：{platformLabel(s.platform)}
              {s.shopName ? ` ｜ 店铺：${s.shopName}` : ''}
              {s.orderedAt ? ` ｜ 下单时间：${formatDateTime(s.orderedAt)}` : ''}
            </div>
            {show.sender ? (
              <>
                <div className="section-title">发件人</div>
                <table>
                  <tbody>
                    <tr>
                      <th style={{ width: 90 }}>店铺</th>
                      <td>{s.shopName || '—'}</td>
                      <th style={{ width: 90 }}>平台</th>
                      <td>{platformLabel(s.platform)}</td>
                    </tr>
                  </tbody>
                </table>
              </>
            ) : null}
            {show.recipient ? (
              <>
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
                      <td colSpan={3}>{s.customerEmail || '—'}</td>
                    </tr>
                  </tbody>
                </table>
              </>
            ) : null}
            {show.remark ? (
              <>
                <div className="section-title">备注</div>
                <table>
                  <tbody>
                    <tr>
                      <td>{s.remark || '—'}</td>
                    </tr>
                  </tbody>
                </table>
              </>
            ) : null}
            {show.items ? (
              <>
                <div className="section-title">商品明细（拣货）</div>
                <table>
                  <thead>
                    <tr>
                      <th>商品</th>
                      <th style={{ width: 120 }}>规格</th>
                      <th style={{ width: 100 }}>规格编码</th>
                      <th style={{ width: 50 }}>数量</th>
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
              </>
            ) : null}
            <div className="section-title">物流（发货）</div>
            <table>
              <thead>
                <tr>
                  <th style={{ width: 140 }}>物流商</th>
                  <th>运单号</th>
                  <th style={{ width: 90 }}>状态</th>
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
            {template?.footerText ? <div className="footer-text">{template.footerText}</div> : null}
          </div>
        ))
      )}
    </div>
  );
}

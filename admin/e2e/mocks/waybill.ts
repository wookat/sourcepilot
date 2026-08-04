import { ok } from './envelope';

export const E2E_TEMPLATE_DEFAULT_ID = 'e2e-waybill-template-default';
export const E2E_TEMPLATE_SMALL_ID = 'e2e-waybill-template-small';
export const E2E_TEMPLATE_A4_ID = 'e2e-waybill-template-a4';
export const E2E_RULE_ID = 'e2e-shipping-rule-1';
export const E2E_PRINT_ORDER_ID = 'e2e-print-order-1';

export const e2eWaybillTemplates = [
  {
    id: E2E_TEMPLATE_DEFAULT_ID,
    tenantId: 1,
    name: '标准面单 100×180',
    sizeCode: '100x180',
    showRecipient: true,
    showSender: true,
    showItems: true,
    showRemark: true,
    showCarrierLogo: false,
    headerText: 'E2E 页眉文本',
    footerText: 'E2E 页脚文本',
    isDefault: true,
    isPreset: true,
    sortOrder: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: E2E_TEMPLATE_SMALL_ID,
    tenantId: 1,
    name: '小号面单 100×150',
    sizeCode: '100x150',
    showRecipient: true,
    showSender: false,
    showItems: true,
    showRemark: false,
    showCarrierLogo: true,
    headerText: '',
    footerText: '',
    isDefault: false,
    isPreset: true,
    sortOrder: 1,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: E2E_TEMPLATE_A4_ID,
    tenantId: 1,
    name: 'A4 一联单',
    sizeCode: 'a4_list',
    showRecipient: true,
    showSender: true,
    showItems: true,
    showRemark: true,
    showCarrierLogo: false,
    headerText: '',
    footerText: '',
    isDefault: false,
    isPreset: false,
    sortOrder: 2,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eShippingRules = [
  {
    id: E2E_RULE_ID,
    tenantId: 1,
    name: '江浙沪标准件走中通',
    priority: 10,
    enabled: true,
    provinces: ['上海', '江苏', '浙江'],
    platforms: [],
    maxWeightKg: 5,
    carrierCode: 'zto',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'e2e-shipping-rule-2',
    tenantId: 1,
    name: '高客单价订单走顺丰',
    priority: 20,
    enabled: false,
    provinces: [],
    platforms: [],
    minAmount: 500,
    carrierCode: 'sf',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2eCarriers = [
  {
    id: 'e2e-carrier-zto',
    tenantId: 1,
    code: 'zto',
    name: '中通快递',
    enabled: true,
    isPreset: true,
    sortOrder: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'e2e-carrier-sf',
    tenantId: 1,
    code: 'sf',
    name: '顺丰速运',
    enabled: true,
    isPreset: true,
    sortOrder: 1,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2ePrintSheets = [
  {
    orderId: E2E_PRINT_ORDER_ID,
    orderNo: 'SO-E2E-0001',
    platform: 'manual',
    shopName: 'E2E 测试店铺',
    customerName: '测试买家',
    customerPhone: '13800000000',
    customerEmail: '',
    remark: '请尽快发货',
    orderedAt: '2026-01-01T00:00:00Z',
    items: [
      {
        productTitle: 'E2E 测试商品',
        skuName: '红色 / L',
        skuCode: 'SKU-E2E-1',
        quantity: 2,
      },
    ],
    shipments: [],
  },
];

export function waybillResponse(path: string, searchParams: URLSearchParams) {
  if (path === '/api/v1/waybill-templates') return ok({ items: e2eWaybillTemplates });
  if (path === '/api/v1/shipping-rules') return ok({ items: e2eShippingRules });
  if (path === '/api/v1/carriers') return ok({ items: e2eCarriers });
  if (path === '/api/v1/orders/print/sheets') {
    const tid = searchParams.get('templateId');
    const template = e2eWaybillTemplates.find((t) => t.id === tid) || e2eWaybillTemplates[0];
    return ok({ items: e2ePrintSheets, template });
  }
  return null;
}

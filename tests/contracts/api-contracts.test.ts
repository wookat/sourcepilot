import { describe, expect, it } from 'vitest';
import contracts from './api-contracts.json';

const routeKey = (endpoint: { method: string; path: string }) => `${endpoint.method} ${endpoint.path}`;

describe('TradeMind API contract registry', () => {
  it('keeps the backend envelope explicit for frontend and E2E mocks', () => {
    expect(contracts.envelope.success).toEqual(['code', 'message', 'data']);
    expect(contracts.envelope.optional).toContain('traceId');
    expect(contracts.envelope.errorCodeRule).toContain('non-zero');
  });

  it('covers the core Admin product publishing, readiness and logistics endpoints', () => {
    const routes = new Set(contracts.endpoints.map(routeKey));

    expect(routes).toEqual(
      new Set([
        'GET /api/v1/auth/profile',
        'GET /api/v1/image/providers',
        'GET /api/v1/products/:id',
        'GET /api/v1/products/:id/readiness',
        'GET /api/v1/products/:id/publications',
        'GET /api/v1/product-publications/:id/douyin/sku-bindings',
        'GET /api/v1/products/:id/publish-targets',
        'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft',
        'POST /api/v1/products/:id/publish',
        'GET /api/v1/carriers',
        'POST /api/v1/carriers',
        'PUT /api/v1/carriers/:id',
        'DELETE /api/v1/carriers/:id',
        'GET /api/v1/banned-words',
        'POST /api/v1/banned-words',
        'PUT /api/v1/banned-words/:id',
        'DELETE /api/v1/banned-words/:id',
        'GET /api/v1/banned-words/categories',
        'PUT /api/v1/banned-words/categories/:category',
        'GET /api/v1/products/:id/banned-words/check',
        'POST /api/v1/products/banned-words/check-batch',
        'POST /api/v1/orders/shipments/batch',
        'GET /api/v1/orders/print/sheets',
        'POST /api/v1/orders/:id/shipments/:shipmentId/refresh-tracking',
        'POST /api/v1/imports/parse',
        'POST /api/v1/imports/validate',
        'POST /api/v1/imports/commit',
        'GET /api/v1/imports',
        'GET /api/v1/imports/:id',
        'GET /api/v1/imports/:id/errors.csv',
        'GET /api/v1/imports/templates/:kind',
        'GET /api/v1/imports/progress',
        'GET /api/v1/imports/export/:kind',
        'GET /api/v1/imports/mappings',
        'POST /api/v1/imports/mappings',
        'DELETE /api/v1/imports/mappings/:id',
        'GET /api/v1/dashboard/screen',
        'GET /api/v1/dashboard/screen/config',
        'PUT /api/v1/dashboard/screen/config',
        'GET /api/v1/reports/profit',
        'GET /api/v1/reports/profit/export.csv',
        'GET /api/v1/reports/procurement',
        'GET /api/v1/reports/inventory',
        'GET /api/v1/inventory/warehouses',
        'GET /api/v1/inventory/warehouses/summary',
        'GET /api/v1/inventory/warehouses/migration-preview',
        'POST /api/v1/inventory/warehouses',
        'PUT /api/v1/inventory/warehouses/:id',
        'DELETE /api/v1/inventory/warehouses/:id',
        'POST /api/v1/inventory/transfers',
        'GET /api/v1/inventory/sku-warehouse-stocks',
        'GET /api/v1/customer/reply-templates',
        'POST /api/v1/customer/reply-templates',
        'PUT /api/v1/customer/reply-templates/:id',
        'DELETE /api/v1/customer/reply-templates/:id',
        'POST /api/v1/customer/reply-templates/reorder',
        'GET /api/v1/customer/buyer-message-rules',
        'POST /api/v1/customer/buyer-message-rules',
        'PUT /api/v1/customer/buyer-message-rules/:id',
        'DELETE /api/v1/customer/buyer-message-rules/:id',
        'GET /api/v1/customer/buyer-message-rules/backfill-estimate',
        'GET /api/v1/customer/buyer-messages/drafts',
        'POST /api/v1/customer/buyer-messages/generate',
        'PUT /api/v1/customer/buyer-messages/drafts/:id',
        'POST /api/v1/customer/buyer-messages/drafts/:id/regenerate',
        'POST /api/v1/customer/buyer-messages/drafts/:id/mark-sent',
        'POST /api/v1/customer/buyer-messages/drafts/:id/ignore',
        'POST /api/v1/customer/buyer-messages/drafts/batch-mark-sent',
        'GET /api/v1/waybill-templates',
        'POST /api/v1/waybill-templates',
        'PUT /api/v1/waybill-templates/:id',
        'DELETE /api/v1/waybill-templates/:id',
        'GET /api/v1/shipping-rules',
        'POST /api/v1/shipping-rules',
        'POST /api/v1/shipping-rules/recommend',
        'PUT /api/v1/shipping-rules/:id',
        'DELETE /api/v1/shipping-rules/:id',
        'GET /api/v1/order-review-rules',
        'POST /api/v1/order-review-rules',
        'POST /api/v1/order-review-rules/dry-run',
        'PUT /api/v1/order-review-rules/:ruleId',
        'DELETE /api/v1/order-review-rules/:ruleId',
        'GET /api/v1/order-review',
        'POST /api/v1/order-review/approve',
        'POST /api/v1/order-review/reject',
        'GET /api/v1/order-automation-rules',
        'POST /api/v1/order-automation-rules',
        'POST /api/v1/order-automation-rules/dry-run',
        'PUT /api/v1/order-automation-rules/:ruleId',
        'DELETE /api/v1/order-automation-rules/:ruleId',
        'GET /api/v1/order-automation-logs',
        'POST /api/v1/order-automation-logs/:logId/retry',
        'GET /api/v1/orders/:id/automation-logs',
        'POST /api/v1/orders/print/mark',
        'POST /api/v1/orders/shipping-recommendations',
        'GET /api/v1/selection/candidates/:id/insights',
        'GET /api/v1/selection/candidates/:id/price-trend',
        'GET /api/v1/selection/compare',
        'GET /api/v1/selection/market-sources',
        'GET /api/v1/finance/expense-types',
        'GET /api/v1/finance/payments',
        'POST /api/v1/finance/payments',
        'DELETE /api/v1/finance/payments/:id',
        'POST /api/v1/finance/order-expenses',
        'DELETE /api/v1/finance/order-expenses/:id',
        'GET /api/v1/finance/shop-expenses',
        'POST /api/v1/finance/shop-expenses',
        'DELETE /api/v1/finance/shop-expenses/:id',
        'GET /api/v1/finance/orders/:id/summary',
        'GET /api/v1/finance/reconciliation',
        'GET /api/v1/finance/reconciliation/export.csv',
        'GET /api/v1/finance/report',
        'GET /api/v1/finance/report/export.csv',
        'PUT /api/v1/procurement/orders/:id/items/:itemId/actual-price',
        'GET /api/v1/mcp/tokens',
        'POST /api/v1/mcp/tokens',
        'POST /api/v1/mcp/tokens/:id/revoke',
        'GET /api/v1/mcp/audit-logs',
        'POST /api/mcp',
      ]),
    );
  });

  it('defines payload/query contracts for finance bookkeeping APIs', () => {
    const createPayment = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/finance/payments');
    const listPayments = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/finance/payments');
    const createOrderExpense = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/finance/order-expenses',
    );
    const createShopExpense = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/finance/shop-expenses',
    );
    const reconciliation = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/finance/reconciliation');
    const actualPrice = contracts.endpoints.find(
      (item) => routeKey(item) === 'PUT /api/v1/procurement/orders/:id/items/:itemId/actual-price',
    );

    expect(createPayment?.requestBody).toEqual([
      'orderId',
      'amount',
      'currency',
      'feeAmount',
      'receivedAt',
      'channel',
      'remark',
    ]);
    expect(listPayments?.query).toEqual(['orderId', 'shopId', 'status', 'page', 'pageSize']);
    expect(createOrderExpense?.requestBody).toEqual(['orderId', 'typeCode', 'amount', 'currency', 'incurredAt', 'remark']);
    expect(createShopExpense?.requestBody).toEqual(['shopId', 'month', 'typeCode', 'amount', 'currency', 'remark']);
    expect(reconciliation?.query).toEqual(['days', 'start', 'end', 'status']);
    expect(actualPrice?.requestBody).toEqual(['actualPrice']);
  });

  it('defines payload/query contracts for logistics APIs', () => {
    const createCarrier = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/carriers');
    const updateCarrier = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/carriers/:id');
    const listCarriers = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/carriers');
    const batchShipments = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/orders/shipments/batch');
    const printSheets = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/orders/print/sheets');

    expect(createCarrier?.requestBody).toEqual(['code', 'name', 'trackingUrlTemplate', 'sortOrder']);
    expect(updateCarrier?.requestBody).toEqual(['name', 'enabled', 'trackingUrlTemplate', 'sortOrder']);
    expect(listCarriers?.query).toEqual(['enabled', 'keyword']);
    expect(batchShipments?.requestBody).toEqual(['items', 'defaultCarrierCode']);
    expect(printSheets?.query).toEqual(['ids', 'templateId']);
  });

  it('defines payload/query contracts for waybill template and shipping rule APIs', () => {
    const createTemplate = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/waybill-templates');
    const updateTemplate = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/waybill-templates/:id');
    const createRule = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/shipping-rules');
    const updateRule = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/shipping-rules/:id');
    const recommend = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/shipping-rules/recommend');
    const markPrinted = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/orders/print/mark');
    const orderRecs = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/orders/shipping-recommendations',
    );

    const templateBody = [
      'name',
      'sizeCode',
      'showRecipient',
      'showSender',
      'showItems',
      'showRemark',
      'showCarrierLogo',
      'headerText',
      'footerText',
      'isDefault',
      'sortOrder',
    ];
    const ruleBody = [
      'name',
      'priority',
      'enabled',
      'provinces',
      'platforms',
      'minWeightKg',
      'maxWeightKg',
      'minAmount',
      'maxAmount',
      'carrierCode',
    ];
    expect(createTemplate?.requestBody).toEqual(templateBody);
    expect(updateTemplate?.requestBody).toEqual(templateBody);
    expect(createRule?.requestBody).toEqual(ruleBody);
    expect(updateRule?.requestBody).toEqual(ruleBody);
    expect(recommend?.requestBody).toEqual(['province', 'platform', 'weightKg', 'amount']);
    expect(markPrinted?.requestBody).toEqual(['ids']);
    expect(orderRecs?.requestBody).toEqual(['items']);
  });

  it('defines payload/query contracts for banned word compliance APIs', () => {
    const listWords = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/banned-words');
    const createWord = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/banned-words');
    const updateWord = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/banned-words/:id');
    const toggleCategory = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/banned-words/categories/:category');
    const batchCheck = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/banned-words/check-batch');

    expect(listWords?.query).toEqual(['category', 'level', 'keyword', 'enabled']);
    expect(createWord?.requestBody).toEqual(['word', 'category', 'level', 'suggestion']);
    expect(updateWord?.requestBody).toEqual(['enabled', 'level', 'category', 'suggestion']);
    expect(toggleCategory?.requestBody).toEqual(['enabled']);
    expect(batchCheck?.requestBody).toEqual(['productIds']);
  });

  it('defines payload/query contracts for state-changing publish APIs', () => {
    const createDraft = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft');
    const publish = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/publish');
    const readiness = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/products/:id/readiness');

    expect(createDraft?.requestBody).toEqual(['shopId', 'publishMode', 'force']);
    expect(publish?.requestBody).toEqual(['shopId', 'options', 'force']);
    expect(readiness?.query).toEqual(['platform', 'shopId', 'mode']);
  });

  it('defines payload/query contracts for migration import wizard APIs', () => {
    const validate = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/imports/validate');
    const commit = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/imports/commit');
    const list = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/imports');

    const wizardBody = ['kind', 'shopId', 'columns', 'rows', 'mapping', 'fileName', 'fileHash', 'sourceFormat'];
    expect(validate?.requestBody).toEqual(wizardBody);
    expect(commit?.requestBody).toEqual(wizardBody);
    expect(list?.query).toEqual(['page', 'pageSize', 'kind']);
  });

  it('defines query contracts for the deep report read APIs (GET-only, readonly included)', () => {
    const profit = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/reports/profit');
    const profitCsv = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/reports/profit/export.csv');
    const procurement = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/reports/procurement');
    const inventory = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/reports/inventory');

    expect(profit?.query).toEqual(['dimension', 'days', 'start', 'end']);
    expect(profit?.responseData).toBe('ProfitReportDTO');
    expect(profitCsv?.query).toEqual(['dimension', 'days', 'start', 'end']);
    expect(profitCsv?.responseData).toBe('CSV');
    expect(procurement?.query).toEqual(['days', 'start', 'end']);
    expect(procurement?.responseData).toBe('ProcurementReportDTO');
    expect(inventory?.query).toEqual(['slowDays', 'warehouseId']);
    expect(inventory?.responseData).toBe('InventoryReportDTO');
    for (const endpoint of [profit, profitCsv, procurement, inventory]) {
      expect(endpoint?.method).toBe('GET');
    }
  });

  it('defines payload/query contracts for customer reply template APIs', () => {
    const list = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/customer/reply-templates');
    const create = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/customer/reply-templates');
    const update = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/customer/reply-templates/:id');
    const reorder = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/customer/reply-templates/reorder',
    );

    const upsertBody = ['groupKey', 'name', 'content', 'sortOrder', 'enabled', 'defaultLanguage', 'variants'];
    expect(list?.query).toEqual(['group', 'keyword', 'enabled']);
    expect(create?.requestBody).toEqual(upsertBody);
    expect(update?.requestBody).toEqual(upsertBody);
    expect(reorder?.requestBody).toEqual(['groupKey', 'ids']);
  });

  it('defines payload/query contracts for buyer auto message rule and draft APIs', () => {
    const createRule = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/customer/buyer-message-rules',
    );
    const updateRule = contracts.endpoints.find(
      (item) => routeKey(item) === 'PUT /api/v1/customer/buyer-message-rules/:id',
    );
    const listDrafts = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/customer/buyer-messages/drafts',
    );
    const editDraft = contracts.endpoints.find(
      (item) => routeKey(item) === 'PUT /api/v1/customer/buyer-messages/drafts/:id',
    );
    const batchMarkSent = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/customer/buyer-messages/drafts/batch-mark-sent',
    );

    const ruleBody = ['name', 'node', 'templateId', 'enabled', 'platforms', 'shopIds', 'backfill'];
    expect(createRule?.requestBody).toEqual(ruleBody);
    expect(updateRule?.requestBody).toEqual(ruleBody);
    const backfillEstimate = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/customer/buyer-message-rules/backfill-estimate',
    );
    expect(backfillEstimate?.query).toEqual(['node', 'platforms', 'shopIds']);
    expect(listDrafts?.query).toEqual(['page', 'pageSize', 'node', 'status', 'platform', 'shopId', 'keyword']);
    expect(editDraft?.requestBody).toEqual(['content']);
    expect(batchMarkSent?.requestBody).toEqual(['ids']);
  });

  it('defines payload/query contracts for order review rule and workbench APIs', () => {
    const createRule = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-review-rules');
    const updateRule = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/order-review-rules/:ruleId');
    const dryRun = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-review-rules/dry-run');
    const workbench = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/order-review');
    const approve = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-review/approve');
    const reject = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-review/reject');

    const ruleBody = [
      'name',
      'priority',
      'enabled',
      'action',
      'minAmount',
      'maxAmount',
      'addressKeywords',
      'remarkKeywords',
      'platforms',
      'shopIds',
      'maxTotalQuantity',
      'maxSkuQuantity',
      'repeatReceiverMinOrders',
      'repeatReceiverWindowDays',
    ];
    expect(createRule?.requestBody).toEqual(ruleBody);
    expect(updateRule?.requestBody).toEqual(ruleBody);
    expect(dryRun?.requestBody).toEqual(ruleBody);
    expect(workbench?.query).toEqual(['page', 'pageSize', 'reviewStatus', 'keyword']);
    expect(approve?.requestBody).toEqual(['orderIds', 'remark']);
    expect(reject?.requestBody).toEqual(['orderIds', 'remark']);
  });

  it('defines query contracts for the selection insights read APIs (GET-only)', () => {
    const insights = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/selection/candidates/:id/insights',
    );
    const trend = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/selection/candidates/:id/price-trend',
    );
    const compare = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/selection/compare');
    const sources = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/selection/market-sources');

    expect(insights?.responseData).toBe('CandidateInsights');
    expect(trend?.responseData).toBe('PriceTrend');
    expect(compare?.query).toEqual(['ids']);
    expect(compare?.responseData).toBe('CompareRow[]');
    expect(sources?.responseData).toBe('MarketSourceStatus[]');
    for (const endpoint of [insights, trend, compare, sources]) {
      expect(endpoint?.method).toBe('GET');
    }
  });

  it('defines payload/query contracts for order automation rule and execution log APIs', () => {
    const createRule = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-automation-rules');
    const updateRule = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/order-automation-rules/:ruleId');
    const dryRun = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/order-automation-rules/dry-run');
    const logs = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/order-automation-logs');
    const retry = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/order-automation-logs/:logId/retry',
    );
    const trail = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/orders/:id/automation-logs',
    );

    const ruleBody = [
      'name',
      'priority',
      'enabled',
      'triggerEvent',
      'action',
      'minAmount',
      'maxAmount',
      'platforms',
      'shopIds',
      'requireReviewPassed',
      'shippingApplyMode',
      'warehouseStrategy',
      'clearMinAmount',
      'clearMaxAmount',
    ];
    expect(createRule?.requestBody).toEqual(ruleBody);
    expect(updateRule?.requestBody).toEqual(ruleBody);
    expect(dryRun?.requestBody).toEqual(ruleBody);
    expect(dryRun?.responseData).toContain('blocked');
    expect(logs?.query).toEqual(['page', 'pageSize', 'status', 'triggerEvent', 'action', 'ruleId', 'keyword']);
    expect(logs?.responseData).toContain('totalPages');
    expect(retry?.responseData).toBe('OrderAutomationLog');
    expect(trail?.responseData).toBe('{ items: OrderAutomationLog[] }');
  });

  it('keeps the MCP read-only surface free of token plaintext and hashes', () => {
    const list = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/mcp/tokens');
    const create = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/mcp/tokens');
    const revoke = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/mcp/tokens/:id/revoke');
    const audit = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/mcp/audit-logs');
    const mcp = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/mcp');

    expect(list?.responseFields).toContain('items[].maskedToken');
    expect(list?.responseFields).not.toContain('items[].plaintext');
    expect(list?.forbiddenResponseFields).toEqual(['items[].plaintext', 'items[].tokenHash']);
    expect(create?.responseFields).toEqual(['token', 'plaintext']);
    expect(create?.note).toContain('exactly once');
    expect(revoke?.forbiddenResponseFields).toEqual(['token.plaintext', 'token.tokenHash']);

    expect(create?.requestBody).toEqual(['name', 'expiresInDays']);
    expect(list?.responseFields).toContain('items[].expiresAt');
    expect(list?.responseFields).toContain('items[].expired');

    expect(audit?.query).toEqual(['page', 'pageSize', 'tool', 'status']);
    expect(audit?.responseFields).toContain('items[].tokenMasked');
    expect(audit?.forbiddenResponseFields).toEqual([
      'items[].plaintext',
      'items[].tokenHash',
      'items[].arguments',
      'items[].result',
    ]);
    expect(audit?.note).toContain('never recorded');

    expect(mcp?.authScheme).toBe('bearer-mcp-readonly-token');
    expect(mcp?.errorEnvelope).toEqual({ 401: 40101, 429: 42901 });
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(119);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});

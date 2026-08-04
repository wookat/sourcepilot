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
        'POST /api/v1/orders/shipments/batch',
        'GET /api/v1/orders/print/sheets',
        'POST /api/v1/orders/:id/shipments/:shipmentId/refresh-tracking',
        'POST /api/v1/imports/parse',
        'POST /api/v1/imports/validate',
        'POST /api/v1/imports/commit',
        'GET /api/v1/imports',
        'GET /api/v1/imports/:id',
        'GET /api/v1/imports/:id/errors.csv',
        'GET /api/v1/reports/profit',
        'GET /api/v1/reports/profit/export.csv',
        'GET /api/v1/reports/procurement',
        'GET /api/v1/reports/inventory',
      ]),
    );
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
    expect(printSheets?.query).toEqual(['ids']);
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
    expect(inventory?.query).toEqual(['slowDays']);
    expect(inventory?.responseData).toBe('InventoryReportDTO');
    for (const endpoint of [profit, profitCsv, procurement, inventory]) {
      expect(endpoint?.method).toBe('GET');
    }
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(26);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});

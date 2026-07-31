import { describe, expect, it } from 'vitest';
import { parseLogisticsText, parsePlacedText, resolvePurchaseOrderId } from '../batchParse';

const ID_A = '3f2a91b0-1111-4222-8333-444455556666';
const ID_B = '3f2a91b0-9999-4888-8777-666655554444';
const ID_C = '8c4d0000-1234-4123-8123-123412341234';
const CANDIDATES = [ID_A, ID_B, ID_C];

describe('resolvePurchaseOrderId', () => {
  it('accepts full UUID directly', () => {
    expect(resolvePurchaseOrderId(ID_A.toUpperCase(), [])).toEqual({ id: ID_A });
  });
  it('resolves a unique prefix', () => {
    expect(resolvePurchaseOrderId('8c4d', CANDIDATES)).toEqual({ id: ID_C });
  });
  it('rejects an ambiguous prefix', () => {
    const r = resolvePurchaseOrderId('3f2a91b0', CANDIDATES);
    expect(r.id).toBeUndefined();
    expect(r.error).toContain('2');
  });
  it('rejects an unknown prefix', () => {
    expect(resolvePurchaseOrderId('deadbeef', CANDIDATES).error).toBeTruthy();
  });
  it('rejects malformed tokens', () => {
    expect(resolvePurchaseOrderId('总仓', CANDIDATES).error).toBeTruthy();
  });
});

describe('parsePlacedText', () => {
  it('parses tab/comma/space separated lines and skips blank lines', () => {
    const text = `${ID_C}\t20260729001\n\n8c4d,20260729002\n`;
    const rows = parsePlacedText(text, CANDIDATES);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toMatchObject({ purchaseOrderId: ID_C, externalOrderId: '20260729001' });
    expect(rows[1]).toMatchObject({ purchaseOrderId: ID_C, externalOrderId: '20260729002' });
  });
  it('marks lines missing the external order id', () => {
    const rows = parsePlacedText(ID_C, CANDIDATES);
    expect(rows[0].error).toBeTruthy();
  });
  it('keeps line numbers for error reporting', () => {
    const rows = parsePlacedText(`\n\nbadline`, CANDIDATES);
    expect(rows[0].line).toBe(3);
    expect(rows[0].error).toBeTruthy();
  });
});

describe('parseLogisticsText', () => {
  it('parses external order id, tracking no and optional carrier', () => {
    const rows = parseLogisticsText('20260729001 SF123 顺丰\n20260729002,YT456');
    expect(rows[0]).toMatchObject({
      externalOrderId: '20260729001',
      trackingNo: 'SF123',
      carrier: '顺丰',
    });
    expect(rows[1]).toMatchObject({ externalOrderId: '20260729002', trackingNo: 'YT456' });
    expect(rows[1].carrier).toBeUndefined();
  });
  it('marks lines missing tracking no', () => {
    const rows = parseLogisticsText('20260729001');
    expect(rows[0].error).toBeTruthy();
  });
});

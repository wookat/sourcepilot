import { describe, expect, it } from 'vitest';
import { groupImportOrders, parseImportText } from '../importParse';

describe('parseImportText', () => {
  it('parses comma/tab separated lines and skips blank lines', () => {
    const text = 'SO-1,王小明,蓝牙耳机,BT-001,2,19.9,USD\n\nSO-2\t李华\t手机支架\tPH-100\t3\t5.5\n';
    const rows = parseImportText(text);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toMatchObject({
      orderNo: 'SO-1',
      customerName: '王小明',
      productTitle: '蓝牙耳机',
      skuCode: 'BT-001',
      quantity: 2,
      unitPrice: 19.9,
      currency: 'USD',
    });
    expect(rows[1]).toMatchObject({ orderNo: 'SO-2', quantity: 3, unitPrice: 5.5 });
    expect(rows[1].currency).toBeUndefined();
  });

  it('marks lines with missing fields', () => {
    const rows = parseImportText('SO-1,王小明,蓝牙耳机');
    expect(rows[0].error).toBeTruthy();
  });

  it('rejects invalid quantity, price and currency', () => {
    expect(parseImportText('SO-1,A,B,C,0,10')[0].error).toContain('数量');
    expect(parseImportText('SO-1,A,B,C,1,abc')[0].error).toContain('单价');
    expect(parseImportText('SO-1,A,B,C,1,10,人民币')[0].error).toContain('币种');
  });
});

describe('groupImportOrders', () => {
  it('groups lines with the same orderNo into one order and sums total', () => {
    const rows = parseImportText(
      'SO-1,王小明,蓝牙耳机,BT-001,2,19.9,USD\nSO-1,王小明,充电仓,BT-001C,1,9.9,USD\nSO-2,李华,手机支架,PH-100,3,5.5',
    );
    const orders = groupImportOrders(rows);
    expect(orders).toHaveLength(2);
    expect(orders[0].orderNo).toBe('SO-1');
    expect(orders[0].items).toHaveLength(2);
    expect(orders[0].totalAmount).toBeCloseTo(49.7);
    expect(orders[1].currency).toBe('USD');
  });

  it('ignores error lines', () => {
    const rows = parseImportText('bad line\nSO-1,A,B,C,1,10');
    expect(groupImportOrders(rows)).toHaveLength(1);
  });
});

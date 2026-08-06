import { describe, expect, it } from 'vitest';

import { localSkuCodeDisplay } from '../localSkuDisplay';

describe('localSkuCodeDisplay', () => {
  it('未绑定行不回显录入 skuCode', () => {
    expect(localSkuCodeDisplay({ skuCode: 'HAND-001' })).toEqual({ bound: false });
    expect(localSkuCodeDisplay({ productSkuId: '', skuCode: 'HAND-001' })).toEqual({
      bound: false,
    });
    expect(localSkuCodeDisplay({ skuCode: 'HAND-001' }, { productSkuId: '  ' })).toEqual({
      bound: false,
    });
  });

  it('匹配记录带 productSkuId 时展示本地规格编号', () => {
    expect(
      localSkuCodeDisplay(
        { skuCode: 'HAND-001' },
        { productSkuId: 'sku-1', localSkuCode: 'LOCAL-001' },
      ),
    ).toEqual({ bound: true, text: 'LOCAL-001' });
  });

  it('行已绑定但匹配记录缺 localSkuCode 时回退行 skuCode，再回退占位符', () => {
    expect(localSkuCodeDisplay({ productSkuId: 'sku-1', skuCode: 'HAND-001' })).toEqual({
      bound: true,
      text: 'HAND-001',
    });
    expect(localSkuCodeDisplay({ productSkuId: 'sku-1' })).toEqual({ bound: true, text: '—' });
  });
});

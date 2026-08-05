import { describe, expect, it } from 'vitest';
import { INVENTORY_CHANGE_REASON, INVENTORY_CHANGE_TYPE } from '../inventoryLabels';
import { productSourceLabel } from '../userFriendly';

describe('migration import labels', () => {
  it('maps import_opening change type and reason to Chinese', () => {
    expect(INVENTORY_CHANGE_TYPE.import_opening).toBe('期初导入');
    expect(INVENTORY_CHANGE_REASON.import_opening).toBe('期初导入');
  });

  it('maps migration product source to Chinese', () => {
    expect(productSourceLabel('migration')).toBe('数据搬家导入');
  });
});

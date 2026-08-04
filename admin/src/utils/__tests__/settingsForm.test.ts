import { describe, expect, it } from 'vitest';

import { toPutItems } from '../settingsForm';

describe('toPutItems', () => {
  it('builds one item per spec key with values stringified', () => {
    const items = toPutItems(
      'ai',
      { api_key: { encrypted: true }, model: {} },
      { api_key: 'sk-1', model: 'gpt' },
    );
    expect(items).toEqual([
      {
        groupKey: 'ai',
        itemKey: 'api_key',
        itemValue: 'sk-1',
        valueType: 'string',
        isEncrypted: true,
        remark: '',
      },
      {
        groupKey: 'ai',
        itemKey: 'model',
        itemValue: 'gpt',
        valueType: 'string',
        isEncrypted: false,
        remark: '',
      },
    ]);
  });

  it('omits tenantId so writes land on the request tenant (non-tenant0 admins must not 403)', () => {
    const items = toPutItems('collector', { timeout: {} }, { timeout: 30 });
    expect(items[0]).not.toHaveProperty('tenantId');
    expect(items[0].itemValue).toBe('30');
  });
});

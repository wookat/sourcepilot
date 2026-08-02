import { describe, expect, it } from 'vitest';
import type { AppConfigFieldDTO } from '../../services/shops';
import {
  PLATFORM_FIELD_GROUP_LABEL,
  groupPlatformAppFields,
  platformFieldGroupKey,
} from '../platformAppConfig';

function field(name: string, type: AppConfigFieldDTO['type'] = 'text'): AppConfigFieldDTO {
  return { name, label: name, type } as AppConfigFieldDTO;
}

describe('platformFieldGroupKey（R66 平台设置分组）', () => {
  it.each([
    ['app_key', 'credentials'],
    ['app_secret', 'credentials'],
    ['client_secret', 'credentials'],
    ['api_base_url', 'endpoints'],
    ['redirect_uri', 'endpoints'],
    ['timeout_sec', 'endpoints'],
    ['gray_shop_ids', 'features'],
    ['order_sync_max_pages', 'features'],
  ])('%s → %s', (name, expected) => {
    expect(platformFieldGroupKey(field(name))).toBe(expected);
  });

  it('未知开关字段归入功能开关组', () => {
    expect(platformFieldGroupKey(field('some_new_toggle', 'switch'))).toBe('features');
  });

  it('未知普通字段归入其他配置组', () => {
    expect(platformFieldGroupKey(field('some_unknown_text'))).toBe('others');
  });

  it('字段名大小写与空白不影响分组', () => {
    expect(platformFieldGroupKey(field(' APP_KEY '))).toBe('credentials');
  });
});

describe('groupPlatformAppFields', () => {
  it('按固定顺序返回非空分组，组内保持字段原有顺序', () => {
    const groups = groupPlatformAppFields([
      field('order_sync_enabled', 'switch'),
      field('app_key'),
      field('api_base_url'),
      field('app_secret', 'password'),
      field('write_operations_enabled', 'switch'),
    ]);
    expect(groups.map((g) => g.key)).toEqual(['credentials', 'endpoints', 'features']);
    expect(groups[0].fields.map((f) => f.name)).toEqual(['app_key', 'app_secret']);
    expect(groups[2].fields.map((f) => f.name)).toEqual([
      'order_sync_enabled',
      'write_operations_enabled',
    ]);
    expect(groups[0].label).toBe(PLATFORM_FIELD_GROUP_LABEL.credentials);
  });

  it('不丢字段：分组字段总数等于输入字段数', () => {
    const input = [
      field('app_key'),
      field('mystery_field'),
      field('timeout_sec', 'number'),
      field('real_api_enabled', 'switch'),
    ];
    const groups = groupPlatformAppFields(input);
    expect(groups.flatMap((g) => g.fields).length).toBe(input.length);
  });

  it('空字段列表返回空分组', () => {
    expect(groupPlatformAppFields([])).toEqual([]);
  });
});

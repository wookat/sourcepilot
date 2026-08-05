import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  deleteImportMappingPreset,
  parseImportFile,
  queryImportMappingPresets,
  saveImportMappingPreset,
} from '../imports';

const requestMock = vi.mocked(request);

describe('imports service', () => {
  it('parses import files for the four kinds', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { kind: 'inventory' } });

    const file = new File(['SKU编码,期初数量\nA,1'], 'inventory.csv', { type: 'text/csv' });
    await parseImportFile('inventory', file);

    const [path, options] = requestMock.mock.calls.at(-1) as [string, { method: string; data: FormData }];
    expect(path).toBe('/api/v1/imports/parse');
    expect(options.method).toBe('POST');
    expect(options.data.get('kind')).toBe('inventory');
  });

  it('lists, saves and deletes mapping presets', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { list: [] } });

    await queryImportMappingPresets('source');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/imports/mappings', {
      method: 'GET',
      params: { kind: 'source' },
    });

    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { id: 'p-1' } });
    await saveImportMappingPreset({
      kind: 'inventory',
      name: '库存方案',
      columns: ['SKU编码'],
      mapping: { skuCode: 0 },
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/imports/mappings', {
      method: 'POST',
      data: { kind: 'inventory', name: '库存方案', columns: ['SKU编码'], mapping: { skuCode: 0 } },
    });

    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { deleted: true } });
    await deleteImportMappingPreset('p-1');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/imports/mappings/p-1', { method: 'DELETE' });
  });
});

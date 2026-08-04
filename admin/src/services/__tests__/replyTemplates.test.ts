import { beforeEach, describe, expect, it, vi } from 'vitest';

const getWithParams = vi.fn();
const postJSON = vi.fn();
const putJSON = vi.fn();
const deleteJSON = vi.fn();

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: (...args: unknown[]) => getWithParams(...args),
  postJSON: (...args: unknown[]) => postJSON(...args),
  putJSON: (...args: unknown[]) => putJSON(...args),
  deleteJSON: (...args: unknown[]) => deleteJSON(...args),
}));

import {
  createReplyTemplate,
  deleteReplyTemplate,
  queryReplyTemplates,
  reorderReplyTemplates,
  updateReplyTemplate,
} from '../customer';

beforeEach(() => {
  getWithParams.mockReset().mockResolvedValue({ list: [], canWrite: true });
  postJSON.mockReset().mockResolvedValue({ ok: true });
  putJSON.mockReset().mockResolvedValue({ ok: true });
  deleteJSON.mockReset().mockResolvedValue({ ok: true });
});

describe('reply template services', () => {
  it('queries with group/keyword/enabled filters', async () => {
    await queryReplyTemplates({ group: 'logistics', keyword: '物流', enabled: true });
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/customer/reply-templates', {
      group: 'logistics',
      keyword: '物流',
      enabled: 'true',
    });
  });

  it('omits the enabled param when undefined', async () => {
    await queryReplyTemplates({});
    expect(getWithParams).toHaveBeenCalledWith('/api/v1/customer/reply-templates', {
      group: undefined,
      keyword: undefined,
      enabled: undefined,
    });
  });

  it('creates via POST with the upsert payload', async () => {
    await createReplyTemplate({ groupKey: 'presale', name: '欢迎语', content: '您好{买家昵称}' });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/reply-templates', {
      groupKey: 'presale',
      name: '欢迎语',
      content: '您好{买家昵称}',
    });
  });

  it('updates via PUT on the id path', async () => {
    await updateReplyTemplate('tpl-1', { enabled: false });
    expect(putJSON).toHaveBeenCalledWith('/api/v1/customer/reply-templates/tpl-1', {
      enabled: false,
    });
  });

  it('deletes via DELETE on the id path', async () => {
    await deleteReplyTemplate('tpl-2');
    expect(deleteJSON).toHaveBeenCalledWith('/api/v1/customer/reply-templates/tpl-2');
  });

  it('reorders via POST with group and ordered ids', async () => {
    await reorderReplyTemplates({ groupKey: 'refund', ids: ['b', 'a'] });
    expect(postJSON).toHaveBeenCalledWith('/api/v1/customer/reply-templates/reorder', {
      groupKey: 'refund',
      ids: ['b', 'a'],
    });
  });
});

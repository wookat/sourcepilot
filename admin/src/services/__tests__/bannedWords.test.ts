import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  batchCheckBannedWords,
  checkProductBannedWords,
  createBannedWord,
  deleteBannedWord,
  listBannedWordCategories,
  listBannedWords,
  toggleBannedWordCategory,
  updateBannedWord,
} from '../bannedWords';

const requestMock = vi.mocked(request);

describe('banned words service', () => {
  it('lists banned words with filters', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { items: [] } });

    await listBannedWords({ category: 'medical', keyword: '治疗', enabled: true });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words', {
      method: 'GET',
      params: { category: 'medical', keyword: '治疗', enabled: '1' },
    });
  });

  it('creates, updates and deletes words via REST endpoints', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: {} });

    await createBannedWord({ word: '全网首发', level: 'forbidden' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words', {
      method: 'POST',
      data: { word: '全网首发', level: 'forbidden' },
    });

    await updateBannedWord('w-1', { enabled: false });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words/w-1', {
      method: 'PUT',
      data: { enabled: false },
    });

    await deleteBannedWord('w-1');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words/w-1', { method: 'DELETE' });
  });

  it('lists and toggles categories with URL encoding', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { items: [] } });

    await listBannedWordCategories();
    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words/categories', {
      method: 'GET',
      params: {},
    });

    await toggleBannedWordCategory('ad extreme', false);
    expect(requestMock).toHaveBeenCalledWith('/api/v1/banned-words/categories/ad%20extreme', {
      method: 'PUT',
      data: { enabled: false },
    });
  });

  it('checks a product and encodes the product ID', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { productId: 'p/1', status: 'passed', forbiddenCount: 0, warningCount: 0, hits: [] },
    });

    await checkProductBannedWords('p/1');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p%2F1/banned-words/check', {
      method: 'GET',
      params: {},
    });
  });

  it('posts batch checks to the shared contract endpoint', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    await batchCheckBannedWords(['p1', 'p2']);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/banned-words/check-batch', {
      method: 'POST',
      data: { productIds: ['p1', 'p2'] },
    });
  });
});

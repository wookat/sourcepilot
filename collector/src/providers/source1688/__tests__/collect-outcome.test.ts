import { describe, expect, it } from 'vitest';

import { isLikelyOfferPath, resolveCollectOutcome } from '../alibaba-1688.js';
import type { Parse1688Result } from '../types.js';

function makeAssembled(overrides: Partial<Parse1688Result> = {}): Parse1688Result {
  return {
    title: '测试商品标题-三层收纳架',
    mainImages: ['https://cbu01.alicdn.com/img/a.jpg'],
    descriptionImages: [],
    attributes: { 材质: '塑料' },
    skus: [{ specs: { 颜色: '白' }, price: 12.5 }],
    raw: { productPrice: 12.5 },
    ...overrides,
  } as Parse1688Result;
}

const noMissing = { title: false, price: false, images: false, sku: false };

describe('isLikelyOfferPath', () => {
  it('accepts offer detail urls', () => {
    expect(isLikelyOfferPath('https://detail.1688.com/offer/123456789.html')).toBe(true);
  });

  it('rejects 1688 homepage after redirect', () => {
    expect(isLikelyOfferPath('https://www.1688.com/')).toBe(false);
    expect(isLikelyOfferPath('https://www.1688.com/?tracelog=404')).toBe(false);
  });
});

describe('resolveCollectOutcome offer-path guard', () => {
  it('fails with offer_not_found when redirected to a non-offer page even if content parsed', () => {
    const homepageLike = makeAssembled({ title: '1688首页-全球领先的采购批发平台' });
    const outcome = resolveCollectOutcome(homepageLike, noMissing, false, false);
    expect(outcome.kind).toBe('failed');
    if (outcome.kind === 'failed') {
      expect(outcome.code).toBe('offer_not_found');
    }
  });

  it('fails as blocked when off offer path and blocked', () => {
    const outcome = resolveCollectOutcome(makeAssembled(), noMissing, true, false);
    expect(outcome.kind).toBe('failed');
    if (outcome.kind === 'failed') {
      expect(outcome.code).toBe('blocked');
    }
  });

  it('still succeeds on offer path with full fields', () => {
    const outcome = resolveCollectOutcome(makeAssembled(), noMissing, false, true);
    expect(outcome.kind).toBe('success');
  });
});

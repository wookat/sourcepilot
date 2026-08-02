import { describe, expect, it } from 'vitest';
import { canReviewOperationTasks } from '../permission';

describe('canReviewOperationTasks', () => {
  it('admin 与 reviewer 具备运营任务审核权限', () => {
    expect(canReviewOperationTasks('admin')).toBe(true);
    expect(canReviewOperationTasks('reviewer')).toBe(true);
  });

  it('operator 与 readonly 不具备运营任务审核权限', () => {
    expect(canReviewOperationTasks('operator')).toBe(false);
    expect(canReviewOperationTasks('readonly')).toBe(false);
  });

  it('profile 权限列表优先于角色默认权限', () => {
    expect(canReviewOperationTasks('readonly', ['operationtask.review'])).toBe(true);
    expect(canReviewOperationTasks('admin', ['product.view'])).toBe(false);
  });
});

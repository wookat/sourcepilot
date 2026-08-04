import { describe, expect, it } from 'vitest';
import { canReviewOperationTasks, isPlatformAdmin } from '../permission';
import { canAccessPath } from '../menuAccess';

describe('isPlatformAdmin / 平台租户路由', () => {
  it('仅 tenant 0 的 admin 是平台管理员', () => {
    expect(isPlatformAdmin('admin', 0)).toBe(true);
    expect(isPlatformAdmin('admin', 1)).toBe(false);
    expect(isPlatformAdmin('operator', 0)).toBe(false);
    expect(isPlatformAdmin('readonly', 0)).toBe(false);
    expect(isPlatformAdmin('admin', undefined)).toBe(false);
  });

  it('平台租户页仅平台管理员可访问', () => {
    expect(canAccessPath('/settings/platform-tenants', 'admin', undefined, 0)).toBe(true);
    expect(canAccessPath('/settings/platform-tenants', 'admin', undefined, 2)).toBe(false);
    expect(canAccessPath('/settings/platform-tenants', 'operator', undefined, 0)).toBe(false);
    expect(canAccessPath('/settings/platform-tenants', 'readonly', undefined, 0)).toBe(false);
  });

  it('备份/恢复/发布/容灾仅平台管理员可访问', () => {
    for (const path of ['/ops/backups', '/ops/restores', '/ops/releases', '/ops/disaster-recovery']) {
      expect(canAccessPath(path, 'admin', undefined, 0)).toBe(true);
      expect(canAccessPath(path, 'admin', undefined, 2)).toBe(false);
      expect(canAccessPath(path, 'operator', undefined, 0)).toBe(false);
    }
  });
});

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

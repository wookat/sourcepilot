import { describe, expect, it } from 'vitest';
import {
  canOperateAnyStore,
  canOperateStore,
  canReviewOperationTasks,
  isPlatformAdmin,
  operableStoreIds,
} from '../permission';
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

  it('提示词模板与部署级运维页仅平台管理员可访问（round103）', () => {
    for (const path of ['/ai/prompts', '/ops/backups', '/ops/restores', '/ops/releases', '/ops/disaster-recovery']) {
      expect(canAccessPath(path, 'admin', undefined, 0)).toBe(true);
      expect(canAccessPath(path, 'admin', undefined, 2)).toBe(false);
      expect(canAccessPath(path, 'operator', undefined, 0)).toBe(false);
    }
  });

  it('报表本位币设置页需要 settings.manage 权限', () => {
    expect(canAccessPath('/settings/report-currency', 'admin', undefined, 2)).toBe(true);
    expect(canAccessPath('/settings/report-currency', 'operator', undefined, 0)).toBe(false);
    expect(canAccessPath('/settings/report-currency', 'readonly', undefined, 0)).toBe(false);
  });

  it('业务租户仍可访问租户隔离的运维页', () => {
    expect(canAccessPath('/ops/workers/monitor', 'admin', undefined, 2)).toBe(true);
    expect(canAccessPath('/ai/tasks', 'admin', undefined, 2)).toBe(true);
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

describe('operableStoreIds / 可写店铺范围（round165）', () => {
  const grants = [
    { storeId: 's-view', permissionScope: 'view' },
    { storeId: 's-op', permissionScope: 'operate' },
    { storeId: 's-manage', permissionScope: 'manage' },
  ];

  it('admin 全部可写；readonly 不可写', () => {
    expect(operableStoreIds('admin', grants)).toBe('all');
    expect(operableStoreIds('readonly', grants)).toEqual([]);
  });

  it('operator 仅 operate/manage 授权店铺可写，view 授权只读', () => {
    expect(operableStoreIds('operator', grants)).toEqual(['s-op', 's-manage']);
    expect(canOperateStore('s-view', 'operator', grants)).toBe(false);
    expect(canOperateStore('s-op', 'operator', grants)).toBe(true);
    expect(canOperateStore('s-view', 'admin', grants)).toBe(true);
  });

  it('仅 view 授权的 operator 无任何可写店铺', () => {
    const viewOnly = [{ storeId: 's-view', permissionScope: 'view' }];
    expect(canOperateAnyStore('operator', viewOnly)).toBe(false);
    expect(canOperateAnyStore('operator', grants)).toBe(true);
    expect(canOperateAnyStore('readonly', grants)).toBe(false);
    expect(canOperateAnyStore('admin', [])).toBe(true);
  });
});

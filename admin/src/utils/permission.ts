export const ROLES = {
  ADMIN: 'admin',
  OPERATOR: 'operator',
  READONLY: 'readonly',
  REVIEWER: 'reviewer',
} as const;

export type AdminRole = (typeof ROLES)[keyof typeof ROLES];

export const PERMISSIONS = {
  PRODUCT_VIEW: 'product.view',
  PRODUCT_WRITE: 'product.write',
  AI_TEXT_APPLY: 'ai_text.apply',
  AI_IMAGE_APPLY: 'ai_image.apply',
  PUBLISH_CREATE_DRAFT: 'publish.create_draft',
  ORDER_VIEW: 'order.view',
  ORDER_OPERATE: 'order.operate',
  SKU_BIND: 'sku.bind',
  INVENTORY_VIEW: 'inventory.view',
  INVENTORY_OPERATE: 'inventory.operate',
  CUSTOMER_VIEW: 'customer.view',
  CUSTOMER_OPERATE: 'customer.operate',
  TASK_RETRY: 'task.retry',
  OPERATION_TASK_AUDIT_READ: 'operationtask.audit.read',
  OPERATION_TASK_EDIT: 'operationtask.edit',
  OPERATION_TASK_EXECUTE: 'operationtask.execute',
  OPERATION_TASK_REVIEW: 'operationtask.review',
  OPERATION_TASK_RETRY: 'operationtask.retry',
  SETTINGS_MANAGE: 'settings.manage',
  USER_MANAGE: 'user.manage',
  OPERATIONLOG_VIEW: 'operationlog.view',
  STORE_VIEW: 'store.view',
  STORE_OPERATE: 'store.operate',
  OBSERVABILITY_READ: 'observability.read',
  BACKUP_READ: 'backup.read',
  BACKUP_CREATE: 'backup.create',
  BACKUP_VERIFY: 'backup.verify',
  BACKUP_HOLD: 'backup.hold',
  BACKUP_DELETE: 'backup.delete',
  RESTORE_READ: 'restore.read',
  RESTORE_EXECUTE: 'restore.execute',
  RESTORE_VERIFY: 'restore.verify',
  RELEASE_READ: 'release.read',
  RELEASE_CREATE: 'release.create',
  RELEASE_EXECUTE: 'release.execute',
  RELEASE_ROLLBACK: 'release.rollback',
  DR_READ: 'dr.read',
  DR_EXECUTE: 'dr.execute',
} as const;

export type PermissionKey = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

const ROLE_PERMISSIONS: Record<string, PermissionKey[]> = {
  admin: Object.values(PERMISSIONS),
  operator: [
    PERMISSIONS.PRODUCT_VIEW,
    PERMISSIONS.PRODUCT_WRITE,
    PERMISSIONS.AI_TEXT_APPLY,
    PERMISSIONS.AI_IMAGE_APPLY,
    PERMISSIONS.PUBLISH_CREATE_DRAFT,
    PERMISSIONS.ORDER_VIEW,
    PERMISSIONS.ORDER_OPERATE,
    PERMISSIONS.SKU_BIND,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.INVENTORY_OPERATE,
    PERMISSIONS.CUSTOMER_VIEW,
    PERMISSIONS.CUSTOMER_OPERATE,
    PERMISSIONS.TASK_RETRY,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_EDIT,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.STORE_OPERATE,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.BACKUP_READ,
    PERMISSIONS.RESTORE_READ,
    PERMISSIONS.RELEASE_READ,
    PERMISSIONS.DR_READ,
  ],
  reviewer: [
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_REVIEW,
    PERMISSIONS.OPERATION_TASK_EXECUTE,
    PERMISSIONS.OPERATION_TASK_RETRY,
  ],
  readonly: [
    PERMISSIONS.PRODUCT_VIEW,
    PERMISSIONS.ORDER_VIEW,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.CUSTOMER_VIEW,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.BACKUP_READ,
    PERMISSIONS.RESTORE_READ,
    PERMISSIONS.RELEASE_READ,
    PERMISSIONS.DR_READ,
  ],
};

export function normalizeRole(role?: string | null): AdminRole {
  const r = (role || '').trim().toLowerCase();
  if (r === ROLES.OPERATOR || r === ROLES.READONLY || r === ROLES.REVIEWER) return r;
  return ROLES.ADMIN;
}

export function permissionsForRole(role?: string | null, fromProfile?: string[]): PermissionKey[] {
  if (fromProfile && fromProfile.length > 0) {
    return fromProfile as PermissionKey[];
  }
  return ROLE_PERMISSIONS[normalizeRole(role)] || ROLE_PERMISSIONS.admin;
}

export function hasPermission(
  role: string | undefined | null,
  perm: PermissionKey,
  fromProfile?: string[],
): boolean {
  return permissionsForRole(role, fromProfile).includes(perm);
}

export function isReadonly(role?: string | null): boolean {
  return normalizeRole(role) === ROLES.READONLY;
}

export function canWriteOrders(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.ORDER_OPERATE, perms);
}

export function canWriteInventory(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.INVENTORY_OPERATE, perms);
}

export function canWriteCustomer(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.CUSTOMER_OPERATE, perms);
}

export function canManageSettings(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.SETTINGS_MANAGE, perms);
}

export function canManageUsers(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.USER_MANAGE, perms);
}

export function canRetryTasks(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.TASK_RETRY, perms);
}

export type StoreScopeGrant = {
  storeId: string;
  permissionScope?: string;
};

/**
 * 可写店铺范围（与后端 adminperm.OperableStoreIDs 口径一致）：
 * admin 返回 'all'；readonly 返回 []；其余角色仅返回 scope 为
 * operate/manage 的授权店铺（scope 为 view 的店铺只读）。
 */
export function operableStoreIds(
  role?: string | null,
  grants?: StoreScopeGrant[] | null,
): 'all' | string[] {
  const r = normalizeRole(role);
  if (r === ROLES.ADMIN) return 'all';
  if (r === ROLES.READONLY) return [];
  return (grants || [])
    .filter((g) => {
      const scope = (g.permissionScope || '').trim().toLowerCase();
      return scope === 'operate' || scope === 'manage';
    })
    .map((g) => g.storeId)
    .filter(Boolean);
}

export function canOperateAnyStore(
  role?: string | null,
  grants?: StoreScopeGrant[] | null,
): boolean {
  const ids = operableStoreIds(role, grants);
  return ids === 'all' || ids.length > 0;
}

export function canOperateStore(
  storeId: string | undefined | null,
  role?: string | null,
  grants?: StoreScopeGrant[] | null,
): boolean {
  const ids = operableStoreIds(role, grants);
  if (ids === 'all') return true;
  if (!storeId) return false;
  return ids.includes(storeId);
}

export function canReadOperationTasks(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.OPERATION_TASK_AUDIT_READ, perms);
}

export function canReviewOperationTasks(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.OPERATION_TASK_REVIEW, perms);
}

/** 平台管理员 = tenant 0 的 admin 账号，可管理平台租户。 */
export function isPlatformAdmin(role?: string | null, tenantId?: number): boolean {
  return normalizeRole(role) === ROLES.ADMIN && tenantId === 0;
}

export const PERMISSION_DENIED_MESSAGE = '当前账号无权限访问此页面';
export const READONLY_DENIED_MESSAGE = '只读账号不可执行写操作';

import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import {
  canManageSettings,
  canManageUsers,
  canOperateAnyStore,
  canOperateStore,
  canRetryTasks,
  canWriteCustomer,
  canWriteInventory,
  canWriteOrders,
  hasPermission,
  isReadonly,
  normalizeRole,
  permissionsForRole,
  type PermissionKey,
} from '@/utils/permission';

export function usePermission() {
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  const role = user?.role;
  const perms = permissionsForRole(role, user?.permissions);

  return {
    user,
    role: normalizeRole(role),
    permissions: perms,
    readonly: isReadonly(role),
    can: (perm: PermissionKey) => hasPermission(role, perm, user?.permissions),
    canWriteOrders: canWriteOrders(role, user?.permissions),
    canWriteInventory: canWriteInventory(role, user?.permissions),
    canWriteCustomer: canWriteCustomer(role, user?.permissions),
    canManageSettings: canManageSettings(role, user?.permissions),
    canManageUsers: canManageUsers(role, user?.permissions),
    canRetryTasks: canRetryTasks(role, user?.permissions),
    storePermissions: user?.storePermissions || [],
    canOperateAnyStore: canOperateAnyStore(role, user?.storePermissions),
    canOperateStore: (storeId?: string | null) =>
      canOperateStore(storeId, role, user?.storePermissions),
  };
}

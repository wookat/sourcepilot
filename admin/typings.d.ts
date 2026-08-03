/// <reference types="@umijs/max/typings" />

declare module '*.less';

declare module '*.png' {
  const url: string;
  export default url;
}

declare namespace API {
  type StorePermission = {
    storeId: string;
    platform?: string;
    permissionScope: string;
  };

  type CurrentUser = {
    id: string;
    username: string; // login identifier (email or phone)
    email?: string;
    phone?: string;
    displayName: string;
    role?: string;
    status?: string;
    tenantId?: number;
    permissions?: string[];
    storePermissions?: StorePermission[];
    createdAt?: string;
    updatedAt?: string;
  };
}

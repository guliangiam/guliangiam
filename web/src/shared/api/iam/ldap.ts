import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import {
  LDAPS,
  LDAP_BY_ID,
  LDAP_PREVIEW_USERS,
  LDAP_SYNC,
  LDAP_TEST_CONNECTION,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// LDAPS
// ─────────────────────────────────────────────
export type IamLdapItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  host: string;
  port: number;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
/** 与后端 app/ldaps.LdapDetail 一致：绑定密码不返回，同步只有 syncInterval。 */
export type IamLdapDetail = IamLdapItem & {
  serverName: string;
  baseDn: string;
  admin: string;
  filter: string;
  filterFields: string[];
  type: string;
  attributeName: string;
  passwordType: string;
  enableSsl: boolean;
  allowSelfSignedCert: boolean;
  defaultGroups: string[];
  customAttributes: Record<string, string>;
  enableGroups: boolean;
  syncInterval: number;
  revision: number;
};
/** 与后端 CreateLdapRequest 一致（绑定账号是 admin/bindPassword，同步只认 syncInterval）。 */
export type IamCreateLdapPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  serverName?: string;
  host: string;
  port?: number;
  admin?: string;
  bindDn?: string;
  bindPassword?: string;
  baseDn?: string;
  type?: string;
  attributeName?: string;
  passwordType?: string;
  filter?: string;
  filterFields?: string[];
  enableSsl?: boolean;
  allowSelfSignedCert?: boolean;
  enableGroups?: boolean;
  defaultGroups?: string[];
  customAttributes?: Record<string, string>;
  isEnabled?: boolean;
  syncInterval?: number;
};
export type IamPatchLdapPayload = {
  displayName?: string;
  serverName?: string;
  host?: string;
  port?: number;
  admin?: string;
  bindDn?: string;
  bindPassword?: string;
  baseDn?: string;
  type?: string;
  attributeName?: string;
  passwordType?: string;
  filter?: string;
  filterFields?: string[];
  enableSsl?: boolean;
  allowSelfSignedCert?: boolean;
  enableGroups?: boolean;
  defaultGroups?: string[];
  customAttributes?: Record<string, string>;
  isEnabled?: boolean;
  syncInterval?: number;
  revision: number;
};

// ─────────────────────────────────────────────
// LDAP SYNC
// ─────────────────────────────────────────────
export type IamLdapPreviewUser = {
  uuid: string;
  uid: string;
  cn: string;
  email: string;
  phone: string;
  givenName: string;
  surname: string;
  title: string;
  department: string;
  dn: string;
  existsLocal: boolean;
};
export type IamLdapPreviewResult = {
  users: IamLdapPreviewUser[];
  existUuids: string[];
  totalCount: number;
  existCount: number;
};
export type IamLdapSyncResult = {
  totalUsers: number;
  addedUsers: number;
  updatedUsers: number;
  failedUsers: number;
  totalGroups: number;
  addedGroups: number;
  failedGroups: number;
  errors?: string[];
};

export const getIamLdaps = (params?: IamListParams) => iamList<IamLdapItem>(LDAPS, params);
export const getIamLdapsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamLdapItem>(LDAPS, params);
export const getIamLdap = (id: string) => iamGet<IamLdapDetail>(LDAP_BY_ID(id));
export const createIamLdap = (p: IamCreateLdapPayload) => iamCreate(LDAPS, p);
export const patchIamLdap = (id: string, p: IamPatchLdapPayload) => iamPatch(LDAP_BY_ID(id), p);
export const deleteIamLdap = (id: string) => iamDelete(LDAP_BY_ID(id));

// LDAP Sync operations
export const getIamLdapPreviewUsers = (id: string) =>
  iamGet<IamLdapPreviewResult>(LDAP_PREVIEW_USERS(id));

export const syncIamLdap = (id: string, payload?: { uuids?: string[] }) =>
  iamCreate(LDAP_SYNC(id), payload ?? {});

export const testIamLdapConnection = (id: string) => iamPostRaw(LDAP_TEST_CONNECTION(id));

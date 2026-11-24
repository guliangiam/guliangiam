import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { KEYS, KEY_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Keys
// ─────────────────────────────────────────────
/** 与 domain/keys.Key 一致（后端只有这一组字段，无 certId/application/user/state）。 */
export type IamKeyItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  keyType: string;
  algorithm: string;
  publicKey: string;
  // 后端读模型不回传私钥（仅创建时写入）；PATCH 留空省略 = 保持不变。
  expiresAt?: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
};
export type IamKeyDetail = IamKeyItem & {
  revision: number;
};
export type IamCreateKeyPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  keyType: string;
  publicKey: string;
  privateKey?: string;
  algorithm?: string;
  expiresAt?: string;
  isEnabled: boolean;
};
export type IamPatchKeyPayload = {
  displayName?: string;
  publicKey?: string;
  privateKey?: string;
  algorithm?: string;
  expiresAt?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamKeys = (params?: IamListParams) => iamList<IamKeyItem>(KEYS, params);
export const getIamKeysPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamKeyItem>(KEYS, params);
export const getIamKey = (id: string) => iamGet<IamKeyDetail>(`${KEY_BY_ID(id)}`);
export const createIamKey = (p: IamCreateKeyPayload) => iamCreate(KEYS, p);
export const patchIamKey = (id: string, p: IamPatchKeyPayload) => iamPatch(`${KEY_BY_ID(id)}`, p);
export const deleteIamKey = (id: string) => iamDelete(`${KEY_BY_ID(id)}`);

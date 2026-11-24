import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { FACE_ID_PROVIDERS, FACE_ID_PROVIDER_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Face ID Providers
// ─────────────────────────────────────────────
export type IamFaceIdProviderItem = {
  id: string;
  organizationId: string;
  name: string;
  providerType: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamFaceIdProviderDetail = IamFaceIdProviderItem & {
  accessKeyId: string;
  accessKeySecretSet: boolean;
  accessKeySecretMasked?: string;
  endpoint: string;
};
export type IamCreateFaceIdProviderPayload = {
  organizationId: string;
  name: string;
  providerType?: string;
  accessKeyId?: string;
  accessKeySecret?: string;
  endpoint?: string;
  isEnabled?: boolean;
};
export type IamPatchFaceIdProviderPayload = {
  accessKeyId?: string;
  accessKeySecret?: string;
  endpoint?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamFaceIdProviders = (params?: IamListParams) =>
  iamList<IamFaceIdProviderItem>(FACE_ID_PROVIDERS, params);
export const getIamFaceIdProvidersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamFaceIdProviderItem>(FACE_ID_PROVIDERS, params);
export const getIamFaceIdProvider = (id: string, organizationId?: string) =>
  iamGet<IamFaceIdProviderDetail>(withOrgQuery(FACE_ID_PROVIDER_BY_ID(id), organizationId));
export const createIamFaceIdProvider = (p: IamCreateFaceIdProviderPayload) =>
  iamCreate(FACE_ID_PROVIDERS, p);
export const patchIamFaceIdProvider = (
  id: string,
  p: IamPatchFaceIdProviderPayload,
  organizationId?: string,
) => iamPatch(withOrgQuery(FACE_ID_PROVIDER_BY_ID(id), organizationId), p);
/** organizationId 仅平台管理员跨组织操作时传入（服务端 trustedOrgID 裁决）。 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export const deleteIamFaceIdProvider = (id: string, organizationId?: string) =>
  iamDelete(withOrgQuery(FACE_ID_PROVIDER_BY_ID(id), organizationId));

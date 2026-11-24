import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { CAPTCHA_PROVIDERS, CAPTCHA_PROVIDER_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Captcha Providers
// ─────────────────────────────────────────────
export type IamCaptchaProviderItem = {
  id: string;
  organizationId: string;
  name: string;
  providerType: string;
  siteKey: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamCaptchaProviderDetail = IamCaptchaProviderItem & {
  secretKeySet: boolean;
  secretKeyMasked?: string;
  /** Vendor extension fields; secret-like values come back masked. */
  config?: Record<string, unknown>;
};
export type IamCreateCaptchaProviderPayload = {
  organizationId: string;
  name: string;
  providerType: string;
  siteKey?: string;
  secretKey?: string;
  config?: Record<string, string | number>;
  isEnabled?: boolean;
};
export type IamPatchCaptchaProviderPayload = {
  name?: string;
  siteKey?: string;
  secretKey?: string;
  config?: Record<string, string | number>;
  isEnabled?: boolean;
  revision: number;
};

export const getIamCaptchaProviders = (params?: IamListParams) =>
  iamList<IamCaptchaProviderItem>(CAPTCHA_PROVIDERS, params);
export const getIamCaptchaProvidersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamCaptchaProviderItem>(CAPTCHA_PROVIDERS, params);
export const getIamCaptchaProvider = (id: string, organizationId?: string) =>
  iamGet<IamCaptchaProviderDetail>(withOrgQuery(CAPTCHA_PROVIDER_BY_ID(id), organizationId));
export const createIamCaptchaProvider = (p: IamCreateCaptchaProviderPayload) =>
  iamCreate(CAPTCHA_PROVIDERS, p);
export const patchIamCaptchaProvider = (
  id: string,
  p: IamPatchCaptchaProviderPayload,
  organizationId?: string,
) => iamPatch(withOrgQuery(CAPTCHA_PROVIDER_BY_ID(id), organizationId), p);
/** organizationId 仅平台管理员跨组织操作时传入（服务端 trustedOrgID 裁决）。 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export const deleteIamCaptchaProvider = (id: string, organizationId?: string) =>
  iamDelete(withOrgQuery(CAPTCHA_PROVIDER_BY_ID(id), organizationId));

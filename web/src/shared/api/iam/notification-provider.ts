import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { NOTIFICATION_PROVIDERS, NOTIFICATION_PROVIDER_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Notification Providers
// ─────────────────────────────────────────────
export type IamNotificationProviderItem = {
  id: string;
  organizationId: string;
  name: string;
  providerType: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamNotificationProviderDetail = IamNotificationProviderItem & {
  webhookUrl: string;
  accessTokenSet: boolean;
  accessTokenMasked?: string;
  /** 非敏感渠道参数（wecom_app: corpId/agentId；wechat_official: appId/templateId）。 */
  config?: Record<string, unknown>;
};
export type IamCreateNotificationProviderPayload = {
  organizationId: string;
  name: string;
  providerType: string;
  webhookUrl?: string;
  accessToken?: string;
  config?: Record<string, unknown>;
  isEnabled?: boolean;
};
export type IamPatchNotificationProviderPayload = {
  webhookUrl?: string;
  accessToken?: string;
  config?: Record<string, unknown>;
  isEnabled?: boolean;
  revision: number;
};

export const getIamNotificationProviders = (params?: IamListParams) =>
  iamList<IamNotificationProviderItem>(NOTIFICATION_PROVIDERS, params);
export const getIamNotificationProvidersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamNotificationProviderItem>(NOTIFICATION_PROVIDERS, params);
export const getIamNotificationProvider = (id: string, organizationId?: string) =>
  iamGet<IamNotificationProviderDetail>(
    withOrgQuery(NOTIFICATION_PROVIDER_BY_ID(id), organizationId),
  );
export const createIamNotificationProvider = (p: IamCreateNotificationProviderPayload) =>
  iamCreate(NOTIFICATION_PROVIDERS, p);
export const patchIamNotificationProvider = (
  id: string,
  p: IamPatchNotificationProviderPayload,
  organizationId?: string,
) => iamPatch(withOrgQuery(NOTIFICATION_PROVIDER_BY_ID(id), organizationId), p);
/** organizationId 仅平台管理员跨组织操作时传入（服务端 trustedOrgID 裁决）。 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export const deleteIamNotificationProvider = (id: string, organizationId?: string) =>
  iamDelete(withOrgQuery(NOTIFICATION_PROVIDER_BY_ID(id), organizationId));

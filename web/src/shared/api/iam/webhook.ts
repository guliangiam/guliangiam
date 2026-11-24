import { iamList, iamGet, iamPost, iamPatch, iamDelete, type IamListParams } from "./client";
import { WEBHOOKS, WEBHOOK_BY_ID } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Webhooks
// ─────────────────────────────────────────────
export type IamWebhookItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  url: string;
  method: string;
  isEnabled: boolean;
  organization: string;
  contentType: string;
  certId: string;
  isError: boolean;
  singleOrgOnly: boolean;
  createdAt: string;
  updatedAt: string;
};
export type IamWebhookDetail = IamWebhookItem & {
  headers: string;
  events: string[];
  customBody: string;
  customHeaders: string;
  // 读模型只回传脱敏投影（XxxSet + XxxMasked）；明文仅在创建响应出现一次。
  secretSet?: boolean;
  secretMasked?: string;
  errorText: string;
  tokenFields: string[];
  objectFields: string[];
  isUserExtended: boolean;
  maxRetries: number;
  retryInterval: number;
  useExponentialBackoff: boolean;
  timeoutMs: number;
  retryCount: number;
  revision: number;
};
export type IamCreateWebhookPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  url: string;
  method?: string;
  headers?: string;
  events?: string[];
  isEnabled?: boolean;
  retryCount?: number;
  timeoutMs?: number;
  organization?: string;
  contentType?: string;
  customBody?: string;
  customHeaders?: string;
  certId?: string;
  tokenFields?: string[];
  objectFields?: string[];
  isUserExtended?: boolean;
  singleOrgOnly?: boolean;
  maxRetries?: number;
  retryInterval?: number;
  useExponentialBackoff?: boolean;
};
export type IamPatchWebhookPayload = {
  displayName?: string;
  url?: string;
  method?: string;
  headers?: string;
  events?: string[];
  isEnabled?: boolean;
  secret?: string;
  retryCount?: number;
  timeoutMs?: number;
  revision: number;
  organization?: string;
  contentType?: string;
  customBody?: string;
  customHeaders?: string;
  certId?: string;
  tokenFields?: string[];
  objectFields?: string[];
  isUserExtended?: boolean;
  singleOrgOnly?: boolean;
  maxRetries?: number;
  retryInterval?: number;
  useExponentialBackoff?: boolean;
};

/** 创建响应信封：secretPlain 只在此出现一次，关闭弹窗后不可再取回。 */
export type IamCreateWebhookResponse = {
  status: string;
  msg: string;
  data?: { secretPlain?: string };
};

export const getIamWebhooks = (params?: IamListParams) => iamList<IamWebhookItem>(WEBHOOKS, params);
export const getIamWebhook = (id: string) => iamGet<IamWebhookDetail>(WEBHOOK_BY_ID(id));
export const createIamWebhook = (p: IamCreateWebhookPayload) =>
  iamPost<IamCreateWebhookResponse>(WEBHOOKS, p);
export const patchIamWebhook = (id: string, p: IamPatchWebhookPayload) =>
  iamPatch(WEBHOOK_BY_ID(id), p);
export const deleteIamWebhook = (id: string) => iamDelete(WEBHOOK_BY_ID(id));

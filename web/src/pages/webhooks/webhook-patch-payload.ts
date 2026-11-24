import type { IamPatchWebhookPayload } from "@/shared/api/iam";

export type WebhookFormValues = {
  displayName: string;
  organization: string;
  url: string;
  method: string;
  contentType: string;
  headers: string;
  events: string[];
  customBody: string;
  customHeaders: string;
  certId: string;
  secret: string;
  isUserExtended: boolean;
  tokenFields: string[];
  singleOrgOnly: boolean;
  objectFields: string[];
  maxRetries: number;
  retryInterval: number;
  useExponentialBackoff: boolean;
  timeoutMs: number;
  isEnabled: boolean;
};

/**
 * BE-B-7: the detail endpoint only returns a masked secret projection
 * (secretSet/secretMasked), so the form starts blank. PATCH semantics are
 * "omit = keep, non-nil empty = clear": submitting the blank init value on
 * every save would wipe the stored signing secret. Only send secret when the
 * operator actually typed a replacement. Headers are real detail data now and
 * are always submitted.
 */
export function buildWebhookPatchPayload(
  values: WebhookFormValues,
  revision: number,
): IamPatchWebhookPayload {
  const payload: IamPatchWebhookPayload = {
    displayName: values.displayName,
    organization: values.organization,
    url: values.url,
    method: values.method,
    contentType: values.contentType,
    headers: values.headers,
    events: values.events,
    customBody: values.customBody,
    customHeaders: values.customHeaders,
    certId: values.certId,
    isUserExtended: values.isUserExtended,
    tokenFields: values.tokenFields,
    singleOrgOnly: values.singleOrgOnly,
    objectFields: values.objectFields,
    maxRetries: values.maxRetries,
    retryInterval: values.retryInterval,
    useExponentialBackoff: values.useExponentialBackoff,
    timeoutMs: values.timeoutMs,
    isEnabled: values.isEnabled,
    revision,
  };
  if ((values.secret ?? "").trim() !== "") {
    payload.secret = values.secret;
  }
  return payload;
}

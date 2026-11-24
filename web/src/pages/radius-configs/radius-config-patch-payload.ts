import type { IamPatchRadiusConfigPayload } from "@/shared/api/iam/radius-config";
import { optionalCredential } from "@/shared/lib/credential-payload";

/** RadiusConfigDetailPage 的表单值。凭据不回显，未输入时为 undefined。 */
export type RadiusConfigFormValues = {
  host: string;
  port: number;
  sharedSecret?: string;
  isEnabled: boolean;
};

/**
 * 详情读接口只返回 `sharedSecretSet` / `sharedSecretMasked`，sharedSecret 输入框
 * 从不预填。空输入表示“保持已存凭据”，必须从 PATCH 请求体中省略；空串会被后端
 * 当作替换值并清空已存凭据。
 */
export function buildRadiusConfigPatchPayload(
  values: RadiusConfigFormValues,
  revision: number,
): IamPatchRadiusConfigPayload {
  const sharedSecret = optionalCredential(values.sharedSecret);
  return {
    host: values.host,
    port: values.port,
    ...(sharedSecret ? { sharedSecret } : {}),
    isEnabled: values.isEnabled,
    revision,
  };
}

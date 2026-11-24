import type { IamPatchFaceIdProviderPayload } from "@/shared/api/iam/face-id-provider";
import { optionalCredential } from "@/shared/lib/credential-payload";

/** FaceIdProviderDetailPage 的表单值。凭据不回显，未输入时为 undefined。 */
export type FaceIdProviderFormValues = {
  accessKeyId: string;
  accessKeySecret?: string;
  endpoint: string;
  isEnabled: boolean;
};

/**
 * 详情读接口只返回 `accessKeySecretSet` / `accessKeySecretMasked`，accessKeySecret
 * 输入框从不预填。空输入表示“保持已存凭据”，必须从 PATCH 请求体中省略；空串会
 * 被后端当作替换值并清空已存凭据。
 */
export function buildFaceIdProviderPatchPayload(
  values: FaceIdProviderFormValues,
  revision: number,
): IamPatchFaceIdProviderPayload {
  const accessKeySecret = optionalCredential(values.accessKeySecret);
  return {
    accessKeyId: values.accessKeyId,
    ...(accessKeySecret ? { accessKeySecret } : {}),
    endpoint: values.endpoint,
    isEnabled: values.isEnabled,
    revision,
  };
}

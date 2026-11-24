import type { IamMcpServerDetail, IamPatchMcpServerPayload } from "@/shared/api/iam/mcp-server";
import { optionalCredential } from "@/shared/lib/credential-payload";

/** McpServerDetailPage 的表单值。凭据不回显，未输入时为 undefined。 */
export type McpServerFormValues = {
  displayName: string;
  url?: string;
  token?: string;
  application?: string;
  isEnabled: boolean;
};

/**
 * 详情读接口只返回 `tokenSet` / `tokenMasked`，token 输入框从不预填：表单初始值
 * 必须显式排除 token，否则一次无关的编辑会把占位展示值当成新凭据提交。
 */
export function buildMcpServerFormValues(detail: IamMcpServerDetail): McpServerFormValues {
  return {
    displayName: detail.displayName,
    url: detail.url,
    application: detail.application,
    isEnabled: detail.isEnabled,
  };
}

/**
 * 空输入表示“保持已存 token”，必须从 PATCH 请求体中省略；空串会被后端当作替换
 * 值并清空已存凭据。只有用户显式输入新值时，才发送 trim 后的替换值。
 */
export function buildMcpServerPatchPayload(
  values: McpServerFormValues,
  revision: number,
): IamPatchMcpServerPayload {
  const token = optionalCredential(values.token);
  return {
    displayName: values.displayName,
    url: values.url,
    ...(token ? { token } : {}),
    application: values.application,
    isEnabled: values.isEnabled,
    revision,
  };
}

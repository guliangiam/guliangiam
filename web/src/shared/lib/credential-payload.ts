/**
 * 归一化编辑表单里的 write-only 凭据字段。
 *
 * 详情读接口只返回 `*Set` / `*Masked`，凭据输入框从空值开始。PATCH 里省略 key
 * 表示“保持已存凭据”，而非空字符串会被后端当作替换值；因此空串、纯空白和
 * `undefined` 都必须归一为 `undefined`，由调用方从请求体中省略。
 *
 * 语义与 `NotificationProviderDetailPage` 的 `accessToken?.trim()` 及
 * `buildLdapPatchPayload` 保持一致：trim 后为空即视为未填写，非空时发送 trim
 * 后的新凭据。
 */
export function optionalCredential(value: string | null | undefined): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

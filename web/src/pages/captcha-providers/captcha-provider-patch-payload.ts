import type { IamPatchCaptchaProviderPayload } from "@/shared/api/iam/captcha-provider";
import { optionalCredential } from "@/shared/lib/credential-payload";
import { CAPTCHA_CONFIG_FIELDS } from "@/shared/ui/ProviderFields/captcha-constants";

/** CaptchaProviderDetailPage 的表单值。凭据不回显，未输入时为 undefined。 */
export type CaptchaProviderFormValues = {
  name: string;
  siteKey: string;
  secretKey?: string;
  isEnabled: boolean;
  /** config 扩展字段以 spec.formName 为键动态存放（captchaSceneId 等）。 */
  [configFormName: string]: unknown;
};

/**
 * 后端 `mergeConfig` 对 config 是「incoming 键集合覆盖既有键集合」：
 * 未出现在请求体中的既有键会被删除。因此当前类型的所有扩展键都必须发送：
 * - password 类（后端视为密钥）：回显掩码串或留空 = 保持原值，粘贴新值 = 替换；
 * - text 类：空串 = 清空既有值；
 * - number 类：未填 = 省略键（删除既有值），填写 = 数值。
 */
export function buildCaptchaConfigPatch(
  kind: string,
  values: Record<string, unknown>,
): Record<string, string | number> {
  const out: Record<string, string | number> = {};
  for (const spec of CAPTCHA_CONFIG_FIELDS[kind] ?? []) {
    const raw = values[spec.formName];
    if (spec.input === "number") {
      if (typeof raw === "number" && Number.isFinite(raw)) out[spec.configKey] = raw;
      continue;
    }
    out[spec.configKey] = typeof raw === "string" ? raw.trim() : "";
  }
  return out;
}

/**
 * 详情读接口只返回 `secretKeySet` / `secretKeyMasked`，secretKey 输入框从不预填。
 * 空输入表示“保持已存凭据”，必须从 PATCH 请求体中省略；空串会被后端当作替换值
 * 并清空已存凭据。
 */
export function buildCaptchaProviderPatchPayload(
  values: CaptchaProviderFormValues,
  revision: number,
  providerType?: string,
): IamPatchCaptchaProviderPayload {
  const secretKey = optionalCredential(values.secretKey);
  const payload: IamPatchCaptchaProviderPayload = {
    name: values.name,
    siteKey: values.siteKey,
    ...(secretKey ? { secretKey } : {}),
    isEnabled: values.isEnabled,
    revision,
  };
  if (providerType && (CAPTCHA_CONFIG_FIELDS[providerType]?.length ?? 0) > 0) {
    payload.config = buildCaptchaConfigPatch(providerType, values);
  }
  return payload;
}

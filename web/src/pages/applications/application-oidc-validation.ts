import type { ApplicationConfigFormValues } from "./tabs/types";
import { isClientSecretAuthMethod, isMTLSAuthMethod } from "./tabs/types";

/**
 * OIDC/OAuth 跨字段校验的纯逻辑层。
 *
 * 这里只产出与语言无关的问题码（`application.validation.<code>`），
 * 由保存链路与详情页实时提示共用，避免两套规则漂移。
 */

export type OidcIssueCode =
  | "noneRequiresAuthorizationCode"
  | "noneForbidsClientCredentials"
  | "secretNotAllowedForMethod"
  | "jwksNotAllowedForMethod"
  | "jwksUriNotAllowedForMethod"
  | "jwksUriInvalid"
  | "fingerprintsNotAllowedForMethod"
  | "jwksRequired"
  | "certificateRequired"
  | "mtlsCertificateRequired";

export type OidcIssue = {
  code: OidcIssueCode;
  /** 占位参数，例如 `{ method: "none" }`。 */
  params?: Record<string, string>;
};

export type OidcValidationOptions = {
  requireClientJwks?: boolean;
  /** 已持久化的认证方式；新建时为 undefined。 */
  persistedAuthMethod?: ApplicationConfigFormValues["tokenEndpointAuthMethod"];
  /** 已持久化的授权类型，用于判断本次保存是否触碰了 OIDC 组合。 */
  persistedGrantTypes?: string[];
  /** 已登记的客户端证书数量。 */
  existingCertificateCount?: number;
};

function normalizeList(values?: string[]): string[] {
  return (values ?? []).map((value) => value.trim()).filter((value) => value !== "");
}

export function sameStringSet(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false;
  const rightSet = new Set(right);
  return left.every((value) => rightSet.has(value));
}

/**
 * 与后端 validateClientJWKSURI 同一拒绝口径：≤2048、绝对 HTTPS、
 * 有主机、无 userinfo、无 fragment。空串合法（表示未配置/清除）。
 */
export function isValidClientJwksUri(raw: string): boolean {
  const trimmed = raw.trim();
  if (trimmed === "") return true;
  if (trimmed.length > 2048) return false;
  try {
    const url = new URL(trimmed);
    return (
      url.protocol === "https:" &&
      url.hostname !== "" &&
      url.username === "" &&
      url.password === "" &&
      url.hash === ""
    );
  } catch {
    return false;
  }
}

/**
 * 计算当前表单值在 OIDC 维度上的全部问题。
 *
 * 历史遗留的 `none` + 不可兑换授权类型仍然可编辑其他字段：只有本次提交
 * 触碰了认证方式/授权类型（或就是新建）时才强制 `none` 的闭环组合。
 */
export function evaluateOidcIssues(
  values: ApplicationConfigFormValues,
  options?: OidcValidationOptions,
): OidcIssue[] {
  const issues: OidcIssue[] = [];
  const method = values.tokenEndpointAuthMethod;
  const secret = (values.clientSecret ?? "").trim();
  const jwks = (values.jwks ?? "").trim();
  const jwksUri = (values.jwksUri ?? "").trim();
  const certificates = values.mtlsCertificates ?? [];
  const grants = normalizeList(values.grantTypes);
  const touchesOidcCombination =
    options?.persistedAuthMethod === undefined ||
    method !== options.persistedAuthMethod ||
    !sameStringSet(grants, normalizeList(options.persistedGrantTypes));

  if (method === "none" && touchesOidcCombination) {
    if (!grants.includes("authorization_code")) {
      issues.push({ code: "noneRequiresAuthorizationCode" });
    }
    if (grants.includes("client_credentials")) {
      issues.push({ code: "noneForbidsClientCredentials" });
    }
  }

  if (!isClientSecretAuthMethod(method) && secret !== "") {
    issues.push({ code: "secretNotAllowedForMethod", params: { method } });
  }
  if (method !== "private_key_jwt" && jwks !== "") {
    issues.push({ code: "jwksNotAllowedForMethod", params: { method } });
  }
  if (method !== "private_key_jwt" && jwksUri !== "") {
    issues.push({ code: "jwksUriNotAllowedForMethod", params: { method } });
  }
  if (method === "private_key_jwt" && jwksUri !== "" && !isValidClientJwksUri(jwksUri)) {
    issues.push({ code: "jwksUriInvalid" });
  }
  if (
    !isMTLSAuthMethod(method) &&
    certificates.some((entry) => (entry.certificate ?? "").trim() !== "")
  ) {
    issues.push({ code: "fingerprintsNotAllowedForMethod", params: { method } });
  }
  if (
    method === "private_key_jwt" &&
    jwks === "" &&
    jwksUri === "" &&
    options?.requireClientJwks !== false
  ) {
    issues.push({ code: "jwksRequired" });
  }
  if (isMTLSAuthMethod(method)) {
    for (const entry of certificates) {
      if ((entry.certificate ?? "").trim() === "") {
        issues.push({ code: "certificateRequired" });
        break;
      }
    }
    const provided = certificates.filter((entry) => (entry.certificate ?? "").trim() !== "").length;
    if (provided === 0) {
      // 编辑已登记的 mTLS 客户端且认证方式未变时可以保留证书；
      // 其他路径会持久化一个永远无法在令牌端点认证的客户端。
      const keepsRegisteredCertificates =
        options?.persistedAuthMethod === method && (options.existingCertificateCount ?? 0) > 0;
      if (!keepsRegisteredCertificates) {
        issues.push({ code: "mtlsCertificateRequired" });
      }
    }
  }
  return issues;
}

/**
 * 自定义作用域的常用预置项。选择后自动补全显示名称与描述（两者都为空时）。
 */
export const CUSTOM_SCOPE_PRESETS = [
  { scope: "openid" },
  { scope: "profile" },
  { scope: "email" },
  { scope: "phone" },
  { scope: "address" },
  { scope: "offline_access" },
  { scope: "read" },
  { scope: "write" },
] as const;

export type CustomScopePreset = (typeof CUSTOM_SCOPE_PRESETS)[number]["scope"];

export function findCustomScopePreset(scope: string): CustomScopePreset | undefined {
  return CUSTOM_SCOPE_PRESETS.find((preset) => preset.scope === scope)?.scope;
}

/** 预置项的显示名称 i18n key。 */
export function customScopePresetNameKey(scope: CustomScopePreset): string {
  return `application.customScopePresets.${scope}.displayName`;
}

/** 预置项的描述 i18n key。 */
export function customScopePresetDescriptionKey(scope: CustomScopePreset): string {
  return `application.customScopePresets.${scope}.description`;
}

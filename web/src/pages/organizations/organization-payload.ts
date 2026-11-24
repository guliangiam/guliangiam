/**
 * Organization 详情页的数据装配
 *
 * 从 OrganizationDetailPage.tsx 拆分而来，承担两个方向的纯转换，均无副作用：
 *   - 读取方向：`OrganizationDetail` → 表单值（`buildOrganizationFormValues`）
 *   - 提交方向：表单值 → 四个 API 请求体（`build*Payload`）
 *
 * 字段映射与默认值必须与后端契约一致：请求体形状以
 * `@/shared/api/iam/organization` 的 Payload 类型为唯一事实源；后端实际读取的
 * 字段见 `internal/transport/http/api/v1/organizations/request.go`。
 */
import type {
  OrganizationDetail,
  UpdateOrganizationPayload,
  UpdateOrganizationSettingsPayload,
  UpdateOrganizationThemePayload,
  UpdateOrganizationSecurityPolicyPayload,
} from "@/shared/api/iam/organization";
import type { OrganizationFormValues } from "./tabs/types";
import { normalizeOrganizationStatus } from "./organization-status";
import {
  formatJson,
  getFlexibleBoolean,
  getNestedObject,
  getStringArray,
  parseJsonObject,
} from "./lib";

/**
 * 表单可读默认值。
 *
 * 仅当服务端返回空集合时注入，让新组织/未配置组织一打开就能看懂字段用途；
 * 用户保存过的非空配置永远优先。
 */
export const DEFAULT_COUNTRY_CODES = ["CN", "US", "SG", "GB", "DE", "IN"];

function withDefault<T>(value: T[], fallback: T[]): T[] {
  return value.length > 0 ? value : [...fallback];
}

const DEFAULT_LANGUAGES = ["zh-CN", "en"];

const DEFAULT_USER_TYPES = ["internal", "external", "partner"];

const DEFAULT_TAGS = ["default"];

const DEFAULT_ACCOUNT_ITEMS = ["Name", "Display name", "Email", "Phone", "Groups", "Roles"].map(
  (name) => ({
    name,
    visible: true,
    viewRule: "Public",
    modifyRule: "Self",
    regex: "",
    tab: "",
  }),
);

const DEFAULT_MFA_ITEMS = [
  { name: "Email", rule: "Optional" },
  { name: "TOTP", rule: "Optional" },
];

/**
 * 后端 `PasswordPolicy` 的 10 个标准键（internal/domain/organizations/password_policy.go）。
 *
 * 详情页「登录与密码」分区用标准表单控件承载这些键；扩展 JSON 只允许出现
 * 非标准键，读取时剔除、提交时先删后写，保证两处不重复配置同一策略。
 */
export const PASSWORD_POLICY_STANDARD_KEYS = [
  "minLength",
  "maxLength",
  "requireUppercase",
  "requireLowercase",
  "requireDigit",
  "requireSpecial",
  "rejectCommonPasswords",
  "rejectUsername",
  "passwordHistoryCount",
  "passwordMaxAgeDays",
] as const;

/** 表单缺省值：与服务端未配置策略时的展示口径一致（用户规格）。 */
const policyBool = (value: unknown): boolean => (typeof value === "boolean" ? value : true);
const policyNumber = (value: unknown, fallback: number): number =>
  typeof value === "number" ? value : fallback;

/**
 * 读取方向：把详情响应展平为表单值。
 *
 * `settings` / `theme` / `securityPolicySummary` 是三个异构嵌套对象，故先用
 * `getNestedObject` 取出子对象，再逐个字段做类型归一化；无法识别的值一律回落到
 * 与该字段语义一致的默认值（空串 / 空数组 / 0 / false）。
 */
export function buildOrganizationFormValues(detail: OrganizationDetail): OrganizationFormValues {
  const settings = detail.settings;
  const theme = detail.theme;
  const securityPolicy = detail.securityPolicySummary;
  const passwordPolicy = getNestedObject(securityPolicy, "passwordPolicy");
  const mfaPolicy = getNestedObject(securityPolicy, "mfaPolicy");
  const sessionPolicy = getNestedObject(securityPolicy, "sessionPolicy");
  const featureFlags = getNestedObject(settings, "featureFlags");
  const {
    minLength,
    maxLength,
    requireUppercase,
    requireLowercase,
    requireDigit,
    requireSpecial,
    rejectCommonPasswords,
    rejectUsername,
    passwordHistoryCount,
    passwordMaxAgeDays,
    ...passwordPolicyRest
  } = passwordPolicy;
  const { required, ...mfaPolicyRest } = mfaPolicy;
  const { idleTimeoutMinutes, ...sessionPolicyRest } = sessionPolicy;
  const { iam, ...featureFlagsRest } = featureFlags;

  return {
    parentId: detail.parentId ?? "",
    regionCode: detail.regionCode ?? "",
    slug: detail.slug,
    displayName: detail.displayName,
    websiteUrl: detail.websiteUrl,
    logoUrl: detail.logoUrl,
    logoDarkUrl: detail.logoDarkUrl,
    faviconUrl: detail.faviconUrl,
    status: normalizeOrganizationStatus(detail.status),
    passwordType: detail.passwordType ?? "argon2id",
    // 凭据字段是 write-only：读接口只回 `*Set` / `*Masked`，永不回明文，因此表单
    // 一律从空值开始（masked 值只作展示，不作为输入值）。留空 = 保持当前值。
    passwordSalt: "",
    passwordObfuscatorType: detail.passwordObfuscatorType ?? "Plain",
    passwordObfuscatorKey: "",
    // passwordOptions 不做默认注入：空集合就是「未启用任何密码选项」，
    // 静默补默认值会在保存时把操作者未选择的策略写进组织。
    passwordOptions: detail.passwordOptions ?? [],
    defaultPassword: "",
    enableSoftDeletion: detail.enableSoftDeletion ?? false,
    ipWhitelist: detail.ipWhitelist ?? "",
    initiateLoginUrl: detail.initiateLoginUrl ?? "",
    balance: detail.balance ?? 0,
    currency: detail.currency ?? "USD",
    disableSignin: detail.disableSignin ?? false,
    enableDeposit: detail.enableDeposit ?? false,
    balanceCredit: detail.balanceCredit ?? 0,
    mu: detail.mu ?? 0,
    defaultQuota: detail.defaultQuota ?? 0,
    defaultCurrency: detail.defaultCurrency ?? "CNY",
    initScore: detail.initScore ?? 0,
    defaultAvatar: detail.defaultAvatar ?? "",
    enableDarkLogo: detail.enableDarkLogo ?? false,
    hasPrivilegeConsent: detail.hasPrivilegeConsent ?? false,
    ldapAttributes: Array.isArray(detail.ldapAttributes)
      ? detail.ldapAttributes.filter((a: unknown) => typeof a === "string")
      : [],
    languages: withDefault(getStringArray(settings.languages), DEFAULT_LANGUAGES),
    defaultApplicationId:
      typeof settings.defaultApplicationId === "string" ? settings.defaultApplicationId : undefined,
    countryCodes: withDefault(getStringArray(settings.countryCodes), DEFAULT_COUNTRY_CODES),
    userTypes: withDefault(getStringArray(settings.userTypes), DEFAULT_USER_TYPES),
    tags: withDefault(getStringArray(settings.tags), DEFAULT_TAGS),
    accountItems:
      Array.isArray(settings.accountItems) && settings.accountItems.length > 0
        ? settings.accountItems.map((item: Record<string, unknown>) => ({
            name: typeof item?.name === "string" ? item.name : "",
            visible: typeof item?.visible === "boolean" ? item.visible : true,
            viewRule: typeof item?.viewRule === "string" ? item.viewRule : "",
            modifyRule: typeof item?.modifyRule === "string" ? item.modifyRule : "",
            regex: typeof item?.regex === "string" ? item.regex : "",
            tab: typeof item?.tab === "string" ? item.tab : "",
          }))
        : DEFAULT_ACCOUNT_ITEMS.map((item) => ({ ...item })),
    mfaItems:
      Array.isArray(settings.mfaItems) && settings.mfaItems.length > 0
        ? settings.mfaItems.map((item: Record<string, unknown>) => ({
            name: typeof item?.name === "string" ? item.name : "",
            rule: typeof item?.rule === "string" ? item.rule : "",
          }))
        : DEFAULT_MFA_ITEMS.map((item) => ({ ...item })),
    masterPassword: "",
    masterVerificationCode: "",
    isProfilePublic: detail.isProfilePublic ?? false,
    useEmailAsUsername: detail.useEmailAsUsername ?? false,
    usePermanentAvatar: detail.usePermanentAvatar ?? false,
    accountMenu: typeof settings.accountMenu === "string" ? settings.accountMenu : "",
    mfaRememberInHours:
      typeof settings.mfaRememberInHours === "number" ? settings.mfaRememberInHours : 12,
    enableTour: detail.enableTour ?? false,
    kerberosRealm: detail.kerberosRealm ?? "",
    kerberosKdcHost: detail.kerberosKdcHost ?? "",
    kerberosServiceName: detail.kerberosServiceName ?? "",
    // keytab 是 write-only 凭据：输入从空开始，留空即保持原值。
    kerberosKeytab: "",
    navPreferencesJson: formatJson(getNestedObject(settings, "navPreferences")),
    featureFlag: getFlexibleBoolean(iam),
    featureFlagsExtraJson: formatJson(featureFlagsRest),
    // 后端 validateUpdateOrganizationThemeCommand 要求 themeName 非空；新建组织
    // 没有主题记录，读接口回空串。若原样带入表单会触发必填校验、保存无法提交，
    // 故这里用与 buildOrganizationThemePayload 相同的 "default" 兜底。
    themeName:
      typeof theme.themeName === "string" && theme.themeName.trim() !== ""
        ? theme.themeName
        : "default",
    themeTokensJson: formatJson(getNestedObject(theme, "tokens")),
    themeCustomCss: typeof theme.customCss === "string" ? theme.customCss : "",
    themeAssetsJson: formatJson(getNestedObject(theme, "assets")),
    passwordMinLength: policyNumber(minLength, 8),
    passwordMaxLength: policyNumber(maxLength, 128),
    passwordExpireDays: policyNumber(passwordMaxAgeDays, 0),
    passwordHistoryCount: policyNumber(passwordHistoryCount, 5),
    passwordRequireUppercase: policyBool(requireUppercase),
    passwordRequireLowercase: policyBool(requireLowercase),
    passwordRequireDigit: policyBool(requireDigit),
    passwordRequireSpecial: policyBool(requireSpecial),
    passwordRejectCommonPasswords: policyBool(rejectCommonPasswords),
    passwordRejectUsername: policyBool(rejectUsername),
    passwordPolicyExtraJson: formatJson(passwordPolicyRest),
    mfaRequired: getFlexibleBoolean(required),
    mfaPolicyExtraJson: formatJson(mfaPolicyRest),
    ipPolicyJson: formatJson(getNestedObject(securityPolicy, "ipPolicy")),
    sessionIdleTimeoutMinutes:
      typeof idleTimeoutMinutes === "number" ? idleTimeoutMinutes : undefined,
    sessionPolicyExtraJson: formatJson(sessionPolicyRest),
    riskPolicyJson: formatJson(getNestedObject(securityPolicy, "riskPolicy")),
    passwordDeriveAlgorithm:
      typeof securityPolicy.passwordDeriveAlgorithm === "string"
        ? securityPolicy.passwordDeriveAlgorithm
        : "",
    dcrPolicy: typeof settings.dcrPolicy === "string" ? settings.dcrPolicy : "",
  };
}

/**
 * 凭据字段是 write-only：读接口只回 `*Set` / `*Masked`，表单也因此从空值开始。
 *
 * 因此 PATCH 只在操作者本次真正填写了非空值时才带上该 key；省略 key 时后端把
 * nil 解释为「保持不变」。空串**不能**下发——后端把非 nil 的 "" 当作「覆盖为
 * 空」，会把读接口不再回显的凭据静默清掉。
 */
export const ORGANIZATION_CREDENTIAL_FIELDS = [
  "passwordSalt",
  "passwordObfuscatorKey",
  "defaultPassword",
  "masterPassword",
  "masterVerificationCode",
  "kerberosKeytab",
] as const satisfies readonly (keyof OrganizationFormValues & keyof UpdateOrganizationPayload)[];

type CredentialPayloadFields = Pick<
  UpdateOrganizationPayload,
  (typeof ORGANIZATION_CREDENTIAL_FIELDS)[number]
>;

function buildCredentialPatch(values: OrganizationFormValues): CredentialPayloadFields {
  const patch: CredentialPayloadFields = {};
  for (const field of ORGANIZATION_CREDENTIAL_FIELDS) {
    const value = values[field];
    if (value !== undefined && value !== "") patch[field] = value;
  }
  return patch;
}

/** 提交方向：组织主体字段（`PUT /organizations/{id}`）。 */
export function buildOrganizationUpdatePayload(
  values: OrganizationFormValues,
  revision: number,
): UpdateOrganizationPayload {
  return {
    parentId: values.parentId ?? "",
    regionCode: values.regionCode ?? "",
    displayName: values.displayName ?? "",
    websiteUrl: values.websiteUrl ?? "",
    logoUrl: values.logoUrl ?? "",
    logoDarkUrl: values.logoDarkUrl ?? "",
    faviconUrl: values.faviconUrl ?? "",
    status: values.status ?? "",
    passwordType: values.passwordType ?? "argon2id",
    ...buildCredentialPatch(values),
    passwordObfuscatorType: values.passwordObfuscatorType ?? "Plain",
    passwordOptions: values.passwordOptions ?? [],
    enableSoftDeletion: values.enableSoftDeletion ?? false,
    ipWhitelist: values.ipWhitelist ?? "",
    initiateLoginUrl: values.initiateLoginUrl ?? "",
    balance: values.balance ?? 0,
    currency: values.currency ?? "USD",
    disableSignin: values.disableSignin ?? false,
    enableDeposit: values.enableDeposit ?? false,
    balanceCredit: values.balanceCredit ?? 0,
    mu: values.mu ?? 0,
    defaultQuota: values.defaultQuota ?? 0,
    defaultCurrency: values.defaultCurrency ?? "CNY",
    initScore: values.initScore ?? 0,
    defaultAvatar: values.defaultAvatar ?? "",
    enableDarkLogo: values.enableDarkLogo ?? false,
    hasPrivilegeConsent: values.hasPrivilegeConsent ?? false,
    useEmailAsUsername: values.useEmailAsUsername ?? false,
    usePermanentAvatar: values.usePermanentAvatar ?? false,
    isProfilePublic: values.isProfilePublic ?? false,
    enableTour: values.enableTour ?? false,
    kerberosRealm: values.kerberosRealm ?? "",
    kerberosKdcHost: values.kerberosKdcHost ?? "",
    kerberosServiceName: values.kerberosServiceName ?? "",
    ldapAttributes: values.ldapAttributes ?? [],
    revision,
  };
}

/**
 * 提交方向：settings 字段（`PUT /organizations/{id}/settings`）。
 *
 * `mfaItems` / `accountMenu` / `mfaRememberInHours` 由后端
 * `organizations/request.go` 读取，但 `UpdateOrganizationSettingsPayload`
 * 未声明，故在此显式补全，避免静默丢字段。
 */
export type OrganizationSettingsPayload = UpdateOrganizationSettingsPayload & {
  mfaItems?: Array<{ name: string; rule: string }>;
  accountMenu?: string;
  mfaRememberInHours?: number;
  dcrPolicy?: string;
};

/** `featureFlags.iam` 是独立开关，其余键作为扩展 JSON 原样透传。 */
export function buildOrganizationFeatureFlags(
  values: OrganizationFormValues,
): Record<string, unknown> {
  const featureFlags = parseJsonObject(
    "Organization featureFlags extended",
    values.featureFlagsExtraJson,
  );
  if (values.featureFlag !== undefined) featureFlags.iam = values.featureFlag;
  return featureFlags;
}

export function buildOrganizationSettingsPayload(
  values: OrganizationFormValues,
  featureFlags: Record<string, unknown>,
): OrganizationSettingsPayload {
  return {
    languages: values.languages ?? [],
    defaultApplicationId: values.defaultApplicationId || undefined,
    accountItems: (values.accountItems ?? []).map((item) => ({
      name: item.name?.trim() ?? "",
      visible: item.visible ?? false,
      viewRule: item.viewRule?.trim() ?? "",
      modifyRule: item.modifyRule?.trim() ?? "",
      regex: item.regex?.trim() ?? "",
      tab: item.tab?.trim() ?? "",
    })),
    mfaItems: (values.mfaItems ?? []).map((item) => ({
      name: item.name?.trim() ?? "",
      rule: item.rule?.trim() ?? "",
    })),
    accountMenu: values.accountMenu?.trim() ?? "",
    mfaRememberInHours: values.mfaRememberInHours ?? 12,
    dcrPolicy: values.dcrPolicy ?? "",
    navPreferences: parseJsonObject("Organization navPreferences", values.navPreferencesJson),
    featureFlags,
    countryCodes: values.countryCodes ?? [],
    userTypes: values.userTypes ?? [],
    tags: values.tags ?? [],
  };
}

/** 提交方向：主题字段（`PUT /organizations/{id}/theme`）。 */
export function buildOrganizationThemePayload(
  values: OrganizationFormValues,
): UpdateOrganizationThemePayload {
  return {
    themeName: values.themeName?.trim() || "default",
    tokens: parseJsonObject("Organization theme.tokens", values.themeTokensJson),
    customCss: values.themeCustomCss ?? "",
    assets: parseJsonObject("Organization theme.assets", values.themeAssetsJson),
  };
}

/** 提交方向：安全策略字段（`PUT /organizations/{id}/security-policy`）。 */
export function buildOrganizationSecurityPolicyPayload(
  values: OrganizationFormValues,
): UpdateOrganizationSecurityPolicyPayload {
  const passwordPolicy = parseJsonObject(
    "Organization passwordPolicy extended",
    values.passwordPolicyExtraJson,
  );
  // 扩展 JSON 只承载非标准键：先删除其中手工写入的标准键，再由标准表单
  // 无条件写回 10 个键，避免「表单 + JSON」两处重复配置互相覆盖。
  for (const key of PASSWORD_POLICY_STANDARD_KEYS) delete passwordPolicy[key];
  passwordPolicy.minLength = policyNumber(values.passwordMinLength, 8);
  passwordPolicy.maxLength = policyNumber(values.passwordMaxLength, 128);
  passwordPolicy.requireUppercase = values.passwordRequireUppercase ?? true;
  passwordPolicy.requireLowercase = values.passwordRequireLowercase ?? true;
  passwordPolicy.requireDigit = values.passwordRequireDigit ?? true;
  passwordPolicy.requireSpecial = values.passwordRequireSpecial ?? true;
  passwordPolicy.rejectCommonPasswords = values.passwordRejectCommonPasswords ?? true;
  passwordPolicy.rejectUsername = values.passwordRejectUsername ?? true;
  passwordPolicy.passwordHistoryCount = policyNumber(values.passwordHistoryCount, 5);
  // 后端 PasswordPolicy 的权威字段是 `passwordMaxAgeDays`（见
  // internal/domain/organizations/password_policy.go）；`expireDays` 会被当成
  // 一个未知扩展键存下来，但对密码过期判定完全无效。
  passwordPolicy.passwordMaxAgeDays = policyNumber(values.passwordExpireDays, 0);

  const sessionPolicy = parseJsonObject(
    "Organization sessionPolicy extended",
    values.sessionPolicyExtraJson,
  );
  if (values.sessionIdleTimeoutMinutes !== undefined && values.sessionIdleTimeoutMinutes !== null)
    sessionPolicy.idleTimeoutMinutes = values.sessionIdleTimeoutMinutes;

  const mfaPolicy = parseJsonObject("Organization mfaPolicy extended", values.mfaPolicyExtraJson);
  if (values.mfaRequired !== undefined) mfaPolicy.required = values.mfaRequired;

  return {
    passwordPolicy,
    passwordDeriveAlgorithm: values.passwordDeriveAlgorithm ?? "",
    mfaPolicy,
    ipPolicy: parseJsonObject("Organization ipPolicy", values.ipPolicyJson),
    sessionPolicy,
    riskPolicy: parseJsonObject("Organization riskPolicy", values.riskPolicyJson),
  };
}

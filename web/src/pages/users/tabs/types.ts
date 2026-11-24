import type {
  UserDetail,
  AddressItem,
  ManagedAccountItem,
  ConsentRecordItem,
  ProductInfoItem,
  FaceIdItem,
} from "@/shared/api/iam/user";
import { BALANCE_CURRENCY_CODES, getCurrencyOptions } from "@/shared/lib/currency-utils";
import type { RoleOption } from "@/shared/api/iam/role";
import type { Dayjs } from "dayjs";

// ─── Form value types ──────────────────────────────────────────

/**
 * 用户详情页唯一的账号状态模型。
 *
 * 后端仍分别存储 `status` / `isForbidden` / `isDeleted`，但详情页只暴露这一个
 * 三态控件；派生字段在 payload 映射层由 `accountStatusFields` 计算，避免出现
 * 「status=active 但 isForbidden=true」这类自相矛盾的组合。
 */
export type UserAccountStatus = "active" | "disabled" | "deleted";

export type UserFormValues = {
  owner: string;
  name: string;
  displayName?: string;
  email?: string;
  phone?: string;
  /** 只读验证标记：仅用于展示，不进入 PATCH/PUT 提交 payload。 */
  emailVerified?: boolean;
  phoneVerified?: boolean;
  countryCode?: string;
  region?: string;
  location?: string;
  firstName?: string;
  lastName?: string;
  avatarUrl?: string;
  gender?: string;
  birthday?: string;
  bio?: string;
  homepage?: string;
  education?: string;
  tag?: string;
  userType?: string;
  externalId?: string;
  accountStatus?: UserAccountStatus;
  isAdmin?: boolean;
  score?: number;
  karma?: number;
  ranking?: number;
  balance?: number;
  balanceCredit?: number;
  balanceCurrency?: string;
  ipWhitelist?: string;
  language?: string;
  realName?: string;
  idCardType?: string;
  idCard?: string;
  affiliation?: string;
  title?: string;
  signupApplication?: string;
  needUpdatePassword?: boolean;
  lastChangePasswordTime?: string;
  custom1?: string;
  custom2?: string;
  custom3?: string;
  custom4?: string;
  custom5?: string;
  custom6?: string;
  custom7?: string;
  custom8?: string;
  custom9?: string;
  custom10?: string;
};

/**
 * 自定义属性编辑行。
 *
 * `rawValue` 保留接口原始 JSON 类型（number/boolean/array/object），
 * 仅在用户未修改该行时原样回传，避免把结构化值降级成字符串。
 */
export type CustomPropertyRow = {
  key: string;
  value: string;
  rawValue?: unknown;
};

export type ProfileFormValues = {
  displayName?: string;
  firstName?: string;
  lastName?: string;
  avatarUrl?: string;
  email?: string;
  phone?: string;
  countryCode?: string;
  region?: string;
  location?: string;
  birthday?: string;
  gender?: string;
  bio?: string;
  realName?: string;
  idCardType?: string;
  idCard?: string;
  homepage?: string;
  education?: string;
  affiliation?: string;
  title?: string;
  tag?: string;
  language?: string;
  idCardFront?: string;
  idCardBack?: string;
  idCardWithPerson?: string;
  // 护照/驾照扩展字段（Schema V6 新增）
  passportNo?: string;
  driverLicenseNo?: string;
  idCardExpiry?: string;
  issueCountry?: string;
  issuingAuthority?: string;
  passportPhotoUrl?: string;
  driverLicenseFrontUrl?: string;
  driverLicenseBackUrl?: string;
  // 由 PATCH /users 写入 profile.properties，PUT 全量替换时需要回填。
  karma?: number;
  ranking?: number;
  properties?: CustomPropertyRow[];
};

export type CredentialsFormValues = {
  newPassword?: string;
  confirmPassword?: string;
  rotateAccessSecret?: boolean;
  needUpdatePassword?: boolean;
};

/**
 * 用户详情页唯一表单的值类型。
 *
 * 所有可编辑 Tab 共用同一个 Form 实例，因此值域是主表单、凭据表单与 Profile
 * 表单的并集；各 Tab 只渲染自己的 Form.Item，payload 装配函数按白名单取字段。
 */
export type UserDetailFormValues = UserFormValues & CredentialsFormValues & ProfileFormValues;

export type IdentityFormItem = {
  id?: string;
  providerId: string;
  identityType: string;
  subject: string;
  providerAccount?: string;
  linkedAt?: string | null;
};

/**
 * Add-Identity 弹窗的受控表单值。
 *
 * 复用 `IdentityFormItem`，仅去掉由服务端生成的 `id` / `linkedAt` 字段，
 * 避免为 props/state 手写近似的对象类型。
 */
export type NewIdentityValues = Omit<IdentityFormItem, "id" | "linkedAt">;

// ─── Query data types ──────────────────────────────────────────

export type ProviderItem = {
  id: string;
  name: string;
  displayName: string;
  category: string;
  type: string;
  clientId: string;
};

export type GroupListItem = {
  id: string;
  owner: string;
  name: string;
  displayName: string;
  type: string;
  parentId: string;
  isTopGroup: boolean;
  isEnabled: boolean;
};

// Re-export for convenience
export type {
  UserDetail,
  AddressItem,
  ManagedAccountItem,
  ConsentRecordItem,
  ProductInfoItem,
  FaceIdItem,
  RoleOption,
};

// ─── Option constants (i18n-enabled getters) ──────────────────
// These are functions that accept a translation function `t` and return
// localized option arrays, avoiding hardcoded English labels.

type TFn = (key: string, options?: Record<string, unknown>) => string;

export const getGenderOptions = (t: TFn) => [
  { label: t("resource:user.genderMale"), value: "male" },
  { label: t("resource:user.genderFemale"), value: "female" },
  { label: t("resource:user.genderOther"), value: "other" },
  { label: t("resource:user.genderUnknown"), value: "unknown" },
];

export const getStatusOptions = (t: TFn) => [
  { label: t("resource:user.statusActive"), value: "active" },
  { label: t("resource:user.statusDisabled"), value: "disabled" },
];

/** 详情页唯一账号状态控件的选项；派生字段不对用户暴露。 */
export const getAccountStatusOptions = (t: TFn) => [
  { label: t("resource:user.statusActive"), value: "active" },
  { label: t("resource:user.statusDisabled"), value: "disabled" },
  { label: t("resource:user.statusDeleted"), value: "deleted" },
];

export const getUserTypeOptions = (t: TFn) => [
  { label: t("resource:user.userTypeNormal"), value: "normal" },
  { label: t("resource:user.userTypeManager"), value: "manager" },
  { label: t("resource:user.userTypeService"), value: "service" },
];

export const getIdcardTypeOptions = (t: TFn) => [
  { label: t("resource:user.idCardTypeIdCard"), value: "id_card" },
  { label: t("resource:user.idCardTypePassport"), value: "passport" },
  { label: t("resource:user.idCardTypeDriverLicense"), value: "driver_license" },
  { label: t("resource:user.idCardTypeOther"), value: "other" },
];

export const getBalanceCurrencyOptions = (t: TFn) => getCurrencyOptions(t, BALANCE_CURRENCY_CODES);

export const getMfaTypeOptions = (t: TFn) => [
  { label: t("resource:user.mfaTypeNone"), value: "none" },
  { label: t("resource:user.mfaTypeTotp"), value: "totp" },
  { label: t("resource:user.mfaTypeSms"), value: "sms" },
  { label: t("resource:user.mfaTypeEmail"), value: "email" },
  { label: t("resource:user.mfaTypeWebauthn"), value: "webauthn" },
  { label: t("resource:user.mfaTypeRecovery"), value: "recovery" },
];

export const getLanguageOptions = (t: TFn) => [
  { label: t("resource:user.langEnglish"), value: "en" },
  { label: t("resource:user.langChineseSimplified"), value: "zh-CN" },
  { label: t("resource:user.langChineseTraditional"), value: "zh-TW" },
  { label: t("resource:user.langJapanese"), value: "ja" },
  { label: t("resource:user.langKorean"), value: "ko" },
  { label: t("resource:user.langFrench"), value: "fr" },
  { label: t("resource:user.langGerman"), value: "de" },
  { label: t("resource:user.langSpanish"), value: "es" },
];

// ─── Legacy static constants (for backward compatibility) ─────
// These use English labels as fallback and will be removed once all
// consumers are migrated to the getter functions above.

export const GENDER_OPTIONS = [
  { label: "Male", value: "male" },
  { label: "Female", value: "female" },
  { label: "Other", value: "other" },
  { label: "Unknown", value: "unknown" },
];

export const STATUS_OPTIONS = [
  { label: "Active", value: "active" },
  { label: "Inactive", value: "inactive" },
  { label: "Suspended", value: "suspended" },
];

export const USER_TYPE_OPTIONS = [
  { label: "Normal", value: "normal" },
  { label: "Manager", value: "manager" },
  { label: "Service", value: "service" },
];

export const IDCARD_TYPE_OPTIONS = [
  { label: "ID Card", value: "id_card" },
  { label: "Passport", value: "passport" },
  { label: "Driver License", value: "driver_license" },
  { label: "Other", value: "other" },
];

export const BALANCE_CURRENCY_OPTIONS = BALANCE_CURRENCY_CODES.map((c) => ({ label: c, value: c }));

export const MFA_TYPE_OPTIONS = [
  { label: "None", value: "none" },
  { label: "TOTP", value: "totp" },
  { label: "SMS", value: "sms" },
  { label: "Email", value: "email" },
  { label: "WebAuthn", value: "webauthn" },
  { label: "Recovery", value: "recovery" },
];

export const LANGUAGE_OPTIONS = [
  { label: "English", value: "en" },
  { label: "Chinese (Simplified)", value: "zh-CN" },
  { label: "Chinese (Traditional)", value: "zh-TW" },
  { label: "Japanese", value: "ja" },
  { label: "Korean", value: "ko" },
  { label: "French", value: "fr" },
  { label: "German", value: "de" },
  { label: "Spanish", value: "es" },
];

// ─── Helper functions ──────────────────────────────────────────

export function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

export function getStr(values: Record<string, unknown> | undefined, key: string) {
  const v = values?.[key];
  return typeof v === "string" ? v : "";
}

export function getBool(
  values: Record<string, unknown> | undefined,
  key: string,
  fallback = false,
) {
  const v = values?.[key];
  return typeof v === "boolean" ? v : fallback;
}

export function getNum(values: Record<string, unknown> | undefined, key: string, fallback = 0) {
  const v = values?.[key];
  return typeof v === "number" ? v : fallback;
}

// ─── User create form values ───────────────────────────────────

/** Values collected by the user create form (flat shape, assembled into a payload by `buildUserCreatePayload`). */
export type UserCreateFormValues = {
  organizationId?: string;
  username?: string;
  displayName?: string;
  userType?: string;
  status?: string;
  // Personal profile
  avatarUrl?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
  phone?: string;
  countryCode?: string;
  region?: string;
  location?: string;
  gender?: string;
  birthday?: Dayjs | null;
  language?: string;
  bio?: string;
  homepage?: string;
  education?: string;
  // Security & identity
  password?: string;
  isForbidden?: boolean;
  isDeleted?: boolean;
  needUpdatePassword?: boolean;
  ipWhitelist?: string;
  realName?: string;
  idCardType?: string;
  idCard?: string;
  externalId?: string;
  // Roles & extensions
  isAdmin?: boolean;
  roleBindingIds?: string[];
  groupIds?: string[];
  signupApplicationId?: string;
  tag?: string;
  affiliation?: string;
  title?: string;
  score?: number;
  karma?: number;
  ranking?: number;
  balance?: number;
  balanceCredit?: number;
  balanceCurrency?: string;
};

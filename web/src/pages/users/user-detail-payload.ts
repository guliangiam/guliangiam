/**
 * UserDetailPage 的表单值装配 / 请求体装配纯函数。
 *
 * 从 UserDetailPage.tsx 拆分而来，职责单一、无 JSX：
 *   - API 响应（UserDetail）→ antd Form values
 *   - Form values → 提交 payload
 *
 * 所有字段映射与默认值均与原实现逐字节保持一致，只做搬运不做行为改动。
 */
import type { PatchUserPayload, UserDetail } from "@/shared/api/iam/user";
import { getBool, getNum, getStr } from "./tabs/types";
import type {
  CredentialsFormValues,
  CustomPropertyRow,
  IdentityFormItem,
  ProfileFormValues,
  UserAccountStatus,
  UserFormValues,
} from "./tabs/types";

/** 后端三字段 → 详情页唯一的账号状态（deleted 优先于 disabled）。 */
export function deriveAccountStatus(
  user: Pick<UserDetail, "status" | "isForbidden" | "isDeleted">,
): UserAccountStatus {
  if (user.isDeleted) return "deleted";
  if (user.isForbidden || user.status !== "active") return "disabled";
  return "active";
}

/**
 * 账号状态 → 后端字段组合。
 *
 * 后端仍接收 `status` / `isForbidden` / `isDeleted` 三个字段，字段集合与改造前
 * 一致；这里只保证三者永远来自同一个状态，不再出现互相矛盾的组合。
 */
export function accountStatusFields(state: UserAccountStatus): {
  status: string;
  isForbidden: boolean;
  isDeleted: boolean;
} {
  switch (state) {
    case "deleted":
      return { status: "disabled", isForbidden: true, isDeleted: true };
    case "disabled":
      return { status: "disabled", isForbidden: true, isDeleted: false };
    default:
      return { status: "active", isForbidden: false, isDeleted: false };
  }
}

/** UserDetail → 主表单（Basic / Account / Custom）初始值。 */
export function buildUserFormValues(user: UserDetail): UserFormValues {
  return {
    owner: user.organizationId,
    name: user.username,
    displayName: getStr(user.profile, "displayName"),
    email: getStr(user.profile, "email"),
    phone: getStr(user.profile, "phone"),
    emailVerified: getBool(user.profile, "emailVerified"),
    phoneVerified: getBool(user.profile, "phoneVerified"),
    countryCode: getStr(user.profile, "countryCode"),
    region: getStr(user.profile, "region"),
    location: getStr(user.profile, "location"),
    firstName: getStr(user.profile, "firstName"),
    lastName: getStr(user.profile, "lastName"),
    avatarUrl: getStr(user.profile, "avatarUrl"),
    gender: getStr(user.profile, "gender"),
    birthday: getStr(user.profile, "birthday"),
    bio: getStr(user.profile, "bio"),
    homepage: getStr(user.profile, "homepage") || getStr(user.profile, "websiteUrl"),
    education: getStr(user.profile, "education"),
    tag: user.tag,
    userType: user.userType,
    externalId: user.externalId,
    accountStatus: deriveAccountStatus(user),
    isAdmin: user.isAdmin,
    score: user.score,
    karma: getNum(user, "karma"),
    ranking: getNum(user, "ranking"),
    balance: user.balance,
    balanceCredit: user.balanceCredit,
    balanceCurrency: user.balanceCurrency,
    ipWhitelist: user.ipWhitelist,
    language: getStr(user.profile, "language") || getStr(user, "language"),
    realName: getStr(user.profile, "realName"),
    idCardType: getStr(user.profile, "idCardType"),
    // 规范 9.5：证件号永不回传原文，响应只有 idCardSet/idCardMasked。表单值
    // 恒为空串 = 「保持不变」（PUT 空值保留语义）；输入新值才会替换存储值。
    idCard: "",
    affiliation: getStr(user.profile, "affiliation"),
    title: getStr(user.profile, "title"),
    signupApplication: user.signupApplicationId,
    needUpdatePassword: getBool(user.credentialSummary, "needUpdatePassword"),
    lastChangePasswordTime: getStr(user.credentialSummary, "lastChangePasswordTime"),
    custom1: user.custom1,
    custom2: user.custom2,
    custom3: user.custom3,
    custom4: user.custom4,
    custom5: user.custom5,
    custom6: user.custom6,
    custom7: user.custom7,
    custom8: user.custom8,
    custom9: user.custom9,
    custom10: user.custom10,
  };
}

/** UserDetail → Profile / Real Name 表单初始值。 */
export function buildProfileFormValues(user: UserDetail): ProfileFormValues {
  return {
    displayName: getStr(user.profile, "displayName"),
    firstName: getStr(user.profile, "firstName"),
    lastName: getStr(user.profile, "lastName"),
    avatarUrl: getStr(user.profile, "avatarUrl"),
    email: getStr(user.profile, "email"),
    phone: getStr(user.profile, "phone"),
    countryCode: getStr(user.profile, "countryCode"),
    region: getStr(user.profile, "region"),
    location: getStr(user.profile, "location"),
    birthday: getStr(user.profile, "birthday"),
    gender: getStr(user.profile, "gender"),
    bio: getStr(user.profile, "bio"),
    realName: getStr(user.profile, "realName"),
    idCardType: getStr(user.profile, "idCardType"),
    // 规范 9.5：证件号永不回传原文，空串 = 「保持不变」（见下方 passportNo 注）。
    idCard: "",
    homepage: getStr(user.profile, "homepage") || getStr(user.profile, "websiteUrl"),
    education: getStr(user.profile, "education"),
    language: getStr(user.profile, "language") || getStr(user, "language"),
    idCardFront: getStr(user.profile, "idCardFront"),
    idCardBack: getStr(user.profile, "idCardBack"),
    idCardWithPerson: getStr(user.profile, "idCardWithPerson"),
    // 规范 9.5：证件号永不回传原文（响应仅 Set/Masked 对）。表单值恒为空串 =
    // 「保持不变」；PUT 提交空值时服务端保留存储值，输入新值才会替换。
    passportNo: "",
    driverLicenseNo: "",
    idCardExpiry: getStr(user.profile, "idCardExpiry"),
    issueCountry: getStr(user.profile, "issueCountry"),
    issuingAuthority: getStr(user.profile, "issuingAuthority"),
    passportPhotoUrl: getStr(user.profile, "passportPhotoUrl"),
    driverLicenseFrontUrl: getStr(user.profile, "driverLicenseFrontUrl"),
    driverLicenseBackUrl: getStr(user.profile, "driverLicenseBackUrl"),
    tag: user.tag,
  };
}

/**
 * PATCH /users/{id} 的可选字符串字段在 DTO 中是 `*string`：省略键表示“不改”，
 * 传空串才表示“清空”。antd 清空输入后表单值可能是 `undefined`，直接写回对象
 * 会被 `JSON.stringify` 丢掉整个键，导致清空意图根本没到后端。这里只在字段
 * 确实变化时才把 `undefined` 归一化为空串，未修改的字段仍然被省略。
 */
function normalizeOptionalText(value: string | undefined): string {
  return value ?? "";
}

function optionalTextChanged(next: string | undefined, previous: string | undefined): boolean {
  return normalizeOptionalText(next) !== normalizeOptionalText(previous);
}

/** 主表单 values → PATCH /users/{id} 请求体（仅提交发生变化的字段）。 */
export function buildUserUpdatePayload(
  values: UserFormValues,
  user: UserDetail | undefined,
): PatchUserPayload {
  // 只比较「状态模型」的派生结果：未触碰状态控件时，历史用户即使存在
  // status=suspended 或 isForbidden=true 也不会被其他字段的保存静默改写。
  const previousAccount = accountStatusFields(user ? deriveAccountStatus(user) : "active");
  const nextAccount = accountStatusFields(
    values.accountStatus ?? (user ? deriveAccountStatus(user) : "active"),
  );
  return {
    ...(optionalTextChanged(values.externalId, user?.externalId) && {
      externalId: normalizeOptionalText(values.externalId),
    }),
    ...(optionalTextChanged(values.signupApplication, user?.signupApplicationId) && {
      signupApplicationId: normalizeOptionalText(values.signupApplication),
    }),
    ...(values.userType !== user?.userType && { userType: values.userType }),
    ...(nextAccount.status !== previousAccount.status && { status: nextAccount.status }),
    ...(values.isAdmin !== user?.isAdmin && { isAdmin: values.isAdmin }),
    ...(nextAccount.isForbidden !== previousAccount.isForbidden && {
      isForbidden: nextAccount.isForbidden,
    }),
    ...(nextAccount.isDeleted !== previousAccount.isDeleted && {
      isDeleted: nextAccount.isDeleted,
    }),
    ...(optionalTextChanged(values.tag, user?.tag) && { tag: normalizeOptionalText(values.tag) }),
    ...(values.score !== user?.score && { score: values.score }),
    ...(values.karma !== undefined &&
      values.karma !== getNum(user, "karma") && { karma: values.karma }),
    ...(values.ranking !== undefined &&
      values.ranking !== getNum(user, "ranking") && { ranking: values.ranking }),
    ...(values.balance !== user?.balance && { balance: values.balance }),
    ...(values.balanceCredit !== user?.balanceCredit && {
      balanceCredit: values.balanceCredit,
    }),
    ...(optionalTextChanged(values.balanceCurrency, user?.balanceCurrency) && {
      balanceCurrency: normalizeOptionalText(values.balanceCurrency),
    }),
    ...(optionalTextChanged(values.ipWhitelist, user?.ipWhitelist) && {
      ipWhitelist: normalizeOptionalText(values.ipWhitelist),
    }),
    ...(values.language !== undefined &&
      values.language !== getStr(user?.profile, "language") && { language: values.language }),
    ...(optionalTextChanged(values.custom1, user?.custom1) && {
      custom1: normalizeOptionalText(values.custom1),
    }),
    ...(optionalTextChanged(values.custom2, user?.custom2) && {
      custom2: normalizeOptionalText(values.custom2),
    }),
    ...(optionalTextChanged(values.custom3, user?.custom3) && {
      custom3: normalizeOptionalText(values.custom3),
    }),
    ...(optionalTextChanged(values.custom4, user?.custom4) && {
      custom4: normalizeOptionalText(values.custom4),
    }),
    ...(optionalTextChanged(values.custom5, user?.custom5) && {
      custom5: normalizeOptionalText(values.custom5),
    }),
    ...(optionalTextChanged(values.custom6, user?.custom6) && {
      custom6: normalizeOptionalText(values.custom6),
    }),
    ...(optionalTextChanged(values.custom7, user?.custom7) && {
      custom7: normalizeOptionalText(values.custom7),
    }),
    ...(optionalTextChanged(values.custom8, user?.custom8) && {
      custom8: normalizeOptionalText(values.custom8),
    }),
    ...(optionalTextChanged(values.custom9, user?.custom9) && {
      custom9: normalizeOptionalText(values.custom9),
    }),
    ...(optionalTextChanged(values.custom10, user?.custom10) && {
      custom10: normalizeOptionalText(values.custom10),
    }),
    revision: user?.revision ?? 0,
  };
}

/**
 * Profile 表单白名单（= PUT /users/{id}/profile 接受的全部字段）。
 * `tag` 属于 users 列，由 PATCH /users 保存；`language` 虽也由 PATCH 写入，
 * 但 PUT 是全量替换 profile.properties，必须一并带上才不会清空。
 */
const PROFILE_FIELD_KEYS = [
  "displayName",
  "firstName",
  "lastName",
  "avatarUrl",
  "email",
  "phone",
  "countryCode",
  "region",
  "location",
  "birthday",
  "gender",
  "bio",
  "realName",
  "idCardType",
  "idCard",
  "affiliation",
  "homepage",
  "education",
  "title",
  "idCardFront",
  "idCardBack",
  "idCardWithPerson",
  "passportNo",
  "driverLicenseNo",
  "idCardExpiry",
  "issueCountry",
  "issuingAuthority",
  "passportPhotoUrl",
  "driverLicenseFrontUrl",
  "driverLicenseBackUrl",
  "language",
] as const satisfies ReadonlyArray<keyof ProfileFormValues>;

/**
 * 由后端折叠进 profile.properties 的系统字段：不参与「自定义属性」表格展示，
 * 否则既会在 Properties 页出现重复行，又会被当作普通属性覆盖。
 *
 * karma/ranking 由 PATCH /users 以数字写入，表格只支持字符串编辑，
 * 展示出来既改不动也会把数字降级成字符串，因此同样视为保留键。
 */
const SYSTEM_PROFILE_PROPERTY_KEYS = new Set([
  "affiliation",
  "homepage",
  "education",
  "title",
  "language",
  "idCardFront",
  "idCardBack",
  "idCardWithPerson",
  "idCard",
  "passportNo",
  "driverLicenseNo",
  "idCardExpiry",
  "issueCountry",
  "issuingAuthority",
  "passportPhotoUrl",
  "driverLicenseFrontUrl",
  "driverLicenseBackUrl",
  "karma",
  "ranking",
]);

/** 自定义属性保留键：UI 不允许新建/改名到这些键，避免与系统字段互相覆盖。 */
export const RESERVED_PROFILE_PROPERTY_KEYS = SYSTEM_PROFILE_PROPERTY_KEYS;

export const MAX_CUSTOM_PROPERTIES = 100;
export const MAX_CUSTOM_PROPERTY_KEY_LENGTH = 64;

/** 属性值 → 表格可编辑文本；结构化值用 JSON 展示，便于识别而不是得到 [object Object]。 */
export function formatPropertyValue(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  try {
    return JSON.stringify(value) ?? "";
  } catch {
    return String(value);
  }
}

/**
 * 自定义属性行校验（前端防御，权威校验在后端）。
 *
 * 返回 i18n 后缀（如 `keyRequired`），无错误返回 null。
 */
export function validateCustomProperties(rows: CustomPropertyRow[]): string | null {
  if (rows.length > MAX_CUSTOM_PROPERTIES) return "tooMany";
  const seen = new Set<string>();
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) return "keyRequired";
    if (key.length > MAX_CUSTOM_PROPERTY_KEY_LENGTH) return "keyTooLong";
    if (RESERVED_PROFILE_PROPERTY_KEYS.has(key)) return "keyReserved";
    if (seen.has(key)) return "keyDuplicate";
    seen.add(key);
  }
  return null;
}

/**
 * DatePicker 编辑后会把 form store 里的 birthday 变成 dayjs 对象；后端 profile
 * 接收的是 `YYYY-MM-DD` 字符串。不在这里归一化，JSON 序列化会把对象写成
 * ISO 时间串（`2026-11-08T00:00:00.000Z`），导致回显被截断/解析失败。
 */
function normalizeProfileValue(value: unknown): unknown {
  if (
    value &&
    typeof value === "object" &&
    typeof (value as { format?: unknown }).format === "function"
  ) {
    return (value as { format: (pattern: string) => string }).format("YYYY-MM-DD");
  }
  return value;
}

/** 自定义属性行 → profile.properties（不含 karma/ranking 回填）。 */
function buildCustomPropertiesPayload(
  customProperties: CustomPropertyRow[],
): Record<string, unknown> {
  const properties: Record<string, unknown> = {};
  for (const row of customProperties) {
    const key = row.key.trim();
    // 空键 / 空值都不写入：PUT 是全量替换，缺省即删除，满足「清空 = 删除」。
    if (!key || row.value === "") continue;
    // 未修改的行原样回传原始 JSON 类型（数字/布尔/数组/对象）。
    properties[key] =
      row.rawValue !== undefined && row.value === formatPropertyValue(row.rawValue)
        ? row.rawValue
        : row.value;
  }
  return properties;
}

/** Profile 表单 values + 自定义属性 → PUT /users/{id}/profile 请求体。 */
export function buildUserSettingsPayload(
  values: ProfileFormValues,
  customProperties: CustomPropertyRow[],
): Record<string, unknown> {
  const payload: Record<string, unknown> = {};
  for (const key of PROFILE_FIELD_KEYS) {
    if (values[key] !== undefined) payload[key] = normalizeProfileValue(values[key]);
  }
  const properties = buildCustomPropertiesPayload(customProperties);
  // karma/ranking 由 PATCH 写入 profile.properties，而 PUT 是全量替换：
  // 不在这里按当前表单值回填，刚保存的数值会被覆盖回旧值。
  if (values.karma !== undefined) properties.karma = values.karma;
  if (values.ranking !== undefined) properties.ranking = values.ranking;
  return { ...payload, properties };
}

/**
 * 当前用户全量 profile → PUT 保留基线。账号页分段表单（基本资料/实名）共用
 * 全量替换端点，提交前必须先合并本基线，否则保存一段会清空另一段的字段。
 */
export function buildProfileRetentionPayload(user: UserDetail): Record<string, unknown> {
  const payload = buildUserSettingsPayload(
    buildProfileFormValues(user),
    buildCustomProperties(user.profile),
  );
  // affiliation/title 在 PUT 白名单里但不在表单映射中，直接从 profile 回填。
  for (const key of ["affiliation", "title"] as const) {
    const value = user.profile?.[key];
    if (typeof value === "string") payload[key] = value;
  }
  // karma/ranking 是系统保留属性，自定义属性表格不含它们，须原样保留。
  const properties = payload.properties as Record<string, unknown>;
  const current = user.profile?.properties as Record<string, unknown> | undefined;
  for (const key of ["karma", "ranking"] as const) {
    if (current?.[key] !== undefined) properties[key] = current[key];
  }
  return payload;
}

/**
 * PATCH /users 是否真的带了改动。
 *
 * 装配函数总是带 `revision`（乐观锁），只有它说明「没有字段变化」。
 */
export function hasUserUpdateChanges(payload: PatchUserPayload): boolean {
  return Object.keys(payload).some((key) => key !== "revision");
}

/** 属性对象按 key 排序后序列化，避免行顺序变化造成「未修改也被判脏」。 */
function canonicalProperties(properties: Record<string, unknown>): string {
  return JSON.stringify(
    Object.keys(properties)
      .sort()
      .map((key) => [key, properties[key]]),
  );
}

/**
 * PUT /profile 是否有改动：逐字段比较整份表单 store 与用户数据。
 *
 * 页面共用一个 Form 实例且 Tab 卸载后 antd 默认保留值，因此这里能拿到所有
 * 已访问 Tab 的当前值；保存时据此决定是否需要补一次 PUT，而不是只看当前 Tab。
 */
export function hasUserSettingsChanges(
  values: ProfileFormValues,
  customProperties: CustomPropertyRow[],
  user: UserDetail | undefined,
): boolean {
  if (!user) return false;
  const baseline = { ...buildUserFormValues(user), ...buildProfileFormValues(user) };
  for (const key of PROFILE_FIELD_KEYS) {
    if (normalizeProfileValue(values[key]) !== normalizeProfileValue(baseline[key])) return true;
  }
  return (
    canonicalProperties(buildCustomPropertiesPayload(customProperties)) !==
    canonicalProperties(buildCustomPropertiesPayload(buildCustomProperties(user.profile)))
  );
}

/**
 * PUT /credentials 是否有改动。
 *
 * Selector 类开关只有「打开」才有动作；needUpdatePassword 是布尔值，需要与
 * 用户当前的凭据摘要比较。跨 Tab 的这两个开关同样要进入保存计划。
 */
export function hasCredentialChanges(
  values: CredentialsFormValues,
  user: UserDetail | undefined,
): boolean {
  if (values.rotateAccessSecret === true) return true;
  return (
    (values.needUpdatePassword ?? false) !== getBool(user?.credentialSummary, "needUpdatePassword")
  );
}

/** Credentials 表单 values → PUT /users/{id}/credentials 请求体。
 * 口令不在该端点：改口令统一走 POST /users/{id}/set-password（users:password:reset）。 */
export function buildCredentialsPayload(values: CredentialsFormValues): CredentialsFormValues {
  return {
    rotateAccessSecret: values.rotateAccessSecret ?? false,
    needUpdatePassword: values.needUpdatePassword ?? false,
  };
}

/** user.profile.properties → 自定义属性编辑行（排除后端折叠的系统字段）。 */
export function buildCustomProperties(
  profile: Record<string, unknown> | undefined,
): CustomPropertyRow[] {
  const properties = profile?.properties;
  return properties && typeof properties === "object"
    ? Object.entries(properties)
        .filter(([key]) => !SYSTEM_PROFILE_PROPERTY_KEYS.has(key))
        .map(([key, value]) => ({
          key,
          value: formatPropertyValue(value),
          rawValue: value,
        }))
    : [];
}

/** UserDetail.identities → 可编辑 identity 状态。 */
export function buildIdentities(user: UserDetail): IdentityFormItem[] {
  return (user.identities ?? []).map((id) => ({
    id: id.id,
    providerId: id.providerId ?? "",
    identityType: id.identityType ?? "",
    subject: id.subject ?? "",
    providerAccount: id.providerAccount ?? "",
    linkedAt: id.linkedAt ?? null,
  }));
}

import { z } from "zod";
import { HttpError, requestDocumentJson } from "@/shared/api/http";
import { captureError } from "@/shared/lib/sentry";
import { pageMetaSchema, flexibleObjectSchema } from "./schemas";
import type { OrganizationIdentity } from "./schemas";
import {
  ORGANIZATIONS,
  ORGANIZATIONS_NAMES,
  ORGANIZATION_BY_ID,
  ORGANIZATION_BY_SLUG,
  ORGANIZATION_SECURITY_POLICY,
  ORGANIZATION_SETTINGS,
  ORGANIZATION_THEME,
  ORGANIZATION_APPLICATIONS,
  ORGANIZATION_DEFAULT_APPLICATION,
  ORGANIZATION_DEPLOYMENT_REGIONS,
  ORGANIZATION_CONFIGURATION,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type AccountItem = {
  name: string;
  visible: boolean;
  viewRule: string;
  modifyRule: string;
  regex: string;
  tab: string;
};

export type OrganizationOption = {
  id: string;
  parentId: string;
  name: string;
  slug: string;
  displayName: string;
  label: string;
  value: string;
};

export type OrganizationDetail = {
  id: string;
  parentId: string;
  regionCode: string;
  slug: string;
  displayName: string;
  websiteUrl: string;
  logoUrl: string;
  logoDarkUrl: string;
  faviconUrl: string;
  status: string;
  passwordType: string;
  /**
   * Credential columns are write-only: the read API returns an is-set flag plus
   * a masked display value, never the stored secret (same shape as certs /
   * providers). Create/Patch payloads still accept the plaintext.
   */
  passwordSaltSet: boolean;
  passwordSaltMasked: string;
  passwordObfuscatorType: string;
  passwordObfuscatorKeySet: boolean;
  passwordObfuscatorKeyMasked: string;
  passwordOptions: string[];
  defaultPasswordSet: boolean;
  defaultPasswordMasked: string;
  enableSoftDeletion: boolean;
  ipWhitelist: string;
  initiateLoginUrl: string;
  balance: number;
  currency: string;
  disableSignin: boolean;
  enableDeposit: boolean;
  balanceCredit: number;
  mu: number;
  defaultQuota: number;
  defaultCurrency: string;
  initScore: number;
  defaultAvatar: string;
  enableDarkLogo: boolean;
  hasPrivilegeConsent: boolean;
  ldapAttributes: string[];
  settings: Record<string, unknown>;
  theme: Record<string, unknown>;
  securityPolicySummary: Record<string, unknown>;
  masterPasswordSet: boolean;
  masterPasswordMasked: string;
  masterVerificationCodeSet: boolean;
  masterVerificationCodeMasked: string;
  isProfilePublic: boolean;
  useEmailAsUsername: boolean;
  usePermanentAvatar: boolean;
  enableTour: boolean;
  mfaRememberInHours: number;
  kerberosRealm: string;
  kerberosKdcHost: string;
  kerberosKeytabSet: boolean;
  kerberosKeytabMasked: string;
  kerberosServiceName: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type CreateOrganizationPayload = {
  parentId?: string;
  regionCode?: string;
  slug: string;
  displayName: string;
  websiteUrl?: string;
  logoUrl?: string;
  logoDarkUrl?: string;
  faviconUrl?: string;
  defaultApplicationId?: string;
  defaultAvatar?: string;
  enableDarkLogo?: boolean;
  initScore?: number;
  status: string;
  languages?: string[];

  // password policy
  passwordType?: string;
  passwordSalt?: string;
  passwordObfuscatorType?: string;
  passwordObfuscatorKey?: string;
  passwordOptions?: string[];
  defaultPassword?: string;
  masterPassword?: string;
  masterVerificationCode?: string;

  // security & signin
  disableSignin?: boolean;
  enableTour?: boolean;
  hasPrivilegeConsent?: boolean;
  isProfilePublic?: boolean;
  useEmailAsUsername?: boolean;
};

export type CreateOrganizationResult = {
  id: string;
  status: string;
};

export type UpdateOrganizationPayload = {
  parentId?: string;
  regionCode?: string;
  displayName?: string;
  websiteUrl?: string;
  logoUrl?: string;
  logoDarkUrl?: string;
  faviconUrl?: string;
  status?: string;
  passwordType?: string;
  passwordSalt?: string;
  passwordObfuscatorType?: string;
  passwordObfuscatorKey?: string;
  passwordOptions?: string[];
  defaultPassword?: string;
  enableSoftDeletion?: boolean;
  ipWhitelist?: string;
  initiateLoginUrl?: string;
  balance?: number;
  currency?: string;
  disableSignin?: boolean;
  enableDeposit?: boolean;
  balanceCredit?: number;
  mu?: number;
  defaultQuota?: number;
  defaultCurrency?: string;
  initScore?: number;
  defaultAvatar?: string;
  enableDarkLogo?: boolean;
  hasPrivilegeConsent?: boolean;
  useEmailAsUsername?: boolean;
  usePermanentAvatar?: boolean;
  masterPassword?: string;
  masterVerificationCode?: string;
  isProfilePublic?: boolean;
  enableTour?: boolean;
  kerberosRealm?: string;
  kerberosKdcHost?: string;
  kerberosKeytab?: string;
  kerberosServiceName?: string;
  ldapAttributes?: string[];
  revision: number;
};

export type UpdateOrganizationSettingsPayload = {
  languages?: string[];
  defaultApplicationId?: string;
  accountItems?: AccountItem[];
  navPreferences?: Record<string, unknown>;
  featureFlags?: Record<string, unknown>;
  countryCodes?: string[];
  userTypes?: string[];
  tags?: string[];
};

export type UpdateOrganizationThemePayload = {
  themeName?: string;
  tokens?: Record<string, unknown>;
  customCss?: string;
  assets?: Record<string, unknown>;
};

export type UpdateOrganizationSecurityPolicyPayload = {
  passwordPolicy?: Record<string, unknown>;
  passwordDeriveAlgorithm?: string;
  mfaPolicy?: Record<string, unknown>;
  ipPolicy?: Record<string, unknown>;
  sessionPolicy?: Record<string, unknown>;
  riskPolicy?: Record<string, unknown>;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const organizationListItemSchema = z.object({
  id: z.string(),
  parentId: z.string().default(""),
  regionCode: z.string().default(""),
  slug: z.string(),
  displayName: z.string(),
  websiteUrl: z.string(),
  logoUrl: z.string(),
  defaultApplicationId: z.string(),
  status: z.string(),
  passwordType: z.string().default("argon2id"),
  balance: z.number().default(0),
  currency: z.string().default("USD"),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const organizationListResponseSchema = z.object({
  items: z.array(organizationListItemSchema),
  page: pageMetaSchema,
});

const organizationDetailSchema = organizationListItemSchema.extend({
  logoDarkUrl: z.string().default(""),
  faviconUrl: z.string().default(""),
  passwordType: z.string().default("argon2id"),
  passwordSaltSet: z.boolean().default(false),
  passwordSaltMasked: z.string().default(""),
  passwordObfuscatorType: z.string().default("Plain"),
  passwordObfuscatorKeySet: z.boolean().default(false),
  passwordObfuscatorKeyMasked: z.string().default(""),
  passwordOptions: z.array(z.string()).catch([]),
  defaultPasswordSet: z.boolean().default(false),
  defaultPasswordMasked: z.string().default(""),
  enableSoftDeletion: z.boolean().default(false),
  ipWhitelist: z.string().default(""),
  initiateLoginUrl: z.string().default(""),
  balance: z.number().default(0),
  currency: z.string().default("USD"),
  disableSignin: z.boolean().default(false),
  enableDeposit: z.boolean().default(false),
  balanceCredit: z.number().default(0),
  mu: z.number().default(0),
  defaultQuota: z.number().default(0),
  // 设计口径：defaultCurrency 缺省为 CNY（currency 缺省仍为 USD）。
  defaultCurrency: z.string().default("CNY"),
  initScore: z.number().default(0),
  defaultAvatar: z.string().default(""),
  enableDarkLogo: z.boolean().default(false),
  hasPrivilegeConsent: z.boolean().default(false),
  ldapAttributes: z.array(z.string()).catch([]),
  settings: flexibleObjectSchema,
  theme: flexibleObjectSchema,
  securityPolicySummary: flexibleObjectSchema,
  masterPasswordSet: z.boolean().default(false),
  masterPasswordMasked: z.string().default(""),
  masterVerificationCodeSet: z.boolean().default(false),
  masterVerificationCodeMasked: z.string().default(""),
  isProfilePublic: z.boolean().default(false),
  useEmailAsUsername: z.boolean().default(false),
  usePermanentAvatar: z.boolean().default(false),
  enableTour: z.boolean().default(false),
  mfaRememberInHours: z.number().default(0),
  kerberosRealm: z.string().default(""),
  kerberosKdcHost: z.string().default(""),
  kerberosKeytabSet: z.boolean().default(false),
  kerberosKeytabMasked: z.string().default(""),
  kerberosServiceName: z.string().default(""),
  passwordObfuscator: z.string().default(""),
  revision: z.number(),
});

const createOrganizationResultSchema = z.object({
  id: z.string(),
  status: z.string(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

export type OrganizationItem = {
  id: string;
  name: string;
  slug: string;
  displayName: string;
  websiteUrl: string;
  defaultApplicationId: string;
  defaultApplication: string;
  logoUrl: string;
  status: string;
  passwordType: string;
  balance: number;
  currency: string;
  createdAt: string;
  updatedAt: string;
};

function toOrganizationItem(item: z.infer<typeof organizationListItemSchema>): OrganizationItem {
  return {
    ...item,
    name: item.slug,
    defaultApplication: item.defaultApplicationId,
  };
}

function toOrganizationOption(
  item: z.infer<typeof organizationListItemSchema>,
): OrganizationOption {
  return {
    id: item.id,
    parentId: item.parentId,
    name: item.slug,
    slug: item.slug,
    displayName: item.displayName,
    label: item.displayName,
    value: item.id,
  };
}

function toOrganizationDetail(item: z.infer<typeof organizationDetailSchema>): OrganizationDetail {
  return {
    id: item.id,
    parentId: item.parentId ?? "",
    regionCode: item.regionCode ?? "",
    slug: item.slug,
    displayName: item.displayName,
    websiteUrl: item.websiteUrl,
    logoUrl: item.logoUrl,
    logoDarkUrl: item.logoDarkUrl,
    faviconUrl: item.faviconUrl,
    status: item.status,
    passwordType: item.passwordType,
    passwordSaltSet: item.passwordSaltSet,
    passwordSaltMasked: item.passwordSaltMasked,
    passwordObfuscatorType: item.passwordObfuscatorType,
    passwordObfuscatorKeySet: item.passwordObfuscatorKeySet,
    passwordObfuscatorKeyMasked: item.passwordObfuscatorKeyMasked,
    passwordOptions: item.passwordOptions,
    defaultPasswordSet: item.defaultPasswordSet,
    defaultPasswordMasked: item.defaultPasswordMasked,
    enableSoftDeletion: item.enableSoftDeletion,
    ipWhitelist: item.ipWhitelist,
    initiateLoginUrl: item.initiateLoginUrl,
    balance: item.balance,
    currency: item.currency,
    disableSignin: item.disableSignin,
    enableDeposit: item.enableDeposit,
    balanceCredit: item.balanceCredit,
    mu: item.mu,
    defaultQuota: item.defaultQuota,
    defaultCurrency: item.defaultCurrency,
    initScore: item.initScore,
    defaultAvatar: item.defaultAvatar,
    enableDarkLogo: item.enableDarkLogo,
    hasPrivilegeConsent: item.hasPrivilegeConsent,
    ldapAttributes: item.ldapAttributes,
    settings: item.settings,
    theme: item.theme,
    securityPolicySummary: item.securityPolicySummary,
    masterPasswordSet: item.masterPasswordSet,
    masterPasswordMasked: item.masterPasswordMasked,
    masterVerificationCodeSet: item.masterVerificationCodeSet,
    masterVerificationCodeMasked: item.masterVerificationCodeMasked,
    isProfilePublic: item.isProfilePublic,
    useEmailAsUsername: item.useEmailAsUsername,
    usePermanentAvatar: item.usePermanentAvatar,
    enableTour: item.enableTour,
    mfaRememberInHours: item.mfaRememberInHours ?? 0,
    kerberosRealm: item.kerberosRealm,
    kerberosKdcHost: item.kerberosKdcHost,
    kerberosKeytabSet: item.kerberosKeytabSet,
    kerberosKeytabMasked: item.kerberosKeytabMasked,
    kerberosServiceName: item.kerberosServiceName,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export type DeploymentRegionOption = {
  id: string;
  regionCode: string;
  name: string;
  countryCode: string;
  timezone: string;
  isPrimary: boolean;
};

const deploymentRegionListSchema = z.object({
  items: z.array(
    z.object({
      id: z.string(),
      regionCode: z.string(),
      name: z.string().default(""),
      countryCode: z.string().default(""),
      timezone: z.string().default(""),
      isPrimary: z.boolean().default(false),
    }),
  ),
});

export async function getDeploymentRegions(): Promise<DeploymentRegionOption[]> {
  const response = await requestDocumentJson(ORGANIZATION_DEPLOYMENT_REGIONS);
  return deploymentRegionListSchema.parse(response).items;
}

export async function getOrganizations() {
  const response = await requestDocumentJson(`${ORGANIZATIONS}?page=1&pageSize=50`);
  return organizationListResponseSchema.parse(response).items.map(toOrganizationItem);
}

export type OrganizationListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  status?: string;
};

export async function getOrganizationsPaginated(params?: OrganizationListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.status) query.set("status", params.status);

  const response = await requestDocumentJson(`${ORGANIZATIONS}?${query.toString()}`);
  const parsed = organizationListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toOrganizationItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getOrganizationOptions() {
  const response = await requestDocumentJson(`${ORGANIZATIONS}?page=1&pageSize=50`);
  return organizationListResponseSchema.parse(response).items.map(toOrganizationOption);
}

const organizationIdentitySchema = z.object({
  id: z.string(),
  slug: z.string(),
});

// organization id 是不透明 UUID（含 UUIDv7）；slug 是既有的业务标识。
const ORGANIZATION_UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export async function getOrganizationIdentityBySlug(
  identifier: string,
): Promise<OrganizationIdentity | null> {
  // 解析链路同时接受 organization id 与 slug：id 走管理详情端点，租户边界由
  // 后端 TrustedOrganizationID + 既有 scope 裁决（平台管理员 all-scope 显式
  // 放行跨租户，普通用户跨租户仍是 404）；前端不新增信任 query/body 的旁路。
  // UUID 形态优先按 id 解析，命中不到再回退 slug，避免把 UUID 当 slug 查询。
  const candidates = ORGANIZATION_UUID_PATTERN.test(identifier)
    ? [ORGANIZATION_BY_ID(identifier), ORGANIZATION_BY_SLUG(identifier)]
    : [ORGANIZATION_BY_SLUG(identifier)];

  let lastError: unknown;
  for (const path of candidates) {
    try {
      const response = await requestDocumentJson(path);
      const matched = organizationIdentitySchema.parse(response);
      return { id: matched.id, slug: matched.slug };
    } catch (error) {
      // 只有 404 才表示「组织不存在」；403/5xx/网络错误必须向上抛，让调用方
      // 展示错误态与重试入口，而不是把授权/服务故障误渲染成 not-found。
      if (error instanceof HttpError && error.status === 404) {
        lastError = error;
        continue;
      }
      captureError(error, { identifier });
      throw error;
    }
  }

  captureError(lastError, { identifier });
  return null;
}

export async function getOrganization(id: string): Promise<OrganizationDetail> {
  // 管理端详情 GET 在响应头带 `private, max-age=30` + ETag。写入后用 TanStack
  // Query 失效重取时，浏览器会直接回放 30s 内的旧响应（revision 也是旧值），
  // 表现为保存后回显旧值、连续两次保存触发 409 revision mismatch。
  // 编辑器必须读到刚写入的结果，故显式绕过 HTTP 缓存。
  const response = await requestDocumentJson(ORGANIZATION_BY_ID(id), { cache: "no-store" });
  return toOrganizationDetail(organizationDetailSchema.parse(response));
}

export async function updateOrganization(id: string, payload: UpdateOrganizationPayload) {
  await requestDocumentJson(ORGANIZATION_BY_ID(id), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export type SaveOrganizationConfigurationPayload = {
  organization: UpdateOrganizationPayload;
  settings: UpdateOrganizationSettingsPayload & {
    mfaItems?: Array<{ name: string; rule: string }>;
    accountMenu?: string;
    mfaRememberInHours?: number;
    dcrPolicy?: string;
  };
  theme: UpdateOrganizationThemePayload;
  securityPolicy: UpdateOrganizationSecurityPolicyPayload;
};

/** Atomically saves the four parts of the organization detail page. */
export async function saveOrganizationConfiguration(
  id: string,
  payload: SaveOrganizationConfigurationPayload,
) {
  await requestDocumentJson(ORGANIZATION_CONFIGURATION(id), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updateOrganizationSettings(
  id: string,
  payload: UpdateOrganizationSettingsPayload,
) {
  await requestDocumentJson(ORGANIZATION_SETTINGS(id), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function updateOrganizationTheme(id: string, payload: UpdateOrganizationThemePayload) {
  await requestDocumentJson(ORGANIZATION_THEME(id), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function updateOrganizationSecurityPolicy(
  id: string,
  payload: UpdateOrganizationSecurityPolicyPayload,
) {
  await requestDocumentJson(ORGANIZATION_SECURITY_POLICY(id), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function createOrganization(
  payload: CreateOrganizationPayload,
): Promise<CreateOrganizationResult> {
  const response = await requestDocumentJson(ORGANIZATIONS, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
  return createOrganizationResultSchema.parse(response);
}

export async function deleteOrganization(id: string) {
  await requestDocumentJson(ORGANIZATION_BY_ID(id), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// Default Application & Organization Names
// ─────────────────────────────────────────────

export type DefaultApplication = {
  id: string;
  organizationId: string;
  slug: string;
  name: string;
  displayName: string;
  category: string;
  appType: string;
  domain: string;
  status: string;
};

export async function getDefaultApplication(orgId: string): Promise<DefaultApplication> {
  const response = await requestDocumentJson(ORGANIZATION_DEFAULT_APPLICATION(orgId));
  return response as DefaultApplication;
}

export type OrganizationNameItem = {
  id: string;
  displayName: string;
  slug: string;
};

export async function getOrganizationNames(): Promise<OrganizationNameItem[]> {
  const response = await requestDocumentJson(ORGANIZATIONS_NAMES);
  return response as OrganizationNameItem[];
}

// ─────────────────────────────────────────────
// Sub-resource GET: Settings, Theme, Security Policy
// ─────────────────────────────────────────────

export async function getOrganizationSettings(
  id: string,
): Promise<UpdateOrganizationSettingsPayload> {
  const response = await requestDocumentJson(ORGANIZATION_SETTINGS(id));
  return response as UpdateOrganizationSettingsPayload;
}

export async function getOrganizationTheme(id: string): Promise<UpdateOrganizationThemePayload> {
  const response = await requestDocumentJson(ORGANIZATION_THEME(id));
  return response as UpdateOrganizationThemePayload;
}

const organizationPublicThemeSchema = z.object({
  theme: z.object({
    themeName: z.string().optional(),
    tokens: flexibleObjectSchema.optional(),
    customCss: z.string().optional(),
    assets: flexibleObjectSchema.optional(),
  }),
});

/**
 * 匿名主题发现：by-slug 公开投影携带 theme（品牌/token/自定义 CSS，无凭据列）。
 * 登录/注册等登录前页面没有会话组织，管理面 theme 子资源会 401，只能走这条
 * 公开链路。解析失败返回 null，页面回退个人/品牌默认主题。
 */
export async function getOrganizationPublicTheme(
  slug: string,
): Promise<UpdateOrganizationThemePayload | null> {
  try {
    const response = await requestDocumentJson(ORGANIZATION_BY_SLUG(slug));
    return organizationPublicThemeSchema.parse(response).theme;
  } catch (error) {
    captureError(error, { slug, fn: "getOrganizationPublicTheme" });
    return null;
  }
}

export async function getOrganizationSecurityPolicy(
  id: string,
): Promise<UpdateOrganizationSecurityPolicyPayload> {
  const response = await requestDocumentJson(ORGANIZATION_SECURITY_POLICY(id));
  return response as UpdateOrganizationSecurityPolicyPayload;
}

/** List applications for an organization. */
export async function getOrganizationApplications(orgId: string, page = 1, pageSize = 50) {
  const query = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  });
  const response = await requestDocumentJson(
    `${ORGANIZATION_APPLICATIONS(orgId)}?${query.toString()}`,
  );
  return response as { items: unknown[]; page: { page: number; pageSize: number; total: number } };
}

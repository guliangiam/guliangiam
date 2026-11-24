import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { pageMetaSchema } from "./schemas";
import {
  PROVIDERS,
  PROVIDER_BY_ID,
  PROVIDER_TEST_CONNECTION,
  PROVIDER_CONFIG,
} from "@/shared/api/paths";
import { getIamCaptchaProvidersPaginated } from "./captcha-provider";
import { getIamNotificationProvidersPaginated } from "./notification-provider";
import { getIamFaceIdProvidersPaginated } from "./face-id-provider";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type ProviderDetail = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  category: string;
  providerType: string;
  status: string;
  subType: string;
  method: string;
  clientId: string;
  clientSecret: string;
  clientSecretSet: boolean;
  clientSecretMasked: string;
  clientId2: string;
  clientSecret2: string;
  clientSecret2Set: boolean;
  clientSecret2Masked: string;
  authUrl: string;
  tokenUrl: string;
  userInfoUrl: string;
  callbackUrl: string;
  signingAlg: string;
  scopes: string[];
  idpEntityId: string;
  ssoUrl: string;
  sloUrl: string;
  metadata: string;
  issuerUrl: string;
  region: string;
  host: string;
  port: number;
  databaseType: string;
  userMapping: Record<string, unknown>;
  template: string;
  customApi: string;
  customHeader: string;
  customBody: string;
  admin: string;
  password: string;
  passwordSet: boolean;
  passwordMasked: string;
  appId: string;
  agentId: string;
  appSecretSet: boolean;
  appSecretMasked: string;
  contentType: string;
  sender: string;
  receiver: string;
  providerUrl: string;
  enableSignUp: boolean;
  enablePkce: boolean;
  emailRegex: string;
  enableProxy: boolean;
  httpHeaders: Record<string, string>;
  requireMessageAuthenticator: boolean;
  enableSignAuthnRequest: boolean;
  customLogoutUrl: string;
  customLogo: string;
  cert: string;
  state: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type CreateProviderPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  category: string;
  providerType: string;
  subType?: string;
  method?: string;
  clientId?: string;
  clientSecret?: string;
  clientId2?: string;
  clientSecret2?: string;
  authUrl?: string;
  tokenUrl?: string;
  userInfoUrl?: string;
  callbackUrl?: string;
  signingAlg?: string;
  scopes?: string[];
  idpEntityId?: string;
  ssoUrl?: string;
  sloUrl?: string;
  metadata?: string;
  issuerUrl?: string;
  region?: string;
  host?: string;
  port?: number;
  databaseType?: string;
  userMapping?: Record<string, unknown>;
  template?: string;
  customApi?: string;
  customHeader?: string;
  customBody?: string;
  admin?: string;
  password?: string;
  appId?: string;
  agentId?: string;
  contentType?: string;
  sender?: string;
  receiver?: string;
  providerUrl?: string;
  emailRegex?: string;
  enablePkce?: boolean;
  enableProxy?: boolean;
  httpHeaders?: Record<string, string>;
  requireMessageAuthenticator?: boolean;
  enableSignAuthnRequest?: boolean;
  customLogoutUrl?: string;
  customLogo?: string;
  cert?: string;
  state?: string;
};

// 写操作统一返回 `{status, msg}` 操作信封（开发规范 §4.1.2）；创建接口额外
// 返回新资源 id，调用方直接使用该 id，不再按 name 回查（回查会因重名/搜索
// 延迟拿到空 id）。
export type CreateProviderResult = {
  status: string;
  msg: string;
  id: string;
};

export type UpdateProviderPayload = {
  displayName?: string;
  category?: string;
  providerType?: string;
  subType?: string;
  method?: string;
  clientId?: string;
  clientSecret?: string;
  clientId2?: string;
  clientSecret2?: string;
  authUrl?: string;
  tokenUrl?: string;
  userInfoUrl?: string;
  callbackUrl?: string;
  signingAlg?: string;
  scopes?: string[];
  idpEntityId?: string;
  ssoUrl?: string;
  sloUrl?: string;
  metadata?: string;
  issuerUrl?: string;
  region?: string;
  host?: string;
  port?: number;
  databaseType?: string;
  userMapping?: Record<string, unknown>;
  template?: string;
  customApi?: string;
  customHeader?: string;
  customBody?: string;
  admin?: string;
  password?: string;
  appId?: string;
  agentId?: string;
  contentType?: string;
  sender?: string;
  receiver?: string;
  providerUrl?: string;
  emailRegex?: string;
  enablePkce?: boolean;
  enableProxy?: boolean;
  httpHeaders?: Record<string, string>;
  requireMessageAuthenticator?: boolean;
  enableSignAuthnRequest?: boolean;
  customLogoutUrl?: string;
  customLogo?: string;
  cert?: string;
  state?: string;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const providerListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  displayName: z.string(),
  category: z.string(),
  providerType: z.string(),
  subType: z.string().default(""),
  // 后端 ProviderListItem 已返回 status/enabled（internal/app/providers/dto.go）；
  // 统一列表的状态列直接读这两个字段，无需再拉详情。
  status: z.string().default(""),
  enabled: z.boolean().default(false),
  clientId: z.string(),
  providerUrl: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const providerListResponseSchema = z.object({
  items: z.array(providerListItemSchema),
  page: pageMetaSchema,
});

const providerDetailSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  displayName: z.string(),
  category: z.string(),
  providerType: z.string(),
  status: z.string().default("draft"),
  subType: z.string().default(""),
  method: z.string().default(""),
  clientId: z.string(),
  // 服务端不回显机密：详情/更新接口省略 clientSecret（或返回脱敏值），
  // 因此这里必须是可选默认空串，否则 z.object 校验直接抛 invalid_type。
  clientSecret: z.string().default(""),
  // 凭据指示位（internal/app/providers/dto.go）：不回显明文，只回显是否已设置
  // 与脱敏尾段；缺失会误导用户以为密钥未配置而重复填写或误清空。
  clientSecretSet: z.boolean().default(false),
  clientSecretMasked: z.string().default(""),
  clientId2: z.string().default(""),
  clientSecret2: z.string().default(""),
  clientSecret2Set: z.boolean().default(false),
  clientSecret2Masked: z.string().default(""),
  authUrl: z.string().default(""),
  tokenUrl: z.string().default(""),
  userInfoUrl: z.string().default(""),
  callbackUrl: z.string().default(""),
  signingAlg: z.string().default(""),
  scopes: z.array(z.string()).catch([]),
  idpEntityId: z.string().default(""),
  ssoUrl: z.string().default(""),
  sloUrl: z.string().default(""),
  metadata: z.string().default(""),
  issuerUrl: z.string().default(""),
  region: z.string().default(""),
  host: z.string().default(""),
  port: z.number().default(0),
  databaseType: z.string().default(""),
  // 服务端 user_mapping 列可空：JSON null 时 Zod 的 .default() 不会生效
  // （default 只覆盖 undefined），因此显式把 null 归一为空对象。
  userMapping: z
    .record(z.string(), z.unknown())
    .nullish()
    .transform((value) => value ?? {}),
  template: z.string().default(""),
  customApi: z.string().default(""),
  customHeader: z.string().default(""),
  customBody: z.string().default(""),
  admin: z.string().default(""),
  password: z.string().default(""),
  passwordSet: z.boolean().default(false),
  passwordMasked: z.string().default(""),
  appId: z.string().default(""),
  agentId: z.string().default(""),
  appSecretSet: z.boolean().default(false),
  appSecretMasked: z.string().default(""),
  contentType: z.string().default(""),
  sender: z.string().default(""),
  receiver: z.string().default(""),
  providerUrl: z.string(),
  enableSignUp: z.boolean().default(false),
  enablePkce: z.boolean().default(false),
  emailRegex: z.string().default(""),
  enableProxy: z.boolean().default(false),
  httpHeaders: z.record(z.string(), z.string()).catch({}),
  requireMessageAuthenticator: z.boolean().default(false),
  enableSignAuthnRequest: z.boolean().default(false),
  customLogoutUrl: z.string().default(""),
  customLogo: z.string().default(""),
  cert: z.string().default(""),
  state: z.string().default(""),
  createdAt: z.string(),
  updatedAt: z.string(),
  revision: z.number(),
});

const createProviderResultSchema = z.object({
  status: z.string(),
  msg: z.string(),
  id: z.string(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toProviderItem(item: z.infer<typeof providerListItemSchema>) {
  return {
    id: item.id,
    organizationId: item.organizationId,
    name: item.name,
    displayName: item.displayName,
    category: item.category,
    type: item.providerType,
    subType: item.subType,
    status: item.status,
    enabled: item.enabled,
    clientId: item.clientId,
    providerUrl: item.providerUrl,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  };
}

function toProviderDetail(item: z.infer<typeof providerDetailSchema>): ProviderDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    name: item.name,
    displayName: item.displayName,
    category: item.category,
    providerType: item.providerType,
    status: item.status,
    subType: item.subType,
    method: item.method,
    clientId: item.clientId,
    clientSecret: item.clientSecret,
    clientSecretSet: item.clientSecretSet,
    clientSecretMasked: item.clientSecretMasked,
    clientId2: item.clientId2,
    clientSecret2: item.clientSecret2,
    clientSecret2Set: item.clientSecret2Set,
    clientSecret2Masked: item.clientSecret2Masked,
    authUrl: item.authUrl,
    tokenUrl: item.tokenUrl,
    userInfoUrl: item.userInfoUrl,
    callbackUrl: item.callbackUrl,
    signingAlg: item.signingAlg,
    scopes: item.scopes,
    idpEntityId: item.idpEntityId,
    ssoUrl: item.ssoUrl,
    sloUrl: item.sloUrl,
    metadata: item.metadata,
    issuerUrl: item.issuerUrl,
    region: item.region,
    host: item.host,
    port: item.port,
    databaseType: item.databaseType,
    userMapping: item.userMapping,
    template: item.template,
    customApi: item.customApi,
    customHeader: item.customHeader,
    customBody: item.customBody,
    admin: item.admin,
    password: item.password,
    passwordSet: item.passwordSet,
    passwordMasked: item.passwordMasked,
    appId: item.appId,
    agentId: item.agentId,
    appSecretSet: item.appSecretSet,
    appSecretMasked: item.appSecretMasked,
    contentType: item.contentType,
    sender: item.sender,
    receiver: item.receiver,
    providerUrl: item.providerUrl,
    enableSignUp: item.enableSignUp,
    enablePkce: item.enablePkce,
    emailRegex: item.emailRegex,
    enableProxy: item.enableProxy,
    httpHeaders: item.httpHeaders,
    requireMessageAuthenticator: item.requireMessageAuthenticator,
    enableSignAuthnRequest: item.enableSignAuthnRequest,
    customLogoutUrl: item.customLogoutUrl,
    customLogo: item.customLogo,
    cert: item.cert,
    state: item.state,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getProviders(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }

  const response = await requestDocumentJson(`${PROVIDERS}?${query.toString()}`);
  return providerListResponseSchema.parse(response).items.map(toProviderItem);
}

export type ProviderListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  category?: string;
  providerType?: string;
};

export async function getProvidersPaginated(params?: ProviderListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.category) query.set("category", params.category);
  if (params?.providerType) query.set("providerType", params.providerType);

  const response = await requestDocumentJson(`${PROVIDERS}?${query.toString()}`);
  const parsed = providerListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toProviderItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

// ─────────────────────────────────────────────
// Unified Provider Center (aggregated list)
// ─────────────────────────────────────────────

/** 统一提供商列表的行来源；决定编辑/删除走哪个端点。 */
export type UnifiedProviderSource = "unified" | "captcha" | "notification" | "face-id";

/**
 * 统一提供商列表行：类别（category）+ 身份提供商类型（type）是唯一的分类依据。
 * captcha/notification/face-id 行来自各自专用表（运行期事实源，见
 * OPEN-FE-09），在列表层归一为同一形状；操作按 source 分流回专用端点。
 */
export type UnifiedProviderRow = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  category: string;
  type: string;
  subType: string;
  enabled: boolean;
  clientId: string;
  providerUrl: string;
  source: UnifiedProviderSource;
  revision?: number;
  createdAt: string;
  updatedAt: string;
};

/**
 * 聚合抓取的每页大小：后端列表端点 pageSize 上限为 100（超出返回 422
 * `pageSize must be an integer between 1 and 100`），因此按该上限翻页抓全量，
 * 合并后再做客户端分页。
 */
const UNIFIED_MERGE_PAGE_SIZE = 100;
/** 翻页安全上限，避免异常 total 造成无限循环。 */
const UNIFIED_MERGE_MAX_PAGES = 50;

/** 按后端 pageSize 上限翻页抓取单一来源的全部行；空页或已抓满 total 即停。 */
async function collectAllPages<T>(
  fetchPage: (page: number) => Promise<{ items: T[]; total: number }>,
): Promise<T[]> {
  const rows: T[] = [];
  for (let page = 1; page <= UNIFIED_MERGE_MAX_PAGES; page += 1) {
    const { items, total } = await fetchPage(page);
    rows.push(...items);
    if (items.length === 0 || rows.length >= total) break;
  }
  return rows;
}

/** 类型过滤归一：`reCAPTCHA`/`recaptcha`、`DingTalk`/`dingtalk` 视为同一值。 */
function normalizeProviderType(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[-_\s]/g, "");
}

/**
 * 统一提供商分页列表：并行拉取统一 /providers 与命中类别的专用端点，
 * 归一合并后按名称排序、客户端分页。organizationId 仅平台管理员生效，
 * 租户隔离由各端点服务端的 trustedOrgID 裁决保证。
 */
export async function getUnifiedProvidersPaginated(
  params?: ProviderListParams,
): Promise<{ items: UnifiedProviderRow[]; page: number; pageSize: number; total: number }> {
  const page = params?.page ?? 1;
  const pageSize = params?.pageSize ?? 50;
  const { search, organizationId, category, providerType } = params ?? {};
  const wantCategory = (target: string) => !category || category === target;

  const tasks: Promise<UnifiedProviderRow[]>[] = [
    collectAllPages((p) =>
      getProvidersPaginated({
        page: p,
        pageSize: UNIFIED_MERGE_PAGE_SIZE,
        search,
        organizationId,
        category,
        providerType,
      }),
    ).then((items) =>
      items.map((item) => ({
        id: item.id,
        organizationId: item.organizationId,
        name: item.name,
        displayName: item.displayName,
        category: item.category,
        type: item.type,
        subType: item.subType,
        enabled: item.enabled,
        clientId: item.clientId,
        providerUrl: item.providerUrl,
        source: "unified" as const,
        createdAt: item.createdAt,
        updatedAt: item.updatedAt,
      })),
    ),
  ];

  if (wantCategory("Captcha")) {
    tasks.push(
      collectAllPages((p) =>
        getIamCaptchaProvidersPaginated({
          page: p,
          pageSize: UNIFIED_MERGE_PAGE_SIZE,
          search,
          organizationId,
        }),
      ).then((items) =>
        items.map((item) => ({
          id: item.id,
          organizationId: item.organizationId,
          name: item.name,
          displayName: item.name,
          category: "Captcha",
          type: item.providerType,
          subType: "",
          enabled: item.isEnabled,
          clientId: item.siteKey,
          providerUrl: "",
          source: "captcha" as const,
          revision: item.revision,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        })),
      ),
    );
  }

  if (wantCategory("Notification")) {
    tasks.push(
      collectAllPages((p) =>
        getIamNotificationProvidersPaginated({
          page: p,
          pageSize: UNIFIED_MERGE_PAGE_SIZE,
          search,
          organizationId,
        }),
      ).then((items) =>
        items.map((item) => ({
          id: item.id,
          organizationId: item.organizationId,
          name: item.name,
          displayName: item.name,
          category: "Notification",
          type: item.providerType,
          subType: "",
          enabled: item.isEnabled,
          clientId: "",
          providerUrl: "",
          source: "notification" as const,
          revision: item.revision,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        })),
      ),
    );
  }

  if (wantCategory("FaceId")) {
    tasks.push(
      collectAllPages((p) =>
        getIamFaceIdProvidersPaginated({
          page: p,
          pageSize: UNIFIED_MERGE_PAGE_SIZE,
          search,
          organizationId,
        }),
      ).then((items) =>
        items.map((item) => ({
          id: item.id,
          organizationId: item.organizationId,
          name: item.name,
          displayName: item.name,
          category: "FaceId",
          type: item.providerType,
          subType: "",
          enabled: item.isEnabled,
          clientId: "",
          providerUrl: "",
          source: "face-id" as const,
          revision: item.revision,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        })),
      ),
    );
  }

  let rows = (await Promise.all(tasks)).flat();
  if (providerType) {
    // 专用源的类型值是后端 kind（recaptcha/dingtalk），统一源可能是 UI 值
    //（reCAPTCHA/DingTalk），归一后比较。
    const target = normalizeProviderType(providerType);
    rows = rows.filter((row) => normalizeProviderType(row.type) === target);
  }
  rows.sort((a, b) => a.name.localeCompare(b.name));
  const total = rows.length;
  const start = (page - 1) * pageSize;
  return { items: rows.slice(start, start + pageSize), page, pageSize, total };
}

/**
 * 详情级端点在服务端强制可信租户：平台管理员会话组织可能与目标提供商所属
 * 组织不同，必须把目标组织显式作为 query 传入，否则请求会落到会话组织
 * 导致 404。租户用户可不传。
 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export async function getProvider(id: string, organizationId?: string): Promise<ProviderDetail> {
  // 同 organization/application：详情 GET 的 `private, max-age=30` 会让
  // 保存/启用后的失效重取读到旧 revision 与旧状态；编辑器必须绕开 HTTP 缓存。
  const response = await requestDocumentJson(withOrgQuery(PROVIDER_BY_ID(id), organizationId), {
    cache: "no-store",
  });
  return toProviderDetail(providerDetailSchema.parse(response));
}

export async function createProvider(
  payload: CreateProviderPayload,
): Promise<CreateProviderResult> {
  const response = await requestDocumentJson(`${PROVIDERS}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
  return createProviderResultSchema.parse(response);
}

export async function updateProvider(
  id: string,
  payload: UpdateProviderPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(PROVIDER_BY_ID(id), organizationId), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function deleteProvider(id: string, organizationId?: string) {
  await requestDocumentJson(withOrgQuery(PROVIDER_BY_ID(id), organizationId), {
    method: "DELETE",
  });
}

export type TestConnectionResult = {
  connected: boolean;
  error?: string;
};

export async function testProviderConnection(id: string, organizationId?: string) {
  const result = await requestDocumentJson(
    withOrgQuery(PROVIDER_TEST_CONNECTION(id), organizationId),
    {
      method: "POST",
    },
  );
  return result as TestConnectionResult;
}

/** 探测通过后的显式启用；与探测分离，避免测试动作产生状态/审计写入。 */
export async function enableProvider(id: string, organizationId?: string): Promise<void> {
  await requestDocumentJson(withOrgQuery(`${PROVIDER_BY_ID(id)}/enable`, organizationId), {
    method: "POST",
  });
}

// ─────────────────────────────────────────────
// Provider Config（非机密 per-provider 选项，如 IDV 的 idvSceneId）
// ─────────────────────────────────────────────

/**
 * 统一 create/patch 请求体不含 config（后端 DisallowUnknownFields），非机密
 * 选项走独立端点 GET/PUT /providers/{id}/config。configType 用 "default"：
 * 运行期 IDV resolver 经 GetConfig 读取最新一条非 registry 配置行
 *（internal/app/providers/idv_resolver.go），与 unified_service_test 约定一致。
 */
export const PROVIDER_CONFIG_TYPE_DEFAULT = "default";

const providerConfigResponseSchema = z.object({
  providerId: z.string(),
  configType: z.string().default(""),
  // 无配置行时后端返回空对象；null 也归一为空对象（.default 不覆盖 null）。
  config: z
    .record(z.string(), z.unknown())
    .nullish()
    .transform((value) => value ?? {}),
});

/** 读取提供商的非机密配置 map；无配置行时返回 {}（后端不 404）。 */
export async function getProviderConfig(
  id: string,
  organizationId?: string,
): Promise<Record<string, unknown>> {
  const response = await requestDocumentJson(withOrgQuery(PROVIDER_CONFIG(id), organizationId), {
    cache: "no-store",
  });
  return providerConfigResponseSchema.parse(response).config;
}

/** 整体写入提供商配置 map（PUT 为全量替换，调用方需带上既有键）。 */
export async function updateProviderConfig(
  id: string,
  config: Record<string, unknown>,
  organizationId?: string,
): Promise<void> {
  await requestDocumentJson(withOrgQuery(PROVIDER_CONFIG(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ configType: PROVIDER_CONFIG_TYPE_DEFAULT, config }),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

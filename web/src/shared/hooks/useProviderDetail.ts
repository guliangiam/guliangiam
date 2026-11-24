import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App } from "antd";
import type { FormInstance } from "antd";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  createProvider,
  deleteProvider,
  enableProvider,
  getProvider,
  getProviderConfig,
  updateProvider,
  updateProviderConfig,
  type ProviderDetail,
  type CreateProviderPayload,
  type UpdateProviderPayload,
} from "@/shared/api/iam/provider-extended";
import { createIamCaptchaProvider } from "@/shared/api/iam/captcha-provider";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { captchaUITypeToKind } from "@/shared/hooks/useProviderCategories";
import { assembleCaptchaConfig } from "@/shared/ui/ProviderFields/captcha-constants";
import { useSessionStore } from "@/features/session/session-store";
import { localizedError } from "@/shared/lib/server-error";

// ─────────────────────────────────────────────
// Provider Form Values Type
// ─────────────────────────────────────────────

export type ProviderHttpHeaderRow = {
  key: string;
  value: string;
};

export type ProviderFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  category: string;
  providerType: string;
  subType: string;
  method: string;
  clientId: string;
  clientSecret: string;
  clientId2: string;
  clientSecret2: string;
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
  template: string;
  customApi: string;
  customHeader: string;
  customBody: string;
  admin: string;
  password: string;
  appId: string;
  agentId: string;
  contentType: string;
  sender: string;
  receiver: string;
  providerUrl: string;
  userMapping: string;
  enableSignUp: boolean;
  enablePkce: boolean;
  emailRegex: string;
  enableProxy: boolean;
  httpHeaders: ProviderHttpHeaderRow[];
  requireMessageAuthenticator: boolean;
  enableSignAuthnRequest: boolean;
  customLogoutUrl: string;
  customLogo: string;
  cert: string;
  state: string;
  /** IDVerification 认证场景 ID（Aliyun CloudAuth SceneId），存于 provider_configs.config.idvSceneId。 */
  idvSceneId: string;
  signinHtml: string;
  customCss: string;
  customJs: string;
};

// ─────────────────────────────────────────────
// Form Value Mappers
// ─────────────────────────────────────────────

/** Create default form values for a new provider */
export function createDefaultProvider(organizationId: string): ProviderFormValues {
  return {
    organizationId,
    name: "",
    displayName: "",
    category: "OAuth",
    providerType: "Custom",
    subType: "",
    method: "",
    clientId: "",
    clientSecret: "",
    clientId2: "",
    clientSecret2: "",
    authUrl: "",
    tokenUrl: "",
    userInfoUrl: "",
    callbackUrl: "",
    signingAlg: "",
    scopes: [],
    idpEntityId: "",
    ssoUrl: "",
    sloUrl: "",
    metadata: "",
    issuerUrl: "",
    region: "",
    host: "",
    port: 0,
    databaseType: "",
    template: "",
    customApi: "",
    customHeader: "",
    customBody: "",
    admin: "",
    password: "",
    appId: "",
    agentId: "",
    contentType: "",
    sender: "",
    receiver: "",
    providerUrl: "",
    userMapping: "",
    enableSignUp: false,
    enablePkce: false,
    emailRegex: "",
    enableProxy: false,
    httpHeaders: [],
    requireMessageAuthenticator: false,
    enableSignAuthnRequest: false,
    customLogoutUrl: "",
    customLogo: "",
    cert: "",
    state: "",
    idvSceneId: "",
    signinHtml: "",
    customCss: "",
    customJs: "",
  };
}

/** Convert API ProviderDetail to form values (secrets blanked) */
export function detailToFormValues(detail: ProviderDetail): ProviderFormValues {
  return {
    organizationId: detail.organizationId,
    name: detail.name,
    displayName: detail.displayName,
    category: detail.category,
    providerType: detail.providerType,
    subType: detail.subType,
    method: detail.method,
    clientId: detail.clientId,
    clientSecret: "",
    clientId2: detail.clientId2,
    clientSecret2: "",
    authUrl: detail.authUrl,
    tokenUrl: detail.tokenUrl,
    userInfoUrl: detail.userInfoUrl,
    callbackUrl: detail.callbackUrl,
    signingAlg: detail.signingAlg,
    scopes: detail.scopes,
    idpEntityId: detail.idpEntityId,
    ssoUrl: detail.ssoUrl,
    sloUrl: detail.sloUrl,
    metadata: detail.metadata,
    issuerUrl: detail.issuerUrl,
    region: detail.region,
    host: detail.host,
    port: detail.port,
    databaseType: detail.databaseType,
    template: detail.template,
    customApi: detail.customApi,
    customHeader: detail.customHeader,
    customBody: detail.customBody,
    admin: detail.admin,
    password: "",
    appId: detail.appId,
    agentId: detail.agentId,
    contentType: detail.contentType,
    sender: detail.sender,
    receiver: detail.receiver,
    providerUrl: detail.providerUrl,
    userMapping: detail.userMapping ? JSON.stringify(detail.userMapping) : "",
    enableSignUp: detail.enableSignUp ?? false,
    enablePkce: detail.enablePkce ?? false,
    emailRegex: detail.emailRegex ?? "",
    enableProxy: detail.enableProxy ?? false,
    httpHeaders: httpHeadersToRows(detail.httpHeaders),
    requireMessageAuthenticator: detail.requireMessageAuthenticator ?? false,
    enableSignAuthnRequest: detail.enableSignAuthnRequest ?? false,
    customLogoutUrl: detail.customLogoutUrl ?? "",
    customLogo: detail.customLogo ?? "",
    cert: detail.cert ?? "",
    state: detail.state ?? "",
    // 认证场景 ID 不在详情 DTO 中，由独立 config 查询回显（见 configQuery effect）。
    idvSceneId: "",
    signinHtml: "",
    customCss: "",
    customJs: "",
  };
}

/** Parse the JSON-string field produced by UserMappingFields into the API payload shape. */
function parseUserMapping(raw: string): Record<string, unknown> | undefined {
  const trimmed = raw?.trim();
  if (!trimmed) return undefined;
  try {
    const parsed: unknown = JSON.parse(trimmed);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
}

/** Convert the API's string map into rows editable by HttpHeaderTable (echo direction). */
export function httpHeadersToRows(
  headers: Record<string, string> | null | undefined,
): ProviderHttpHeaderRow[] {
  return Object.entries(headers ?? {}).map(([key, value]) => ({ key, value }));
}

/** Convert HttpHeaderTable rows into the string map the API stores (submit direction). */
export function rowsToHttpHeaders(
  rows: ProviderHttpHeaderRow[] | null | undefined,
): Record<string, string> | undefined {
  if (!rows?.length) return undefined;
  const headers: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) continue;
    headers[key] = row.value;
  }
  return Object.keys(headers).length > 0 ? headers : undefined;
}

/** Extract common fields shared between create and update payloads.
 *
 * preserveEmpty=false（创建）：空串归一为 undefined，省略未填字段。
 * preserveEmpty=true（更新）：显式传空串——后端 applyOptionalString 对非 nil
 * 指针赋值，传 "" 才能清空已落库旧值（FE-B-13）；undefined 会被 PATCH 省略而保留旧值。
 * 凭据（clientSecret/password 等）不在此处，仍由 build*Payload 以 `|| undefined`
 * 处理——留空表示保持原密钥。 */
function extractCommonFields(values: ProviderFormValues, preserveEmpty = false) {
  const s = (v: string) => (preserveEmpty ? v : v || undefined);
  return {
    subType: values.subType,
    method: values.method,
    clientId: s(values.clientId),
    clientId2: s(values.clientId2),
    authUrl: s(values.authUrl),
    tokenUrl: s(values.tokenUrl),
    userInfoUrl: s(values.userInfoUrl),
    callbackUrl: s(values.callbackUrl),
    signingAlg: s(values.signingAlg),
    scopes: values.scopes,
    idpEntityId: s(values.idpEntityId),
    ssoUrl: s(values.ssoUrl),
    sloUrl: s(values.sloUrl),
    metadata: s(values.metadata),
    issuerUrl: s(values.issuerUrl),
    region: s(values.region),
    host: s(values.host),
    port: values.port || undefined,
    databaseType: s(values.databaseType),
    userMapping: parseUserMapping(values.userMapping),
    template: s(values.template),
    customApi: s(values.customApi),
    customHeader: s(values.customHeader),
    customBody: s(values.customBody),
    admin: s(values.admin),
    appId: s(values.appId),
    agentId: s(values.agentId),
    contentType: s(values.contentType),
    sender: s(values.sender),
    receiver: s(values.receiver),
    providerUrl: s(values.providerUrl),
    emailRegex: s(values.emailRegex),
    enablePkce: values.enablePkce,
    enableProxy: values.enableProxy,
    httpHeaders: rowsToHttpHeaders(values.httpHeaders),
    requireMessageAuthenticator: values.requireMessageAuthenticator,
    enableSignAuthnRequest: values.enableSignAuthnRequest,
    customLogoutUrl: s(values.customLogoutUrl),
    customLogo: s(values.customLogo),
    cert: s(values.cert),
    state: s(values.state),
  };
}

/** Build the create payload from form values */
export function buildCreatePayload(values: ProviderFormValues): CreateProviderPayload {
  return {
    organizationId: values.organizationId,
    name: values.name,
    displayName: values.displayName,
    category: values.category,
    providerType: values.providerType,
    ...extractCommonFields(values),
    clientSecret: values.clientSecret || undefined,
    clientSecret2: values.clientSecret2 || undefined,
    password: values.password || undefined,
  };
}

/** Build the update payload from form values and current revision */
export function buildUpdatePayload(
  values: ProviderFormValues,
  revision: number,
): UpdateProviderPayload {
  return {
    displayName: values.displayName,
    category: values.category,
    providerType: values.providerType,
    ...extractCommonFields(values, true),
    clientSecret: values.clientSecret || undefined,
    clientSecret2: values.clientSecret2 || undefined,
    password: values.password || undefined,
    revision,
  };
}

/** 主配置 PATCH 已落库、但其后的认证场景 ID config 写入失败（FE-B-08）。
 * 与整体保存失败区分：此时不能提示“保存失败”，否则用户会误以为主配置未生效。 */
export class ProviderSavedConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ProviderSavedConfigError";
  }
}

// ─────────────────────────────────────────────
// Hook: useProviderDetail
// ─────────────────────────────────────────────

export type UseProviderDetailOptions = {
  /** Provider ID from route params — "new" for creation */
  providerId: string;
  /** Ant Design form instance */
  form: FormInstance<ProviderFormValues>;
};

export type UseProviderDetailReturn = {
  /** Whether this is a new provider creation */
  isNew: boolean;
  /** Query for fetching provider detail */
  detailQuery: ReturnType<typeof useQuery<ProviderDetail, Error>>;
  /** Query for organization options */
  orgOptionsQuery: ReturnType<
    typeof useQuery<Awaited<ReturnType<typeof getOrganizationOptions>>, Error>
  >;
  /** Mutation for saving (create or update) */
  saveMutation: ReturnType<typeof useMutation<string, Error, ProviderFormValues>>;
  /** Mutation for explicit activation after a successful probe */
  enableMutation: ReturnType<typeof useMutation<void, Error, void>>;
  /** Mutation for deleting */
  deleteMutation: ReturnType<typeof useMutation<void, Error, void>>;
};

/**
 * Hook that manages provider detail page state: fetching, form population,
 * save (create/update), and delete mutations.
 *
 * Encapsulates all React Query logic, form value mapping, and payload
 * construction that was previously inline in ProviderDetailPage.
 */
export function useProviderDetail(options: UseProviderDetailOptions): UseProviderDetailReturn {
  const { providerId, form } = options;
  const { message } = App.useApp();
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const isNew = providerId === "new";

  // 平台管理员跨组织打开详情时，列表链接携带 ?organizationId=…；
  // 详情级端点不传会回落到会话组织导致 404。
  const [searchParams] = useSearchParams();
  const orgIdFromQuery = searchParams.get("organizationId") || undefined;
  const orgQuerySuffix = orgIdFromQuery
    ? `?organizationId=${encodeURIComponent(orgIdFromQuery)}`
    : "";

  // ── Detail query ──
  const detailQuery = useQuery({
    queryKey: ["provider", providerId, orgIdFromQuery],
    queryFn: () => getProvider(providerId, orgIdFromQuery),
    enabled: !isNew && providerId !== "",
  });
  const resolvedOrgId = detailQuery.data?.organizationId ?? orgIdFromQuery;

  // ── Organization options query ──
  const orgOptionsQuery = useQuery({
    queryKey: ["organization-options"],
    queryFn: () => getOrganizationOptions(),
    enabled: true,
  });

  // ── Provider config query（IDVerification 认证场景 ID 回显）──
  // 非机密选项不在详情 DTO 中，走独立 config 端点；仅 IDVerification 需要。
  const configQuery = useQuery({
    queryKey: ["provider-config", providerId, orgIdFromQuery],
    queryFn: () => getProviderConfig(providerId, orgIdFromQuery),
    enabled: !isNew && providerId !== "" && detailQuery.data?.category === "IDVerification",
  });

  // ── Populate form when data arrives ──
  useEffect(() => {
    if (isNew) {
      form.setFieldsValue(createDefaultProvider(organization?.id ?? ""));
    } else if (detailQuery.data) {
      form.setFieldsValue(detailToFormValues(detailQuery.data));
    }
  }, [form, isNew, detailQuery.data, organization?.id]);

  // 认证场景 ID 由 config 端点回显；声明在详情填充 effect 之后，避免被
  // detailToFormValues 的 idvSceneId:"" 覆盖。
  useEffect(() => {
    if (configQuery.data) {
      const sceneId = configQuery.data.idvSceneId;
      form.setFieldsValue({ idvSceneId: typeof sceneId === "string" ? sceneId : "" });
    }
  }, [form, configQuery.data]);

  // ── Save mutation (create or update) ──
  const saveMutation = useMutation({
    mutationFn: async (values: ProviderFormValues) => {
      if (isNew) {
        // Captcha rows live in the dedicated `captcha_providers` table; the
        // unified endpoint rejects them (P1 422). Governance ruling: unified
        // UI entry, dedicated API — route the submit to the captcha endpoint.
        if (values.category === "Captcha") {
          const kind = captchaUITypeToKind(values.providerType);
          const created = (await createIamCaptchaProvider({
            organizationId: values.organizationId,
            name: values.name,
            providerType: kind,
            siteKey: values.clientId || undefined,
            secretKey: values.clientSecret || undefined,
            config: assembleCaptchaConfig(kind, values as unknown as Record<string, unknown>),
            isEnabled: values.state !== "disabled",
          })) as { id?: string } | undefined;
          return created?.id ?? "";
        }
        // 创建接口直接返回新资源 id，不需要再按 name 回查列表。
        const { id } = await createProvider(buildCreatePayload(values));
        // 认证场景 ID 走独立 config 端点：统一 create 请求体不含 config。
        if (values.category === "IDVerification") {
          await updateProviderConfig(
            id,
            { idvSceneId: values.idvSceneId ?? "" },
            values.organizationId,
          );
        }
        return id;
      }
      await updateProvider(
        providerId,
        buildUpdatePayload(values, detailQuery.data?.revision ?? 1),
        resolvedOrgId,
      );
      if (values.category === "IDVerification") {
        // PUT 为全量替换：先读既有 config 再合并，避免清空 idvEndpoint 等其它键。
        try {
          const current = await getProviderConfig(providerId, resolvedOrgId);
          await updateProviderConfig(
            providerId,
            { ...current, idvSceneId: values.idvSceneId ?? "" },
            resolvedOrgId,
          );
        } catch {
          // 主配置已保存：失效详情/列表缓存避免界面停留旧数据，并用专用文案提示。
          await queryClient.invalidateQueries({ queryKey: ["provider", providerId] });
          await queryClient.invalidateQueries({ queryKey: ["providers"] });
          throw new ProviderSavedConfigError(tr("provider.savedButConfigFailed"));
        }
      }
      return providerId;
    },
    onSuccess: async (id, values) => {
      await queryClient.invalidateQueries({ queryKey: ["providers"] });
      message.success(isNew ? tr("provider.providerCreated") : tr("provider.providerSaved"));
      if (isNew && values.category === "Captcha") {
        await queryClient.invalidateQueries({ queryKey: ["captcha-providers"] });
        navigate("/providers?category=Captcha");
        return;
      }
      if (id) {
        await queryClient.invalidateQueries({ queryKey: ["provider", id] });
        if (values.category === "IDVerification") {
          await queryClient.invalidateQueries({ queryKey: ["provider-config", id] });
        }
        navigate(`/providers/${id}${orgQuerySuffix}`);
        return;
      }
      navigate("/providers");
    },
    onError: (error) => {
      if (error instanceof ProviderSavedConfigError) {
        message.error(localizedError(error, tr, "common:error"));
        return;
      }
      message.error(localizedError(error, tr, "provider.saveFailed"));
    },
  });

  // ── Enable mutation（探测成功后的显式启用）──
  const enableMutation = useMutation({
    mutationFn: async () => {
      await enableProvider(providerId, resolvedOrgId);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["provider", providerId] });
      await queryClient.invalidateQueries({ queryKey: ["providers"] });
      message.success(tr("provider.providerEnabled"));
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "provider.enableFailed"));
    },
  });

  // ── Delete mutation ──
  const deleteMutation = useMutation({
    mutationFn: async () => {
      await deleteProvider(providerId, resolvedOrgId);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["providers"] });
      message.success(tr("provider.providerDeleted"));
      navigate("/providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "provider.deleteFailed"));
    },
  });

  return {
    isNew,
    detailQuery,
    orgOptionsQuery,
    saveMutation,
    enableMutation,
    deleteMutation,
  };
}

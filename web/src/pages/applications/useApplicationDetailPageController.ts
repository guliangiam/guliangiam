import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Form } from "antd";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  createApplication,
  deleteApplication,
  exportApplicationConfig,
  getApplication,
  getApplicationClientCertificates,
  getApplicationDeleteSummary,
  getApplications,
  rotateApplicationClientSecret,
  updateApplication,
  updateApplicationProviderBindings,
} from "@/shared/api/iam/application";
import type {
  ApplicationClientCertificateInput,
  ApplicationDetail,
} from "@/shared/api/iam/application";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { getGroups } from "@/shared/api/iam/group";
import { getIamFaceIdProvidersPaginated } from "@/shared/api/iam/face-id-provider";
import { getIamCaptchaProvidersPaginated } from "@/shared/api/iam/captcha-provider";
import { getIamNotificationProvidersPaginated } from "@/shared/api/iam/notification-provider";
import { useProviderOptions } from "@/shared/hooks/useProviderOptions";
import { getErrorDetails, isErrorCode, isHttpError } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import {
  downloadApplicationExport,
  formatDeleteDescription,
  formatTokenEndpointAuthMethod,
  normalizeSlug,
  deepEqual,
} from "./application-helpers";
import {
  buildApplicationConfigPayload,
  buildCreateApplicationPayload,
  createDefaultApplicationConfigFormValues,
  detailToConfigFormValues,
  normalizeConfigSnapshot,
  UI_CUSTOMIZATION_FIELDS,
  validateApplicationConfigValues,
} from "./application-payload";
import type { ApplicationConfigFormValues } from "./tabs/types";
import { isClientSecretAuthMethod, isMTLSAuthMethod } from "./tabs/types";
import { buildClientCertificateInputs } from "./mtls-certificate";

function pickUiFields(values: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of UI_CUSTOMIZATION_FIELDS) out[field] = values[field];
  return structuredClone(out);
}

/**
 * POST 已成功、但随后的详情 GET 或配置 PATCH 失败。
 *
 * 与“创建失败”不同，记录已经存在；提示必须说明“已创建、配置未加载”，
 * 不能让用户以为整个创建动作失败而重复提交。
 */
class ApplicationCreatedConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ApplicationCreatedConfigError";
  }
}

/** 本次会话里自己刚刚创建过该标识，重试时的 409 不是“别人占用了名字”。 */
class SelfCreatedDuplicateError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SelfCreatedDuplicateError";
  }
}

/**
 * 主配置已保存、但身份提供商绑定未保存。
 *
 * 两部分仍走各自的接口（数据模型未合并），但由一次保存动作串行提交；
 * 后半段失败时必须明确告知“配置已保存”，不能整体报失败让用户重试覆盖。
 */
class ApplicationBindingsPartialError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ApplicationBindingsPartialError";
  }
}

function sameProviderIds(left: string[] | undefined, right: string[] | undefined) {
  const a = [...(left ?? [])].sort();
  const b = [...(right ?? [])].sort();
  return a.length === b.length && a.every((id, index) => id === b[index]);
}

/**
 * 绑定登记表（application_provider_bindings）由可编辑的 providerItems 派生。
 *
 * providerItems.provider 历史上可能存 ID（控制台选择）或名称（导入的
 * Casdoor 数据，后端 social_binding 也兼容两种写法），而 PUT
 * provider-bindings 只接受存在的 provider ID。这里先按 provider 目录把名称
 * 解析成 ID；无法解析的名称跳过，避免主配置已保存后绑定返回 422。
 * provider 目录未加载时保持原值（这些值来自编辑器，通常是 ID）。
 */
function deriveProviderIds(
  items: ApplicationConfigFormValues["providerItems"],
  providers: Array<{ id?: string; name: string }>,
): string[] {
  const raw = [
    ...new Set((items ?? []).map((item) => (item.provider ?? "").trim()).filter(Boolean)),
  ];
  if (providers.length === 0) return raw;

  const knownIDs = new Set<string>();
  const idByName = new Map<string, string>();
  for (const provider of providers) {
    if (!provider.id) continue;
    knownIDs.add(provider.id);
    idByName.set(provider.name, provider.id);
  }

  const derived: string[] = [];
  for (const value of raw) {
    const id = knownIDs.has(value) ? value : idByName.get(value);
    if (id) derived.push(id);
  }
  return derived;
}

type ApplicationConfigSaveInput = {
  values: ApplicationConfigFormValues;
  clientCertificates: ApplicationClientCertificateInput[];
};

async function buildCertificatesForSave(
  tr: (key: string) => string,
  values: ApplicationConfigFormValues,
): Promise<ApplicationClientCertificateInput[]> {
  try {
    return await buildClientCertificateInputs(
      values.mtlsCertificates,
      values.tokenEndpointAuthMethod === "self_signed_tls_client_auth",
    );
  } catch {
    throw new Error(tr("application.validation.certificateInvalid"));
  }
}

export function useApplicationDetailPageController() {
  const { message } = App.useApp();
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const { applicationId = "" } = useParams();
  const isNew = applicationId === "new";
  const { nameMap: orgNameMap } = useOrganizationNames();
  const sessionOrganizationId = organization?.id?.trim() ?? "";

  const [configForm] = Form.useForm<ApplicationConfigFormValues>();

  const watchedOrganizationId = Form.useWatch("organizationId", configForm) ?? "";
  const watchedDisplayName = Form.useWatch("displayName", configForm) ?? "";
  const watchedSlug = Form.useWatch("slug", configForm) ?? "";

  const lastAutoSlugRef = useRef("");
  // 共享表单壳已用 saving/isSubmittingRef 挡住按钮连点；这里再兜一层直接
  // 调用 mutation 的路径：两次并发提交只发出一个 POST。
  const createInFlightRef = useRef<ReturnType<typeof createApplication> | null>(null);
  // 本会话内 POST 201 成功的标识：用于把“自己刚创建过”的 409 与真正重名区分开。
  const createdSlugsRef = useRef<Set<string>>(new Set());
  const [isDirty, setIsDirty] = useState(false);
  // effect 里读取「是否有未保存输入」而无需把 isDirty 放进依赖。
  const isDirtyRef = useRef(false);
  // 已初始化的实体键：只有实体切换才允许 reset 表单，后台 refetch 不得覆盖编辑中的值。
  const initializedConfigEntityRef = useRef<string>("");
  // 灌值完成信号（state 版）：子组件 effect 先于父组件执行，撤销历史必须等
  // setFieldsValue 完成后再捕获基线，否则会把旧实体值当成新实体的起点。
  const [initializedEntityKey, setInitializedEntityKey] = useState("");
  const initialConfigRef = useRef<Record<string, unknown> | null>(null);
  // 界面定制字段组的「已保存基线」：整组重置只回滚这些键，不波及其他 Tab。
  const uiBaselineRef = useRef<Record<string, unknown> | null>(null);
  const [openDeleteConfirm, setOpenDeleteConfirm] = useState(false);
  // One-time plaintext secret from a rotation or from creation. It must never
  // outlive this page; closing the modal discards it for the rest of the session.
  const [clientSecretReveal, setClientSecretReveal] = useState<{
    plain: string;
    prefix: string;
  } | null>(null);
  const afterRevealRef = useRef<(() => void) | null>(null);

  // ─── Queries ───────────────────────────────────────────────────
  const detailQuery = useQuery({
    queryKey: queryKeys.applications.detail(applicationId),
    queryFn: () => getApplication(applicationId),
    enabled: !isNew && applicationId !== "",
  });

  // Registered mTLS certificates (fingerprints only). Loaded only when the
  // application actually uses an mTLS auth method.
  const clientCertificatesQuery = useQuery({
    queryKey: queryKeys.applications.clientCertificates(applicationId),
    queryFn: () =>
      getApplicationClientCertificates(applicationId, detailQuery.data?.organizationId),
    enabled:
      !isNew &&
      applicationId !== "" &&
      isMTLSAuthMethod(detailQuery.data?.tokenEndpointAuthMethod ?? "none"),
  });

  const providersQuery = useProviderOptions(detailQuery.data?.organizationId, {
    enabled: !isNew && detailQuery.data?.organizationId !== undefined,
  });

  // FaceID 提供商存于专用 face_id_providers 表（登录按组织解析），统一 /providers
  // 目录不含这些行。应用「Face ID」登录方式需要能引用一个已配置的 FaceID 提供商，
  // 故在此单独拉取并并入下拉选项；绑定派生（deriveProviderIds）仍只用统一目录，
  // 专用行 ID 不会写入外键指向 providers 表的 application_provider_bindings。
  const faceIdProvidersQuery = useQuery({
    queryKey: ["face-id-providers", "options", detailQuery.data?.organizationId ?? "all"],
    queryFn: () =>
      getIamFaceIdProvidersPaginated({
        organizationId: detailQuery.data?.organizationId,
        page: 1,
        pageSize: 100,
      }),
    enabled: !isNew && detailQuery.data?.organizationId !== undefined,
    staleTime: 60_000,
  });

  // Captcha / Notification 专用台行同样不在统一目录里，但应用的验证码与
  // 通知配置需要引用它们；与 FaceID 相同：仅并入展示选项，绑定派生仍只用统一目录。
  const captchaProvidersQuery = useQuery({
    queryKey: ["captcha-providers", "options", detailQuery.data?.organizationId ?? "all"],
    queryFn: () =>
      getIamCaptchaProvidersPaginated({
        organizationId: detailQuery.data?.organizationId,
        page: 1,
        pageSize: 100,
      }),
    enabled: !isNew && detailQuery.data?.organizationId !== undefined,
    staleTime: 60_000,
  });

  const notificationProvidersQuery = useQuery({
    queryKey: ["notification-providers", "options", detailQuery.data?.organizationId ?? "all"],
    queryFn: () =>
      getIamNotificationProvidersPaginated({
        organizationId: detailQuery.data?.organizationId,
        page: 1,
        pageSize: 100,
      }),
    enabled: !isNew && detailQuery.data?.organizationId !== undefined,
    staleTime: 60_000,
  });

  const deleteSummaryQuery = useQuery({
    queryKey: queryKeys.applications.deleteSummary(applicationId),
    queryFn: () => getApplicationDeleteSummary(applicationId, detailQuery.data?.organizationId),
    enabled: !isNew && applicationId !== "" && openDeleteConfirm,
    staleTime: 30_000,
  });

  const organizationsQuery = useQuery({
    queryKey: queryKeys.organizations.lists(),
    queryFn: () => getOrganizationOptions(),
    // 编辑态也用于把预览里的 organizationId 解析成真实 slug，不再直接展示 UUID。
    enabled: true,
  });

  const slugOrganizationId = isNew
    ? watchedOrganizationId.trim()
    : (detailQuery.data?.organizationId?.trim() ?? "");

  const existingApplicationsQuery = useQuery({
    queryKey: queryKeys.applications.list({ slugValidation: slugOrganizationId }),
    queryFn: () => getApplications(slugOrganizationId || undefined),
    enabled: slugOrganizationId !== "",
  });

  // defaultGroup 存组 name（Casbin sub 口径），选项来自应用所属组织的组树。
  const groupOrganizationId = isNew
    ? watchedOrganizationId.trim()
    : (detailQuery.data?.organizationId?.trim() ?? "");
  const groupsQuery = useQuery({
    queryKey: queryKeys.groups.list({ organizationId: groupOrganizationId, view: "tree" }),
    queryFn: () => getGroups(groupOrganizationId),
    enabled: groupOrganizationId !== "",
    staleTime: 60_000,
  });

  const detail = detailQuery.data ?? null;

  // ─── Dirty state reset on entity switch ────────────────────────
  // Debounced dirty recompute timer；放在这里是因为下面的 entity switch
  // 清理逻辑会读取它（提前声明避免 TDZ）。
  const dirtyDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Clear stale debounce timer too — SPA navigate doesn't unmount the
  // component, so useEffect([]) cleanup only fires on full unmount.
  const loadedEntityKey = isNew ? "new" : (detail?.id ?? "");
  const [syncedEntityKey, setSyncedEntityKey] = useState(loadedEntityKey);
  if (loadedEntityKey !== syncedEntityKey) {
    setSyncedEntityKey(loadedEntityKey);
    setIsDirty(false);
  }

  // Clear the pending dirty timer after an entity switch. Doing this in an
  // effect keeps refs out of the render phase (react(refs)).
  useEffect(() => {
    if (dirtyDebounceRef.current) {
      clearTimeout(dirtyDebounceRef.current);
      dirtyDebounceRef.current = null;
    }
    // 实体切换：同步清掉 dirty ref（不能在 render 期间写 ref）。
    isDirtyRef.current = false;
    // oxlint-disable-next-line react/exhaustive-effect-dependencies -- loadedEntityKey 是本 effect 的触发键（实体切换时丢弃上一个实体遗留的防抖计时器），effect 体内不读取它
  }, [loadedEntityKey]);

  // ─── Form initialization ───────────────────────────────────────
  // 只在「实体真正切换」时 reset + 灌值。后台 refetch（保存后、窗口聚焦）
  // 不得覆盖用户正在编辑的字段，否则会丢失输入并让 revision 与表单脱节。
  useEffect(() => {
    if (isNew) {
      if (initializedConfigEntityRef.current === "new") return;
      initializedConfigEntityRef.current = "new";
      configForm.resetFields();
      const defaults = createDefaultApplicationConfigFormValues(sessionOrganizationId);
      configForm.setFieldsValue(defaults);
      initialConfigRef.current = normalizeConfigSnapshot(tr, defaults);
      uiBaselineRef.current = pickUiFields(configForm.getFieldsValue(true));
      setInitializedEntityKey("new");
      return;
    }

    if (!detailQuery.data) return;

    const entityKey = detailQuery.data.id;
    const formValues = detailToConfigFormValues(detailQuery.data);

    if (initializedConfigEntityRef.current !== entityKey) {
      initializedConfigEntityRef.current = entityKey;
      configForm.resetFields();
      configForm.setFieldsValue(formValues);
      // Baseline must match the complete form state so dirty detection works for ALL fields,
      // not just the ~16 identity fields we used to pick manually.
      initialConfigRef.current = normalizeConfigSnapshot(tr, formValues);
      uiBaselineRef.current = pickUiFields(configForm.getFieldsValue(true));
      setInitializedEntityKey(entityKey);
      return;
    }

    // 同一实体：仅在无未保存修改时同步乐观锁 revision，避免旧值导致保存 409。
    if (!isDirtyRef.current) {
      configForm.setFieldValue("revision", formValues.revision);
      initialConfigRef.current = normalizeConfigSnapshot(tr, configForm.getFieldsValue(true));
      uiBaselineRef.current = pickUiFields(configForm.getFieldsValue(true));
    }
  }, [configForm, detailQuery.data, isNew, sessionOrganizationId, tr]);

  // ─── Auto-slug on new application ─────────────────────────────
  useEffect(() => {
    if (!isNew) return;

    const nextSlug = normalizeSlug(watchedDisplayName);
    const currentSlug = watchedSlug.trim();
    const shouldAutofill = currentSlug === "" || currentSlug === lastAutoSlugRef.current;
    if (!shouldAutofill || nextSlug === currentSlug) {
      if (currentSlug === "") lastAutoSlugRef.current = "";
      return;
    }

    lastAutoSlugRef.current = nextSlug;
    configForm.setFieldValue("slug", nextSlug);
  }, [configForm, isNew, watchedDisplayName, watchedSlug]);

  // ─── Dirty change handlers ────────────────────────────────────
  // Debounced to avoid re-running deepEqual on every keystroke inside large text areas.
  useEffect(
    () => () => {
      if (dirtyDebounceRef.current) clearTimeout(dirtyDebounceRef.current);
    },
    [],
  );
  const handleConfigFormChange = useCallback(() => {
    if (dirtyDebounceRef.current) clearTimeout(dirtyDebounceRef.current);
    dirtyDebounceRef.current = setTimeout(() => {
      const currentValues = configForm.getFieldsValue(true);
      const currentSnapshot = normalizeConfigSnapshot(tr, currentValues);
      const nextDirty = !deepEqual(currentSnapshot, initialConfigRef.current);
      isDirtyRef.current = nextDirty;
      setIsDirty(nextDirty);
    }, 150);
  }, [configForm, tr]);

  // ─── Mutations ─────────────────────────────────────────────────
  const createApplicationMutation = useMutation({
    mutationFn: (input: ApplicationConfigSaveInput) => {
      const { values, clientCertificates } = input;
      // in-flight 去重：共享表单壳的按钮禁用是异步状态，直接调用 mutation
      // 的并发路径仍在；两次提交共享同一个 POST 结果。
      if (createInFlightRef.current) return createInFlightRef.current;
      const request = (async () => {
        const slugKey = `${values.organizationId}:${values.slug}`;
        let created: Awaited<ReturnType<typeof createApplication>>;
        try {
          // mTLS 证书与 application 同事务落库；POST 之后再补写会留下“应用
          // 已创建但证书缺失”的不可用窗口。
          created = await createApplication(
            buildCreateApplicationPayload(tr, values, clientCertificates),
          );
        } catch (error) {
          // 前端已挡住双提交；走到这里的 409 若命中了本会话刚创建成功的标识，
          // 说明是“我上一次其实成功了”，提示刷新列表而不是让用户改名。
          if (isErrorCode(error, "already_exists") && createdSlugsRef.current.has(slugKey)) {
            await queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() });
            throw new SelfCreatedDuplicateError(tr("application.duplicateOwnCreated"));
          }
          throw error;
        }
        createdSlugsRef.current.add(slugKey);
        // POST /applications 只接受基础字段，创建表单里填的扩展配置必须再由 PATCH
        // 落库，否则会在「创建成功」后静默丢失。
        try {
          // 创建后的详情级请求必须显式携带目标组织：平台管理员会话没有固定
          // 组织（或会话组织 ≠ 目标组织）时，不带 query 的请求会落到会话组织
          // 导致 404，从而触发「应用已创建，但配置加载失败」。
          const createdDetail = await getApplication(created.id, values.organizationId);
          await updateApplication(
            created.id,
            buildApplicationConfigPayload(tr, values, createdDetail),
            values.organizationId,
          );
          const derivedProviderIds = deriveProviderIds(
            values.providerItems,
            providersQuery.data ?? [],
          );
          if (derivedProviderIds.length > 0) {
            await updateApplicationProviderBindings(
              created.id,
              derivedProviderIds,
              values.organizationId,
            );
          }
        } catch {
          // POST 已落库：后续 GET/PATCH 失败时记录依然存在。必须让列表失效，
          // 否则界面只报通用错误且列表停留在旧数据（写成功、列表未刷新）。
          await queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() });
          throw new ApplicationCreatedConfigError(tr("application.createdButConfigFailed"));
        }
        return created;
      })();
      createInFlightRef.current = request;
      const clear = () => {
        if (createInFlightRef.current === request) createInFlightRef.current = null;
      };
      request.then(clear, clear);
      return request;
    },
    onError: (error) => {
      // 自带 onError 即接管展示（AppProviders 不再发通用兜底 toast）。
      if (error instanceof ApplicationCreatedConfigError) {
        message.error(localizedError(error, tr, "common:error"));
        return;
      }
      if (error instanceof SelfCreatedDuplicateError) {
        message.warning(localizedError(error, tr, "common:error"));
        return;
      }
      message.error(serverErrorMessage(error, tr, "common:operationFailed"));
    },
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.detail(result.id) }),
      ]);
      message.success(tr("application.appCreated"));
      if (result.clientSecretPlain) {
        // The server generated the secret for a confidential method. Show it
        // before leaving the create page; the plaintext only exists in this
        // response, so navigation is deferred until the dialog is dismissed.
        afterRevealRef.current = () => navigate(`/applications/${result.id}`);
        setClientSecretReveal({ plain: result.clientSecretPlain, prefix: "" });
        return;
      }
      navigate(`/applications/${result.id}`);
    },
  });

  const applicationConfigMutation = useMutation({
    mutationFn: async ({ values, clientCertificates }: ApplicationConfigSaveInput) => {
      const current = detailQuery.data;
      if (!current) throw new Error(tr("application.detailNotLoaded"));

      // mTLS 证书与认证方式在同一 PATCH 内提交，服务端同事务落库；不再有
      // “配置已保存、证书未保存”的中间态。
      const submitPatch = (base: typeof current, saveValues: ApplicationConfigFormValues) =>
        updateApplication(
          applicationId,
          buildApplicationConfigPayload(tr, saveValues, base, clientCertificates),
          base.organizationId,
        );

      let patchResult;
      try {
        patchResult = await submitPatch(current, values);
      } catch (error) {
        // 详情缓存可能落后（尤其是钥匙轮换、他处保存之后）：重新读取真实
        // revision，仅同步乐观锁字段并保留用户编辑值，自动重试一次。
        if (!isErrorCode(error, "conflict")) throw error;
        const fresh = await getApplication(applicationId, current.organizationId);
        if (!fresh) throw error;
        configForm.setFieldValue("revision", fresh.revision);
        patchResult = await submitPatch(fresh, { ...values, revision: fresh.revision });
      }
      let revision = patchResult?.revision ?? 0;

      // 绑定仍写 provider_bindings（与 providerItems 是两套存储），但由同一
      // 次保存动作提交；登记清单以 providerItems 为唯一可编辑源，保存时派生
      // 并同步。主配置已落库时必须把“绑定失败”与“保存失败”区分开。
      const derivedProviderIds = deriveProviderIds(values.providerItems, providersQuery.data ?? []);
      if (!sameProviderIds(derivedProviderIds, current.providerIds)) {
        try {
          const bindingsResult = await updateApplicationProviderBindings(
            applicationId,
            derivedProviderIds,
            current.organizationId,
          );
          // 绑定写入同样 bump revision，必须就地同步，否则下一次保存必然 409。
          if (bindingsResult && bindingsResult.revision > 0) revision = bindingsResult.revision;
        } catch (error) {
          throw new ApplicationBindingsPartialError(
            tr("application.bindingsPartialFailure", {
              reason: serverErrorMessage(error, tr, "common:operationFailed"),
            }),
          );
        }
      }
      // Switching from a non-secret method to a secret method while no active
      // secret exists would leave the client unable to authenticate. Generate
      // one through the rotation endpoint (the only path that returns plaintext).
      let rotation = null;
      if (
        isClientSecretAuthMethod(values.tokenEndpointAuthMethod) &&
        !isClientSecretAuthMethod(current.tokenEndpointAuthMethod) &&
        !current.clientSecretSet &&
        (values.clientSecret ?? "").trim() === ""
      ) {
        rotation = await rotateApplicationClientSecret(applicationId, current.organizationId);
      }
      return { rotation, revision };
    },
    onSuccess: async ({ rotation, revision }) => {
      // 立即同步落库后的 revision，避免在详情 refetch 完成前再次保存触发假 409。
      if (revision > 0) {
        configForm.setFieldValue("revision", revision);
      }
      // IMPORTANT: update baseline BEFORE setIsDirty(false), otherwise the dirty
      // baseline stays at the old value while the form now holds saved values.
      initialConfigRef.current = normalizeConfigSnapshot(tr, {
        ...configForm.getFieldsValue(true),
        ...(revision > 0 ? { revision } : {}),
      });
      isDirtyRef.current = false;
      setIsDirty(false);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.detail(applicationId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() }),
        queryClient.invalidateQueries({
          queryKey: queryKeys.applications.clientCertificates(applicationId),
        }),
      ]);
      // 保存成功后以详情读回值为准（applications.md）：用服务端归一化结果
      // 刷新表单与基线，避免「提示成功但配置未生效」。用户已开启新一轮编辑
      // 时不覆盖其输入。
      const fresh = queryClient.getQueryData<ApplicationDetail>(
        queryKeys.applications.detail(applicationId),
      );
      if (fresh && !isDirtyRef.current) {
        configForm.setFieldsValue(detailToConfigFormValues(fresh));
        initialConfigRef.current = normalizeConfigSnapshot(tr, configForm.getFieldsValue(true));
        uiBaselineRef.current = pickUiFields(configForm.getFieldsValue(true));
      }
      if (rotation) {
        setClientSecretReveal({
          plain: rotation.clientSecretPlain,
          prefix: rotation.clientSecretPrefix,
        });
      }
      message.success(tr("application.configSaved"));
    },
    onError: async (error) => {
      // 主配置已保存：刷新详情让用户看到已落库的部分，并明确剩余失败项。
      if (error instanceof ApplicationBindingsPartialError) {
        await queryClient.invalidateQueries({
          queryKey: queryKeys.applications.detail(applicationId),
        });
        message.warning(localizedError(error, tr, "common:error"));
        return;
      }
      // 422/400 的字段级错误（结构化 details 或后端英文消息中的字段名）直接落到
      // 对应表单项，让用户看到行内红字，而不是只有一条顶部 toast。数组项路径
      // （providerItems[0].providerId）由 resolver 归位到列表字段本身。
      let fieldErrorsApplied = false;
      if (isHttpError(error)) {
        fieldErrorsApplied = applyServerFieldErrors(
          configForm,
          error,
          buildServerFieldResolver<ApplicationConfigFormValues>(configForm),
          tr,
        );
      }
      // 兜底：resolver 未命中但后端明确点名 providerItems 时仍标红列表。
      const details = getErrorDetails(error);
      const field = typeof details?.field === "string" ? details.field : "";
      if (!fieldErrorsApplied && field.startsWith("providerItems")) {
        configForm.setFields([
          {
            name: "providerItems",
            errors: [serverErrorMessage(error, tr, "application.saveFailed")],
          },
        ]);
      }
      message.error(serverErrorMessage(error, tr, "common:operationFailed"));
    },
  });

  const deleteApplicationMutation = useMutation({
    mutationFn: async () => {
      if (!detail) throw new Error(tr("application.appNotExist"));
      await deleteApplication(detail.id, detail.organizationId);
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.detail(applicationId) }),
      ]);
      message.success(tr("application.appDeleted"));
      navigate("/applications");
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tr, "application.deleteAppFailed"));
    },
  });

  const exportMutation = useMutation({
    mutationFn: async () => {
      const blob = await exportApplicationConfig(applicationId, detail?.organizationId);
      downloadApplicationExport(blob, detail?.name ?? detail?.slug ?? applicationId);
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tr, "application.exportFailed"));
    },
  });

  // ─── Derived values ────────────────────────────────────────────
  const providerNameById = useMemo(
    () =>
      new Map(
        (providersQuery.data ?? [])
          .filter((item) => item.id)
          .map((item) => [item.id as string, item.name]),
      ),
    [providersQuery.data],
  );

  const boundProviders = useMemo(
    () =>
      (detail?.providerIds ?? []).map(
        (providerId) => providerNameById.get(providerId) ?? providerId,
      ),
    [detail?.providerIds, providerNameById],
  );

  const deleteDescription = formatDeleteDescription(
    tr,
    detail?.displayName || detail?.slug || applicationId,
    deleteSummaryQuery.data,
    {
      isLoading: deleteSummaryQuery.isLoading,
      hasError: deleteSummaryQuery.isError,
      onRetry: () => void deleteSummaryQuery.refetch(),
    },
  );

  const canConfirmDelete =
    !deleteSummaryQuery.isLoading && Boolean(deleteSummaryQuery.data || deleteSummaryQuery.isError);

  // 选项携带类型/分类/来源组织，提供商下拉才能渲染图标与来源标识；
  // value 仍是 id（缺省回退 name），保存与绑定派生逻辑不受影响。
  const providerOptions = useMemo(() => {
    const unified = (providersQuery.data ?? []).map((provider) => ({
      label: provider.name,
      value: provider.id ?? provider.name,
      type: provider.type ?? "",
      category: provider.category ?? "",
      orgName: provider.organizationId ? (orgNameMap.get(provider.organizationId) ?? "") : "",
    }));
    const faceId = (faceIdProvidersQuery.data?.items ?? []).map((provider) => ({
      label: provider.name,
      value: provider.id,
      type: provider.providerType ?? "",
      category: "FaceId",
      orgName: provider.organizationId ? (orgNameMap.get(provider.organizationId) ?? "") : "",
    }));
    const captcha = (captchaProvidersQuery.data?.items ?? []).map((provider) => ({
      label: provider.name,
      value: provider.id,
      type: provider.providerType ?? "",
      category: "Captcha",
      orgName: provider.organizationId ? (orgNameMap.get(provider.organizationId) ?? "") : "",
    }));
    const notification = (notificationProvidersQuery.data?.items ?? []).map((provider) => ({
      label: provider.name,
      value: provider.id,
      type: provider.providerType ?? "",
      category: "Notification",
      orgName: provider.organizationId ? (orgNameMap.get(provider.organizationId) ?? "") : "",
    }));
    return [...unified, ...faceId, ...captcha, ...notification];
  }, [
    providersQuery.data,
    faceIdProvidersQuery.data,
    captchaProvidersQuery.data,
    notificationProvidersQuery.data,
    orgNameMap,
  ]);

  const organizationOptions = useMemo(
    () =>
      (organizationsQuery.data ?? []).map((item) => ({
        label: item.displayName ? `${item.displayName} (${item.slug})` : item.slug,
        value: item.id,
      })),
    [organizationsQuery.data],
  );

  const existingApplicationSlugs = useMemo(
    () =>
      new Set(
        (existingApplicationsQuery.data ?? [])
          .filter((item) => item.id !== applicationId)
          .map((item) => item.name.trim().toLowerCase()),
      ),
    [existingApplicationsQuery.data, applicationId],
  );

  const clientSecretExtra = isNew
    ? tr("application.clientSecretExtraConfNew")
    : detail?.clientSecretSet
      ? tr("application.clientSecretMaskedHint", { masked: detail.clientSecretMasked })
      : tr("application.clientSecretExtraConfEdit");

  const rotateClientSecretMutation = useMutation({
    mutationFn: () => rotateApplicationClientSecret(applicationId, detail?.organizationId),
    onSuccess: async (rotation) => {
      setClientSecretReveal({
        plain: rotation.clientSecretPlain,
        prefix: rotation.clientSecretPrefix,
      });
      await queryClient.invalidateQueries({
        queryKey: queryKeys.applications.detail(applicationId),
      });
      message.success(tr("application.clientSecretRotated"));
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tr, "common:operationFailed"));
    },
  });

  const dismissClientSecretReveal = useCallback(() => {
    setClientSecretReveal(null);
    const next = afterRevealRef.current;
    afterRevealRef.current = null;
    next?.();
  }, []);

  // ─── Handlers ──────────────────────────────────────────────────
  // Values come from ResourceFormPage (already validated); reject so the shell keeps
  // the submit lock and renders the failure inside its Alert.
  const handleSave = useCallback(
    async (values: ApplicationConfigFormValues) => {
      // lazy 分区（SAML/Providers/定制/安全/反向代理/自定义 scope）未挂载时，
      // 壳的 validateFields() 只返回已注册字段；详情回填已把完整基线写入
      // store，这里合并整份 store，否则 payload 的 `?? ""`/`?? []` 会把未挂载
      // 分区的既有配置（formCss/samlMetadata/ipWhitelist 等）主动清空（FE-B-01，
      // 参照 ProviderDetailPage 的合并先例）。用户已挂载字段的编辑优先。
      const mergedValues = {
        ...configForm.getFieldsValue(true),
        ...values,
      } as ApplicationConfigFormValues;
      // Block submissions that carry material for a different method (e.g. a
      // secret while "none" is selected) before anything reaches the API.
      const requireClientJwks = isNew || detail?.tokenEndpointAuthMethod !== "private_key_jwt";
      const errors = validateApplicationConfigValues(tr, mergedValues, {
        requireClientJwks,
        // 编辑历史遗留配置（例如 none + 不可兑换授权类型）时，只有本次提交
        // 触碰了认证方式/授权类型才强制闭环组合。
        persistedAuthMethod: detail?.tokenEndpointAuthMethod,
        persistedGrantTypes: detail?.grantTypes,
        existingCertificateCount: clientCertificatesQuery.data?.length ?? 0,
      });
      if (errors.length > 0) {
        throw new Error(errors[0]);
      }
      const clientCertificates = await buildCertificatesForSave(tr, mergedValues);
      if (isNew) {
        await createApplicationMutation.mutateAsync({ values: mergedValues, clientCertificates });
        return;
      }
      await applicationConfigMutation.mutateAsync({ values: mergedValues, clientCertificates });
    },
    [
      clientCertificatesQuery.data,
      configForm,
      detail?.grantTypes,
      detail?.tokenEndpointAuthMethod,
      isNew,
      createApplicationMutation,
      applicationConfigMutation,
      tr,
    ],
  );

  const handleReset = useCallback(() => {
    configForm.resetFields();
    // Reset baseline too — resetFields moves Form back to the last setFieldsValue state,
    // which may be ahead of initialConfigRef (e.g., after a save). Keep them in sync.
    initialConfigRef.current = normalizeConfigSnapshot(tr, configForm.getFieldsValue(true));
    isDirtyRef.current = false;
    setIsDirty(false);
  }, [configForm, tr]);

  const resetUiConfigToSaved = useCallback(() => {
    if (!uiBaselineRef.current) return;
    configForm.setFieldsValue(uiBaselineRef.current as Partial<ApplicationConfigFormValues>);
    handleConfigFormChange();
  }, [configForm, handleConfigFormChange]);

  const resetUiConfigToDefaults = useCallback(() => {
    const defaults = createDefaultApplicationConfigFormValues(sessionOrganizationId);
    configForm.setFieldsValue(pickUiFields(defaults) as Partial<ApplicationConfigFormValues>);
    handleConfigFormChange();
  }, [configForm, handleConfigFormChange, sessionOrganizationId]);

  return {
    // Identifiers
    applicationId,
    isNew,
    sessionOrganizationId,
    // Forms
    configForm,
    // State
    isDirty,
    openDeleteConfirm,
    setOpenDeleteConfirm,
    // Queries
    detailQuery,
    providersQuery,
    organizationsQuery,
    existingApplicationsQuery,
    detail,
    // Mutations
    createApplicationMutation,
    applicationConfigMutation,
    deleteApplicationMutation,
    exportMutation,
    // Derived
    orgNameMap,
    providerOptions,
    organizationOptions,
    existingApplicationSlugs,
    clientSecretExtra,
    clientSecretSet: detail?.clientSecretSet ?? false,
    clientSecretMasked: detail?.clientSecretMasked ?? "",
    clientSecretReveal,
    isRotatingClientSecret: rotateClientSecretMutation.isPending,
    existingClientCertificates: clientCertificatesQuery.data ?? [],
    clientJwks: detail?.clientJwks ?? { kids: [], keys: [] },
    groups: groupsQuery.data ?? [],
    boundProviders,
    deleteDescription,
    canConfirmDelete,
    // Handlers
    handleConfigFormChange,
    initializedEntityKey,
    handleSave,
    handleReset,
    resetUiConfigToSaved,
    resetUiConfigToDefaults,
    handleRotateClientSecret: () => rotateClientSecretMutation.mutate(),
    dismissClientSecretReveal,
    // Helpers
    formatTokenEndpointAuthMethod,
  };
}

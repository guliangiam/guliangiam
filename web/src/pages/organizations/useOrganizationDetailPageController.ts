/**
 * Organization 详情页 Controller
 *
 * 从 OrganizationDetailPage.tsx 拆分而来，承担数据获取、form 初始化、
 * mutation 编排和错误处理。页面组件（View）只消费返回的状态和动作，
 * 不直接调用 useQuery/useMutation。
 */
import { useEffect, useRef } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  getOrganization,
  getDeploymentRegions,
  getOrganizationIdentityBySlug,
  deleteOrganization,
  saveOrganizationConfiguration,
} from "@/shared/api/iam/organization";
import { getApplicationOptions } from "@/shared/api/iam/application";
import { queryKeys } from "@/shared/api/query-keys";
import type { OrganizationFormValues } from "./tabs/types";
import {
  buildOrganizationFormValues,
  buildOrganizationUpdatePayload,
  buildOrganizationSettingsPayload,
  buildOrganizationThemePayload,
  buildOrganizationSecurityPolicyPayload,
  buildOrganizationFeatureFlags,
  ORGANIZATION_CREDENTIAL_FIELDS,
} from "./organization-payload";
import { localizedError } from "@/shared/lib/server-error";

export function useOrganizationDetailPageController() {
  const { message } = App.useApp();
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { organizationName = "" } = useParams();
  const [form] = Form.useForm<OrganizationFormValues>();

  // ── 数据获取 ──
  const identityQuery = useQuery({
    queryKey: queryKeys.organizations.identity(organizationName),
    queryFn: () => getOrganizationIdentityBySlug(organizationName),
    enabled: organizationName !== "",
  });

  const detailQuery = useQuery({
    queryKey: queryKeys.organizations.detail(identityQuery.data?.id ?? ""),
    queryFn: () => getOrganization(identityQuery.data!.id),
    enabled: identityQuery.data !== null && identityQuery.data !== undefined,
  });

  const regionOptionsQuery = useQuery({
    queryKey: [...queryKeys.organizations.all, "deployment-regions"],
    queryFn: () => getDeploymentRegions(),
  });

  const applicationsQuery = useQuery({
    queryKey: queryKeys.applications.options(identityQuery.data?.id ?? ""),
    queryFn: () => getApplicationOptions(identityQuery.data!.id),
    enabled: identityQuery.data !== null && identityQuery.data !== undefined,
  });

  // ── Form 初始化 ──
  // 仅在实体切换时灌值；后台 refetch 只同步 revision，不覆盖未保存输入。
  const syncedEntityRef = useRef("");
  useEffect(() => {
    if (!detailQuery.data) return;
    const entityKey = detailQuery.data.id;
    const values = buildOrganizationFormValues(detailQuery.data);
    // 凭据输入只属于当前编辑：详情刷新（含窗口聚焦重取）不得清掉操作者尚未
    // 提交的值。保存成功后的清空由 saveMutation.onSuccess 负责。
    for (const field of ORGANIZATION_CREDENTIAL_FIELDS) delete values[field];

    if (syncedEntityRef.current === entityKey) return;
    syncedEntityRef.current = entityKey;
    form.setFieldsValue(values);
  }, [detailQuery.data, form]);

  // ── 保存 mutation ──
  const saveMutation = useMutation({
    mutationFn: async (values: OrganizationFormValues) => {
      if (!detailQuery.data) throw new Error(tr("organization.detailNotLoaded"));
      const orgId = detailQuery.data.id;

      // antd Tabs 懒渲染：未访问过的 Tab 字段不会注册，validateFields 不返回它们。
      // 原子保存会覆盖全部子资源，因此这里从 form store 取全量值，
      // 避免把未访问 Tab 的配置按默认空值写回。
      const allValues = {
        ...values,
        ...(form.getFieldsValue(true) as OrganizationFormValues),
      };
      // MFA 闭环：开启强制 MFA 时必须有至少一个 Required 方法，且方法不可重复。
      const mfaItems = allValues.mfaItems ?? [];
      if (allValues.mfaRequired && !mfaItems.some((item) => item.rule === "Required")) {
        throw new Error(tr("organization.mfaMissingRequired"));
      }
      const mfaNames = mfaItems.map((item) => item.name).filter(Boolean);
      if (new Set(mfaNames).size !== mfaNames.length) {
        throw new Error(tr("organization.mfaDuplicateMethod"));
      }

      const featureFlags = buildOrganizationFeatureFlags(allValues);
      await saveOrganizationConfiguration(orgId, {
        organization: buildOrganizationUpdatePayload(allValues, detailQuery.data.revision),
        settings: buildOrganizationSettingsPayload(allValues, featureFlags),
        theme: buildOrganizationThemePayload(allValues),
        securityPolicy: buildOrganizationSecurityPolicyPayload(allValues),
      });
    },
    onMutate: async () => {
      if (!detailQuery.data) return { previousDetail: undefined };
      await queryClient.cancelQueries({
        queryKey: queryKeys.organizations.detail(detailQuery.data.id),
      });
      const previousDetail = queryClient.getQueryData(
        queryKeys.organizations.detail(detailQuery.data.id),
      );
      return { previousDetail };
    },
    onSuccess: async () => {
      // Credential inputs hold plaintext only for the current edit: clear them so
      // the secret does not linger in form state (they are never prefilled from
      // the read response, which serves only `*Set` / `*Masked`).
      form.resetFields([...ORGANIZATION_CREDENTIAL_FIELDS]);
      await queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
      message.success(t("updateSuccess"));
    },
    onError: (_error, _variables, context) => {
      if (detailQuery.data && context?.previousDetail !== undefined) {
        queryClient.setQueryData(
          queryKeys.organizations.detail(detailQuery.data.id),
          context.previousDetail,
        );
      }
      // ResourceFormPage renders the rejection inline (submitError); a toast here
      // would show the same failure twice. The declared onError also keeps the
      // global MutationCache fallback silent. Refetch so a partially applied
      // sub-resource write is visible instead of leaving the form on stale data.
      if (detailQuery.data) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.organizations.detail(detailQuery.data.id),
        });
      }
    },
  });

  // ── 删除 mutation ──
  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("organization.detailNotLoaded"));
      return deleteOrganization(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(t("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
      navigate("/organizations");
    },
    onError: (error) => {
      message.error(localizedError(error, t, "error"));
    },
  });

  const orgId = identityQuery.data?.id ?? "";

  return {
    // Form
    form,
    // Queries
    identityQuery,
    detailQuery,
    applicationsQuery,
    regionOptionsQuery,
    // Mutations
    saveMutation,
    deleteMutation,
    // Constants
    organizationName,
    orgId,
  };
}

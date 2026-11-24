import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import type { PermissionDetail } from "@/shared/api/iam/permission";
import {
  deletePermission,
  getPermission,
  getPermissionPolicy,
  getPermissions,
  updatePermission,
  updatePermissionPolicy,
} from "@/shared/api/iam/permission";
import { getApplicationOptions } from "@/shared/api/iam/application";
import { localizedError } from "@/shared/lib/server-error";

// 高级编辑器的结构化 policy 行
export type AdvancedPolicy = {
  ptype: string;
  subject: string;
  object: string;
  action: string;
};

export type PermissionFormValues = {
  displayName: string;
  description: string;
  model: string;
  adapter: string;
  resourceType: string;
  resources: string[];
  actions: string[];
  effect: string;
  domains: string[];
  users: string[];
  groups: string[];
  roles: string[];
  sorter: string;
  application: string;
  state: string;
  isEnabled: boolean;
};

export const RESOURCE_TYPE_OPTIONS = [
  { label: "Application", value: "Application" },
  { label: "TreeNode", value: "TreeNode" },
  { label: "Custom", value: "Custom" },
  { label: "API", value: "API" },
  { label: "User", value: "User" },
  { label: "Organization", value: "Organization" },
  { label: "Provider", value: "Provider" },
  { label: "Role", value: "Role" },
  { label: "Permission", value: "Permission" },
  { label: "Group", value: "Group" },
  { label: "Resource", value: "Resource" },
  { label: "Session", value: "Session" },
];

export const APPLICATION_ACTIONS = [
  { label: "Read", value: "Read" },
  { label: "Write", value: "Write" },
  { label: "Admin", value: "Admin" },
];

export const API_ACTIONS = [
  { label: "GET", value: "GET" },
  { label: "POST", value: "POST" },
  { label: "PUT", value: "PUT" },
  { label: "DELETE", value: "DELETE" },
  { label: "PATCH", value: "PATCH" },
];

export const EFFECT_OPTIONS = [
  { label: "Allow", value: "allow" },
  { label: "Deny", value: "deny" },
];

export const STATE_OPTIONS = [
  { label: "Approved", value: "Approved" },
  { label: "Pending", value: "Pending" },
];

/** Casbin GetPolicy() returns [subject, object, action] rows keyed by the "p" ptype. */
function rowToPolicy(row: string[]): AdvancedPolicy {
  const hasPType = row.length >= 4 && (row[0] === "p" || row[0] === "g");
  const fields = hasPType ? row.slice(1) : row;
  return {
    ptype: hasPType ? row[0] : "p",
    subject: fields[0] ?? "",
    object: fields[1] ?? "",
    action: fields[2] ?? "",
  };
}

/** The PUT DTO carries pType separately; rules are [subject, object, action]. */
function policyToRules(p: AdvancedPolicy): string[] {
  return [p.subject, p.object, p.action];
}

export function usePermissionDetailPageController() {
  const { organizationName, permissionName } = useParams<{
    organizationName: string;
    permissionName: string;
  }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PermissionFormValues>();
  const [advancedPolicies, setAdvancedPolicies] = useState<AdvancedPolicy[]>([]);
  const [syncedPolicies, setSyncedPolicies] = useState<AdvancedPolicy[] | undefined>(undefined);
  const [activeTab, setActiveTab] = useState("structured");

  // ─── Queries ────────────────────────────────────────────────────
  const listQuery = useQuery({
    queryKey: ["permissions", "auth"],
    queryFn: () => getPermissions(),
    enabled: !!organizationName && !!permissionName,
  });

  const matchedPermission = listQuery.data?.find(
    (p) => p.owner === organizationName && p.name === permissionName,
  );

  const detailQuery = useQuery({
    queryKey: ["permission", "auth", "detail", matchedPermission?.id],
    queryFn: async () => {
      if (!matchedPermission?.id) return null;
      return getPermission(matchedPermission.id);
    },
    enabled: !!matchedPermission?.id,
  });

  const appOptionsQuery = useQuery({
    queryKey: ["app-options"],
    queryFn: () => getApplicationOptions(),
    staleTime: 60_000,
  });

  // ✅ 正确的 policy 数据源：Permission 自己的 /policy 端点，
  //    不再依赖后端根本不返回的 enforcerId
  const policiesQuery = useQuery({
    queryKey: ["permission", "policy", matchedPermission?.id],
    queryFn: async () => {
      if (!matchedPermission?.id) return [];
      const rows = await getPermissionPolicy(matchedPermission.id);
      return rows.map((r) => rowToPolicy(r));
    },
    enabled: !!matchedPermission?.id && activeTab === "advanced",
  });

  // Seed editable policy list
  useEffect(() => {
    if (
      policiesQuery.data !== undefined &&
      JSON.stringify(policiesQuery.data) !== JSON.stringify(syncedPolicies)
    ) {
      // oxlint-disable-next-line react/set-state-in-effect -- 服务端数据同步到可编辑副本（比较后写入，避免级联渲染）
      setSyncedPolicies(policiesQuery.data);
      setAdvancedPolicies(policiesQuery.data);
    }
  }, [policiesQuery.data, syncedPolicies]);

  // ─── Form initialization ────────────────────────────────────────
  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data as PermissionDetail;
      form.setFieldsValue({
        displayName: detail.displayName,
        description: detail.description,
        model: detail.model ?? "",
        adapter: detail.adapter ?? "",
        resourceType: detail.resourceType,
        resources: detail.resources,
        actions: detail.actions,
        effect: detail.effect,
        domains: detail.domains || [],
        users: detail.users || [],
        groups: detail.groups || [],
        roles: detail.roles || [],
        sorter: detail.sorter ?? "",
        application: detail.application ?? "",
        state: detail.state || "Approved",
        isEnabled: detail.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  // ─── Mutations ─────────────────────────────────────────────────
  const updateMutation = useMutation({
    mutationFn: async (values: PermissionFormValues) => {
      if (!detailQuery.data) throw new Error("Permission data not loaded");
      return updatePermission(detailQuery.data.id, {
        displayName: values.displayName,
        description: values.description,
        resourceType: values.resourceType,
        resources: values.resources,
        actions: values.actions,
        effect: values.effect,
        domains: values.domains,
        users: values.users,
        groups: values.groups,
        roles: values.roles,
        model: values.model,
        adapter: values.adapter,
        sorter: values.sorter,
        application: values.application,
        state: values.state,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onMutate: async (values) => {
      if (!detailQuery.data) return { previousDetail: undefined };
      const detailKey = ["permission", "auth", "detail", detailQuery.data.id];
      await queryClient.cancelQueries({ queryKey: detailKey });
      const previousDetail = queryClient.getQueryData(detailKey);
      queryClient.setQueryData(detailKey, (old: Record<string, unknown> | undefined) => {
        if (!old) return old;
        return {
          ...old,
          displayName: values.displayName,
          description: values.description,
          resourceType: values.resourceType,
          resources: values.resources,
          actions: values.actions,
          effect: values.effect,
          domains: values.domains,
          users: values.users,
          groups: values.groups,
          roles: values.roles,
          model: values.model,
          adapter: values.adapter,
          sorter: values.sorter,
          application: values.application,
          state: values.state,
          isEnabled: values.isEnabled,
        };
      });
      return { previousDetail };
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["permissions"] });
      // 详情键是单数 ["permission",…]；乐观合并不带新 revision，必须失效
      // 详情让服务端值回流，否则同页第二次保存 409。
      void queryClient.invalidateQueries({ queryKey: ["permission"] });
    },
    onError: (error, _variables, context) => {
      if (detailQuery.data && context?.previousDetail !== undefined) {
        const detailKey = ["permission", "auth", "detail", detailQuery.data.id];
        queryClient.setQueryData(detailKey, context.previousDetail);
      }
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error("Permission data not loaded");
      return deletePermission(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["permissions"] });
      navigate("/permissions");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const savePoliciesMutation = useMutation({
    mutationFn: async () => {
      if (!matchedPermission?.id) throw new Error("Permission not loaded");
      // PUT /permissions/{id}/policy replaces the whole policy set; the backend
      // only accepts the bulk envelope { policies: [{ pType, rules }] }.
      await updatePermissionPolicy(matchedPermission.id, {
        policies: advancedPolicies.map((p) => ({
          pType: p.ptype,
          rules: policyToRules(p),
        })),
      });
    },
    onMutate: async () => {
      if (!matchedPermission?.id) return { previousPolicies: undefined };
      const policiesKey = ["permission", "policy", matchedPermission.id];
      await queryClient.cancelQueries({ queryKey: policiesKey });
      const previousPolicies = queryClient.getQueryData(policiesKey);
      queryClient.setQueryData(policiesKey, advancedPolicies);
      return { previousPolicies };
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      if (matchedPermission?.id) {
        void queryClient.invalidateQueries({
          queryKey: ["permission", "policy", matchedPermission.id],
        });
      }
    },
    onError: (error, _variables, context) => {
      if (matchedPermission?.id && context?.previousPolicies !== undefined) {
        queryClient.setQueryData(
          ["permission", "policy", matchedPermission.id],
          context.previousPolicies,
        );
      }
      message.error(localizedError(error, tc, "error"));
    },
  });

  // ─── Derived values ────────────────────────────────────────────
  const resourceType = Form.useWatch("resourceType", form) ?? "Application";
  const isApi = resourceType === "API";
  const actionOptions = isApi ? API_ACTIONS : APPLICATION_ACTIONS;

  // ─── Policy handlers ────────────────────────────────────────────
  const addPolicy = (policy: AdvancedPolicy) => setAdvancedPolicies([...advancedPolicies, policy]);
  const removePolicy = (index: number) => {
    const next = [...advancedPolicies];
    next.splice(index, 1);
    setAdvancedPolicies(next);
  };
  const updatePolicy = (index: number, policy: AdvancedPolicy) => {
    const next = [...advancedPolicies];
    next[index] = policy;
    setAdvancedPolicies(next);
  };

  return {
    // Params
    organizationName,
    permissionName,
    // Translations
    tc,
    tr,
    navigate,
    // Form / State
    form,
    activeTab,
    setActiveTab,
    advancedPolicies,
    setAdvancedPolicies,
    // Queries
    listQuery,
    matchedPermission,
    detailQuery,
    appOptionsQuery,
    policiesQuery,
    // Mutations
    updateMutation,
    deleteMutation,
    savePoliciesMutation,
    // Derived
    isApi,
    actionOptions,
    // Handlers
    addPolicy,
    removePolicy,
    updatePolicy,
  };
}

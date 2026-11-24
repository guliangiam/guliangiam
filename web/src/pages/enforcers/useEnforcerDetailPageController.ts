import { useCallback, useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  deleteIamEnforcer,
  getIamEnforcer,
  getEnforcerPolicies,
  addEnforcerPolicy,
  removeEnforcerPolicy,
  updateEnforcerPolicy,
  patchIamEnforcer,
  refreshIamEnforcerEngines,
  reloadIamEnforcerPolicy,
  getIamModels,
  getIamAdapters,
} from "@/shared/api/iam";
import type { Policy } from "@/shared/api/iam/enforcer";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

export type EnforcerFormValues = {
  owner: string;
  displayName: string;
  description: string;
  modelId: string;
  adapterId: string;
  isEnabled: boolean;
};

export function useEnforcerDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<EnforcerFormValues>();

  const detailQuery = useQuery({
    queryKey: queryKeys.enforcers.detail(id!),
    queryFn: () => getIamEnforcer(id!),
    enabled: !!id,
  });
  const policiesQuery = useQuery({
    queryKey: ["enforcer", "iam", "policies", id],
    queryFn: () => getEnforcerPolicies(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const d = detailQuery.data;
      form.setFieldsValue({
        owner: d.owner ?? "",
        displayName: d.displayName,
        description: d.description ?? "",
        modelId: d.modelId ?? "",
        adapterId: d.adapterId ?? "",
        isEnabled: d.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  // 后端 enforcers 的 model_id / adapter_id 是 models/adapters 记录的外键，
  // 不是 ACL/RBAC 之类的枚举名，因此选项要从对应资源加载。
  const modelsQuery = useQuery({
    queryKey: ["models", "iam", "enforcer-options"],
    queryFn: () => getIamModels({ pageSize: 100 }),
    enabled: !!detailQuery.data?.organizationId,
  });
  const adaptersQuery = useQuery({
    queryKey: ["adapters", "iam", "enforcer-options"],
    queryFn: () => getIamAdapters({ pageSize: 100 }),
    enabled: !!detailQuery.data?.organizationId,
  });
  const modelOptions = (modelsQuery.data ?? []).map((m) => ({
    label: m.displayName || m.name,
    value: m.id,
  }));
  const adapterOptions = (adaptersQuery.data ?? []).map((a) => ({
    label: a.displayName || a.name,
    value: a.id,
  }));

  const updateMutation = useMutation({
    mutationFn: async (values: EnforcerFormValues) => {
      if (!detailQuery.data) throw new Error(tr("enforcer.detailNotLoaded"));
      return patchIamEnforcer(detailQuery.data.id, {
        owner: values.owner,
        displayName: values.displayName,
        description: values.description,
        modelId: values.modelId || undefined,
        adapterId: values.adapterId || undefined,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["enforcers"] });
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("enforcer.detailNotLoaded"));
      return deleteIamEnforcer(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["enforcers"] });
      navigate("/enforcers");
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });

  const refreshMutation = useMutation({
    mutationFn: async () => {
      await refreshIamEnforcerEngines();
      if (id) await reloadIamEnforcerPolicy(id);
    },
    onSuccess: () => {
      message.success(tr("enforcer.refreshSuccess", "Policies refreshed successfully"));
      void queryClient.invalidateQueries({ queryKey: ["enforcer", "iam", "policies", id] });
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });

  const invalidatePolicies = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["enforcer", "iam", "policies", id] });
  }, [queryClient, id]);

  const addPolicyMutation = useMutation({
    mutationFn: (policy: Policy) => addEnforcerPolicy(id!, policy),
    onSuccess: () => {
      message.success(tr("enforcer.policyAdded", "Policy added successfully"));
      invalidatePolicies();
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });
  const removePolicyMutation = useMutation({
    mutationFn: (policy: Policy) => removeEnforcerPolicy(id!, policy),
    onSuccess: () => {
      message.success(tr("enforcer.policyRemoved", "Policy removed successfully"));
      invalidatePolicies();
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });
  const updatePolicyMutation = useMutation({
    mutationFn: ({ oldPolicy, newPolicy }: { oldPolicy: Policy; newPolicy: Policy }) =>
      updateEnforcerPolicy(id!, oldPolicy, newPolicy),
    onSuccess: () => {
      message.success(tr("enforcer.policyUpdated", "Policy updated successfully"));
      invalidatePolicies();
    },
    onError: (error) => message.error(localizedError(error, tc, "error")),
  });

  const handleAddPolicy = useCallback(
    (policy: Policy) => addPolicyMutation.mutate(policy),
    [addPolicyMutation],
  );
  const handleRemovePolicy = useCallback(
    (index: number) => {
      const policy = (policiesQuery.data ?? [])[index];
      if (policy) removePolicyMutation.mutate(policy);
    },
    [policiesQuery.data, removePolicyMutation],
  );
  const handleUpdatePolicy = useCallback(
    (index: number, policy: Policy) => {
      const oldPolicy = (policiesQuery.data ?? [])[index];
      if (oldPolicy) updatePolicyMutation.mutate({ oldPolicy, newPolicy: policy });
    },
    [policiesQuery.data, updatePolicyMutation],
  );

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    // Queries
    detailQuery,
    policiesQuery,
    modelOptions,
    adapterOptions,
    // Mutations
    updateMutation,
    deleteMutation,
    refreshMutation,
    addPolicyMutation,
    removePolicyMutation,
    updatePolicyMutation,
    // Handlers
    handleAddPolicy,
    handleRemovePolicy,
    handleUpdatePolicy,
  };
}

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import { deleteRole, getRole, getRoles, updateRole } from "@/shared/api/iam/role";
import type { RoleDetail } from "@/shared/api/iam/role";
import {
  getUserOptions,
  getGroups,
  getRoleOptions,
  getEnforcerPolicies,
  replaceEnforcerPolicies,
  reloadIamEnforcerPolicy,
} from "@/shared/api/iam";
import type { Policy } from "@/shared/api/iam/enforcer";
import { serverErrorMessage } from "@/shared/lib/server-error";

export type RoleFormValues = {
  displayName: string;
  description: string;
  users: string[];
  groups: string[];
  roleIds: string[];
  domains: string[];
  isEnabled: boolean;
};

export function useRoleDetailPageController() {
  const { organizationName, roleName } = useParams<{
    organizationName: string;
    roleName: string;
  }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RoleFormValues>();
  const [advancedPolicies, setAdvancedPolicies] = useState<Policy[]>([]);
  const [syncedEnforcerId, setSyncedEnforcerId] = useState<string | undefined>(undefined);

  const listQuery = useQuery({
    queryKey: ["roles", "auth"],
    queryFn: () => getRoles(),
    enabled: !!organizationName && !!roleName,
  });

  const matchedRole = listQuery.data?.find(
    (r) => r.owner === organizationName && r.name === roleName,
  );

  const detailQuery = useQuery({
    queryKey: ["role", "auth", "detail", matchedRole?.id],
    queryFn: async () => {
      if (!matchedRole?.id) return null;
      return getRole(matchedRole.id);
    },
    enabled: !!matchedRole?.id,
  });

  const enforcerId = (detailQuery.data as (RoleDetail & { enforcerId?: string }) | undefined)
    ?.enforcerId;
  const policiesQuery = useQuery({
    queryKey: ["role", "enforcer-policies", enforcerId],
    queryFn: () => getEnforcerPolicies(enforcerId!),
    enabled: !!enforcerId,
  });

  // 只在切换到另一个 enforcer 时同步策略；后台 refetch 返回新对象不得覆盖
  // 用户未保存的策略编辑（保存成功后 invalidate 触发的刷新同样不覆盖）。
  // 渲染期按 enforcerId 变化同步（React 推荐的「props 变化调整 state」范式），
  // 避免在 effect 内 setState 触发级联渲染。
  if (policiesQuery.data !== undefined && enforcerId !== syncedEnforcerId) {
    setSyncedEnforcerId(enforcerId);
    setAdvancedPolicies(policiesQuery.data);
  }

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        description: detailQuery.data.description,
        users: detailQuery.data.users || [],
        groups: detailQuery.data.groups || [],
        roleIds: detailQuery.data.roleIds || [],
        domains: detailQuery.data.domains || [],
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: RoleFormValues) => {
      if (!detailQuery.data) throw new Error(tr("role.detailNotLoaded"));
      return updateRole(detailQuery.data.id, {
        displayName: values.displayName,
        // PATCH 语义：缺省=保留原值，空串=400（description must not be blank）。
        // 创建时允许不填描述，编辑保存不能把空串强塞给后端。
        description: values.description?.trim() ? values.description : undefined,
        users: values.users,
        groupIds: values.groups,
        roleIds: values.roleIds,
        domains: values.domains,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["roles"] });
      // 详情键是单数 ["role","auth","detail",id]；不失效它 revision 会陈旧，
      // 同页第二次保存必 409。
      void queryClient.invalidateQueries({ queryKey: ["role"] });
    },
    onError: (error) => message.error(serverErrorMessage(error, tc, "error")),
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("role.detailNotLoaded"));
      return deleteRole(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["roles"] });
      navigate("/roles");
    },
    onError: (error) => message.error(serverErrorMessage(error, tc, "error")),
  });

  const savePoliciesMutation = useMutation({
    mutationFn: async () => {
      if (!enforcerId) throw new Error(tr("role.noEnforcer"));
      await replaceEnforcerPolicies(enforcerId, advancedPolicies);
      await reloadIamEnforcerPolicy(enforcerId);
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["role", "enforcer-policies", enforcerId] });
    },
    onError: (error) => message.error(serverErrorMessage(error, tc, "error")),
  });

  const users = Form.useWatch("users", form) ?? [];
  const groups = Form.useWatch("groups", form) ?? [];
  const roleIds = Form.useWatch("roleIds", form) ?? [];

  return {
    organizationName,
    roleName,
    matchedRole,
    tc,
    tr,
    navigate,
    form,
    advancedPolicies,
    setAdvancedPolicies,
    // Queries
    listQuery,
    detailQuery,
    policiesQuery,
    enforcerId,
    // Mutations
    updateMutation,
    deleteMutation,
    savePoliciesMutation,
    // Watches
    users,
    groups,
    roleIds,
    // helpers
    getUserOptions,
    getGroups,
    getRoleOptions,
  };
}

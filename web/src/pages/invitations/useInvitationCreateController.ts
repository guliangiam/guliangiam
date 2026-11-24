import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createInvitation } from "@/shared/api/iam/invitation";
import { queryKeys } from "@/shared/api/query-keys";
import { getGroupOptions } from "@/shared/api/iam/group";
import { getRoleOptions } from "@/shared/api/iam/role";
import { useSessionStore } from "@/features/session/session-store";

export type InvitationFormValues = {
  organizationId: string;
  emails: string;
  roleIds?: string[];
  groupIds?: string[];
  invitedBy?: string;
  validDays?: number;
};

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** 解析批量邮箱输入：按行/逗号/分号/空白拆分并去重。 */
export function parseBatchEmails(raw: string): string[] {
  const seen = new Set<string>();
  for (const token of raw.split(/[\n,;、\s]+/)) {
    const email = token.trim().toLowerCase();
    if (email) seen.add(email);
  }
  return Array.from(seen);
}

export function useInvitationCreateController() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const account = useSessionStore((state) => state.account);
  const [form] = Form.useForm<InvitationFormValues>();

  const organizationId = Form.useWatch("organizationId", form);

  // 邀请人默认当前登录用户（invited_by 是 uuid 列，必须用用户 ID 而非用户名）、有效期默认 7 天。
  useEffect(() => {
    form.setFieldsValue({
      invitedBy: form.getFieldValue("invitedBy") ?? account?.id ?? "",
      validDays: form.getFieldValue("validDays") ?? 7,
    });
  }, [account?.id, form]);

  const { data: roleOptions = [] } = useQuery({
    queryKey: queryKeys.roles.options(organizationId),
    queryFn: () => getRoleOptions(organizationId),
    enabled: Boolean(organizationId),
  });
  const { data: groupOptions = [] } = useQuery({
    queryKey: ["group-options", organizationId],
    queryFn: () => getGroupOptions(organizationId),
    enabled: Boolean(organizationId),
  });

  // 角色/组/邀请人都是组织内绑定：切组织必须重置，否则携旧组织 ID 提交必 422/404。
  const handleOrganizationChange = () => {
    form.setFieldsValue({
      roleIds: [],
      groupIds: [],
      invitedBy: account?.id ?? "",
    });
  };

  const mutation = useMutation({
    mutationFn: async (values: InvitationFormValues) => {
      const emails = parseBatchEmails(values.emails);
      let succeeded = 0;
      const failures: string[] = [];
      // 批量生成：前端循环 POST，不新增后端端点。
      for (const email of emails) {
        try {
          await createInvitation({
            organizationId: values.organizationId,
            email,
            roleIds: values.roleIds,
            groupIds: values.groupIds,
            invitedBy: values.invitedBy,
            validDays: values.validDays,
          });
          succeeded += 1;
        } catch {
          failures.push(email);
        }
      }
      return { succeeded, failures };
    },
    onSuccess: async ({ succeeded, failures }) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.invitations.all });
      if (failures.length > 0) {
        if (succeeded === 0) {
          message.error(tr("invitation.batchFailedEmails", { emails: failures.join(", ") }));
          return;
        }
        message.warning(
          tr("invitation.batchPartial", { success: succeeded, failed: failures.length }),
        );
      } else {
        message.success(
          succeeded > 1
            ? tr("invitation.batchSuccess", { count: succeeded })
            : tr("invitation.created"),
        );
      }
      navigate("/invitations");
    },
  });

  return {
    form,
    tc,
    tr,
    navigate,
    mutation,
    roleOptions,
    groupOptions,
    handleOrganizationChange,
    defaultInvitedBy: account?.id ?? "",
    defaultInvitedByLabel: account?.displayName || account?.name || "",
    EMAIL_PATTERN,
  };
}

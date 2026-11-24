import { App, Button, Card, Descriptions, Form, Popconfirm, Skeleton, Tag, Typography } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getInvitations,
  getInvitation,
  revokeInvitation,
  resendInvitation,
} from "@/shared/api/iam/invitation";
import { getGroupOptions } from "@/shared/api/iam/group";
import { getRoleOptions } from "@/shared/api/iam/role";
import { getOrganizations } from "@/shared/api/iam/organization";
import { queryKeys } from "@/shared/api/query-keys";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { NotFoundResult, QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

const { Text } = Typography;

export default function InvitationDetailPage() {
  return (
    <PermissionGuard action="invitation:read" resource="invitations">
      <InvitationDetailContent />
    </PermissionGuard>
  );
}

function InvitationDetailContent() {
  const { organizationName, invitationName } = useParams();
  const account = useSessionStore((state) => state.account);
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm();
  const { nameMap: orgNameMap } = useOrganizationNames();

  // 定向解析：按路由组织 + email 关键词服务端过滤，避免全量列表前 50 行截断
  // 导致深处的邀请详情不可达。键保持 ["invitations","auth",…] 前缀，
  // 列表页动作后的整族失效仍能刷新本页。
  const listQuery = useQuery({
    queryKey: ["invitations", "auth", "resolve", organizationName, invitationName],
    queryFn: () => getInvitations(organizationName, undefined, invitationName),
    enabled: account !== null && Boolean(organizationName) && Boolean(invitationName),
  });

  const matchedInvitation = listQuery.data?.find(
    (inv) => inv.organizationId === organizationName && inv.email === invitationName,
  );

  const detailQuery = useQuery({
    queryKey: ["invitation", "auth", "detail", matchedInvitation?.id],
    queryFn: () => getInvitation(matchedInvitation!.id),
    enabled: !!matchedInvitation?.id,
  });

  const invitation = detailQuery.data;
  const invitationOrgId = invitation?.organizationId;

  const { data: roleOptions = [] } = useQuery({
    queryKey: queryKeys.roles.options(invitationOrgId),
    queryFn: () => getRoleOptions(invitationOrgId),
    enabled: Boolean(invitationOrgId),
  });
  const { data: groupOptions = [] } = useQuery({
    queryKey: ["group-options", invitationOrgId],
    queryFn: () => getGroupOptions(invitationOrgId),
    enabled: Boolean(invitationOrgId),
  });
  const { data: organizations = [] } = useQuery({
    queryKey: [...queryKeys.organizations.all, "slug-map"],
    queryFn: () => getOrganizations(),
    staleTime: 60_000,
  });

  const revokeMutation = useMutation({
    mutationFn: () => revokeInvitation(matchedInvitation!.id),
    onSuccess: () => {
      message.success(tr("invitation.revoked"));
      queryClient.invalidateQueries({ queryKey: ["invitations", "auth"] });
      // 详情键是单数前缀 ["invitation","auth","detail",id]，必须一并失效才能刷新状态/时间。
      queryClient.invalidateQueries({ queryKey: ["invitation"] });
      navigate("/invitations");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "operationFailed"));
    },
  });

  const resendMutation = useMutation({
    mutationFn: () => resendInvitation(matchedInvitation!.id),
    onSuccess: () => {
      message.success(tr("invitation.resent"));
      queryClient.invalidateQueries({ queryKey: ["invitations", "auth"] });
      // 详情键是单数前缀 ["invitation","auth","detail",id]，必须一并失效才能刷新状态/时间。
      queryClient.invalidateQueries({ queryKey: ["invitation"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "operationFailed"));
    },
  });

  const handleCopySignupUrl = async () => {
    if (!invitation) return;
    // 接受链接指向邀请注册页：code=邀请码，organization=组织 slug（注册接口按 slug 定位组织）。
    const params = new URLSearchParams();
    if (invitation.code) params.set("code", invitation.code);
    const slug = organizations.find((org) => org.id === invitation.organizationId)?.slug;
    if (slug) params.set("organization", slug);
    const signupUrl = `${window.location.origin}/register/invitation?${params.toString()}`;
    try {
      await navigator.clipboard.writeText(signupUrl);
      message.success(tr("invitation.copiedSignupUrl"));
    } catch {
      message.error(tr("invitation.copyFailed", "Copy failed"));
    }
  };

  if (listQuery.isLoading || detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (listQuery.isError) {
    return (
      <QueryErrorResult
        error={listQuery.error}
        onRetry={() => listQuery.refetch()}
        notFoundTitle={tr("invitation.notFound")}
        notFoundDescription={tr("invitation.notFoundDesc", { id: invitationName })}
      />
    );
  }

  if (detailQuery.isError) {
    return (
      <QueryErrorResult
        error={detailQuery.error}
        onRetry={() => detailQuery.refetch()}
        notFoundTitle={tr("invitation.notFound")}
        notFoundDescription={tr("invitation.notFoundDesc", { id: invitationName })}
      />
    );
  }

  if (!invitation) {
    return (
      <NotFoundResult
        title={tr("invitation.notFound")}
        description={tr("invitation.notFoundDesc", { id: invitationName })}
      />
    );
  }

  return (
    <ResourceFormPage
      title={tr("invitation.detailTitle")}
      description={tr("invitation.detailDescription")}
      saving={false}
      form={form}
      onCancel={() => navigate("/invitations")}
      extra={
        <Button onClick={handleCopySignupUrl} disabled={!invitation}>
          {tr("invitation.copySignupUrl")}
        </Button>
      }
    >
      {invitation && (
        <FormSection title={tc("overview", "Overview")}>
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label={tc("email")}>{invitation.email}</Descriptions.Item>
            <Descriptions.Item label={tr("invitation.code")}>
              {invitation.code ? (
                <Text copyable={{ text: invitation.code }}>{invitation.code}</Text>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tc("organizationId")}>
              {orgNameMap.get(invitation.organizationId) || invitation.organizationId}
            </Descriptions.Item>
            <Descriptions.Item label={tc("statusLabel")}>
              <Tag
                color={
                  invitation.status === "pending"
                    ? "blue"
                    : invitation.status === "revoked"
                      ? "red"
                      : "green"
                }
              >
                {invitation.status}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.invitedBy")}>
              {invitation.invitedBy}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.roles")}>
              {invitation.roleIds.length > 0
                ? invitation.roleIds
                    .map((id) => roleOptions.find((role) => role.id === id)?.name ?? id)
                    .join(", ")
                : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.groups")}>
              {invitation.groupIds && invitation.groupIds.length > 0
                ? invitation.groupIds
                    .map((id) => groupOptions.find((group) => group.id === id)?.displayName ?? id)
                    .join(", ")
                : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tc("expiresAt")}>{invitation.expiresAt}</Descriptions.Item>
            <Descriptions.Item label={tr("invitation.acceptedAt")}>
              {invitation.acceptedAt ?? "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tc("createdAt")}>{invitation.createdAt}</Descriptions.Item>
            <Descriptions.Item label={tc("updatedAt")}>{invitation.updatedAt}</Descriptions.Item>
            <Descriptions.Item label={tc("actions")}>
              <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                <Popconfirm
                  title={tr("invitation.revokeConfirm")}
                  onConfirm={() => revokeMutation.mutate()}
                  okText={tc("confirm")}
                  cancelText={tc("cancel")}
                >
                  <Button
                    danger
                    disabled={invitation.status !== "pending"}
                    loading={revokeMutation.isPending}
                  >
                    {tr("invitation.revoke")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
              <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                <Popconfirm
                  title={tr("invitation.resendConfirm")}
                  onConfirm={() => resendMutation.mutate()}
                  okText={tc("confirm")}
                  cancelText={tc("cancel")}
                >
                  <Button
                    style={{ marginLeft: 8 }}
                    disabled={invitation.status !== "pending"}
                    loading={resendMutation.isPending}
                  >
                    {tr("invitation.resend")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
            </Descriptions.Item>
          </Descriptions>
        </FormSection>
      )}
    </ResourceFormPage>
  );
}

import { Button, Popconfirm, Table, Tag, App } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listUserConsents, revokeConsent, type UserConsentItem } from "@/shared/api/iam/user";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { localizedError } from "@/shared/lib/server-error";

interface ConsentsTabProps {
  userId: string;
  /** 用户所属组织：详情级端点要求可信租户，跨组织查看时必须显式携带。 */
  organizationId?: string;
}

export default function ConsentsTab({ userId, organizationId }: ConsentsTabProps) {
  const { t: tr } = useTranslation(["resource", "common"]);
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  const { data: consents = [], isLoading } = useQuery({
    queryKey: ["user-consents", userId],
    queryFn: () => listUserConsents(userId, organizationId),
    enabled: !!userId,
  });

  const revokeMutation = useMutation({
    mutationFn: (consentId: string) => revokeConsent(userId, consentId, organizationId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["user-consents", userId] });
      message.success(tr("user.consents.revokeSuccess", "Consent revoked"));
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "common:error"));
    },
  });

  const columns = [
    {
      title: tr("user.consents.application", "Application"),
      dataIndex: "applicationSlug",
      key: "applicationSlug",
      render: (v: string, record: UserConsentItem) => v || record.applicationId,
    },
    {
      title: tr("user.consents.scopes", "Scopes"),
      dataIndex: "grantedScopes",
      key: "grantedScopes",
      render: (v: string[]) => v?.map((s) => <Tag key={s}>{s}</Tag>),
    },
    {
      title: tr("user.consents.grantedAt", "Granted At"),
      dataIndex: "createdAt",
      key: "createdAt",
    },
    {
      title: tr("user.consents.status", "Status"),
      key: "status",
      render: (_: unknown, record: UserConsentItem) =>
        record.revokedAt ? (
          <Tag color="red">{tr("user.consents.revoked", "Revoked")}</Tag>
        ) : (
          <Tag color="green">{tr("user.consents.active", "Active")}</Tag>
        ),
    },
    {
      title: tr("user.consents.actions", "Actions"),
      key: "actions",
      render: (_: unknown, record: UserConsentItem) =>
        !record.revokedAt ? (
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Popconfirm
              title={tr("user.consents.revokeConfirm", "Revoke this consent?")}
              description={tr(
                "user.consents.revokeDescription",
                "The user will need to re-authorize this application.",
              )}
              onConfirm={() => revokeMutation.mutate(record.id)}
              okText={tr("user.consents.revoke", "Revoke")}
              cancelText={tr("common:action.cancel", "Cancel")}
              okButtonProps={{ danger: true, loading: revokeMutation.isPending }}
            >
              <Button size="small" danger>
                {tr("user.consents.revoke", "Revoke")}
              </Button>
            </Popconfirm>
          </PermissionGuard>
        ) : null,
    },
  ];

  return (
    <Table<UserConsentItem>
      dataSource={consents}
      columns={columns}
      loading={isLoading}
      pagination={false}
      size="small"
      rowKey="id"
    />
  );
}

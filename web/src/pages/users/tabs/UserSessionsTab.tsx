import { App, Button, Popconfirm, Space, Table, Tag } from "antd";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  listUserSessions,
  revokeUserSession,
  forceLogoutUserSessions,
  type ForceLogoutAdminResponse,
  type SessionItem,
} from "@/shared/api/iam/session";
import { formatDateTime } from "@/pages/sessions/device-utils";
import { queryKeys } from "@/shared/api/query-keys";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";

type UserSessionsTabProps = {
  userId: string;
  /** 用户所属组织：详情级端点要求可信租户，跨组织查看时必须显式携带。 */
  organizationId?: string;
};

export default function UserSessionsTab({ userId, organizationId }: UserSessionsTabProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  const sessionsQuery = useQuery({
    queryKey: ["user", userId, "sessions"],
    queryFn: async () => {
      const result = await listUserSessions(userId, { organizationId });
      return result.items;
    },
    enabled: !!userId,
    refetchInterval: 15000, // auto-refresh every 15s to catch revoked rows
  });

  const revokeMutation = useMutation({
    mutationFn: (sessionId: string) => revokeUserSession(userId, sessionId, organizationId),
    onSuccess: async () => {
      message.success(tr("sessions.revoked", "会话已撤销"));
      await queryClient.invalidateQueries({ queryKey: ["user", userId, "sessions"] });
      await queryClient.invalidateQueries({ queryKey: ["session"] });
    },
    onError: () => message.error(t("deleteFailed", "撤销失败")),
  });

  const forceLogoutMutation = useMutation<ForceLogoutAdminResponse, Error>({
    mutationFn: () => forceLogoutUserSessions(userId, organizationId),
    onSuccess: async (result) => {
      message.success(
        tr(
          "sessions.forceLoggedOut",
          "强制下线完成：revoked {{sessions}} sessions · {{refresh}} refresh · {{access}} access",
          {
            sessions: result.revokedCount,
            refresh: result.refreshCount,
            access: result.accessCount,
          },
        ),
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["user", userId, "sessions"] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.users.webauthnCredentials(userId) }),
        queryClient.invalidateQueries({ queryKey: ["session"] }),
      ]);
    },
    onError: () => message.error(t("operationFailed", "操作失败")),
  });

  return (
    <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
      <Space style={{ justifyContent: "space-between", width: "100%" }}>
        <span style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
          {tr("sessions.adminHint", "管理员视角 — 撤销操作级联影响所有设备")}
        </span>
        <PermissionGuard action="users:write" resource="users" fallback={null}>
          <Popconfirm
            title={tr("sessions.forceLogoutConfirm", "确定强制下线该用户全部会话？")}
            description={tr(
              "sessions.forceLogoutDesc",
              "将撤销所有活跃会话 + refresh tokens + access tokens。该操作不可撤销。",
            )}
            okButtonProps={{ danger: true }}
            okText={tr("sessions.forceLogout", "强制下线全部")}
            cancelText={t("cancel")}
            onConfirm={() => forceLogoutMutation.mutate()}
          >
            <Button danger loading={forceLogoutMutation.isPending}>
              {tr("sessions.forceLogout", "强制下线全部")}
            </Button>
          </Popconfirm>
        </PermissionGuard>
      </Space>

      <Table<SessionItem>
        size="small"
        rowKey="id"
        loading={sessionsQuery.isLoading}
        dataSource={sessionsQuery.data ?? []}
        pagination={false}
        columns={[
          {
            title: tr("sessions.device", "设备"),
            dataIndex: "userAgent",
            render: (v: string, record: SessionItem) =>
              `${record.deviceInfo ?? record.userAgent ?? "-"}`,
            ellipsis: true,
          },
          { title: tr("account.ipAddress", "IP"), dataIndex: "ipAddress", width: 140 },
          {
            title: tr("account.sessionState", "状态"),
            dataIndex: "sessionState",
            width: 100,
            render: (v: string) => <Tag color={v === "active" ? "green" : "red"}>{v}</Tag>,
          },
          {
            title: tr("account.lastSeen", "最后活跃"),
            dataIndex: "lastSeenAt",
            width: 170,
            render: (v: string) => (v ? formatDateTime(v) : "-"),
          },
          {
            title: t("actions", "操作"),
            width: 100,
            render: (_: unknown, record: SessionItem) =>
              record.sessionState === "active" ? (
                <PermissionGuard action="users:write" resource="users" fallback={null}>
                  <Popconfirm
                    title={tr("sessions.revokeConfirm", "确定撤销该会话？")}
                    okText={tr("account.revoke", "撤销")}
                    cancelText={t("cancel")}
                    onConfirm={() => revokeMutation.mutate(record.id)}
                  >
                    <Button size="small" danger loading={revokeMutation.isPending}>
                      {tr("account.revoke", "撤销")}
                    </Button>
                  </Popconfirm>
                </PermissionGuard>
              ) : (
                <Tag color="default">{tr("sessions.revoked", "已撤销")}</Tag>
              ),
          },
        ]}
      />
    </Space>
  );
}

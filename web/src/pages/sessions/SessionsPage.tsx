import { useState } from "react";
import { App, Button, Card, Descriptions, Popconfirm, Space, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { useSessionStore } from "@/features/session/session-store";
import {
  getCurrentSession,
  getSessionsPaginated,
  revokeOtherSessions,
  revokeSession,
  type SessionItem,
} from "@/shared/api/iam/session";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import {
  formatDateTime,
  formatDeviceMetadata,
  formatDeviceSummary,
  renderSessionDeviceSummary,
  renderSessionState,
} from "./device-utils";
import { localizedError } from "@/shared/lib/server-error";

type SessionParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
};

export default function SessionsPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const account = useSessionStore((state) => state.account);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");
  const { nameMap: orgNameMap } = useOrganizationNames();

  const list = useCrudList<SessionItem, SessionParams, string>({
    enabled: organization !== null,
    buildQueryKey: (p) =>
      ["sessions", "auth", p.organizationId, p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
    }),
    paginatedFn: getSessionsPaginated,
    deleteFn: revokeSession,
    invalidateKeysOnDelete: [
      ["sessions", "auth"],
      ["sessions", "auth", "current"],
    ],
    onDeleteSuccess: () => message.success(t("sessionRevoked")),
    onDeleteError: (error) => message.error(localizedError(error, t, "revokeSessionFailed")),
  });

  const currentSessionQuery = useQuery({
    queryKey: ["sessions", "auth", "current"],
    queryFn: () => getCurrentSession(),
    retry: false,
  });

  const revokeOthersMutation = useMutation({
    mutationFn: () => revokeOtherSessions(),
    onSuccess: async (result) => {
      message.success(
        t("revokedOtherSessions", {
          defaultValue: `Revoked ${result.revokedCount} active sessions on other devices`,
        }),
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["sessions", "auth"] }),
        queryClient.invalidateQueries({ queryKey: ["sessions", "auth", "current"] }),
      ]);
    },
    onError: (error) => {
      message.error(localizedError(error, t, "logoutOtherDevicesFailed"));
    },
  });

  const currentSession = currentSessionQuery.data;
  const currentSessionUnavailable = currentSessionQuery.isError;

  const toolbar = (
    <PermissionGuard action="session:write" resource="sessions" fallback={null}>
      <Popconfirm
        title={t("logoutOtherDevices")}
        description={t("logoutOtherDevicesDesc")}
        okText={t("confirm")}
        cancelText={t("cancel")}
        okButtonProps={{ danger: true, loading: revokeOthersMutation.isPending }}
        disabled={!currentSession || revokeOthersMutation.isPending}
        onConfirm={() => {
          revokeOthersMutation.mutate();
        }}
      >
        <Button type="primary" danger disabled={!currentSession}>
          {t("logoutOtherDevices")}
        </Button>
      </Popconfirm>
    </PermissionGuard>
  );

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />

      <Card className="gl-panel gl-resource-card" variant="borderless">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2}>{t("currentDeviceSession")}</Typography.Title>
            <Typography.Paragraph>{t("sessionPageDesc")}</Typography.Paragraph>
          </div>
          {toolbar}
        </div>

        {currentSession ? (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label={t("statusLabel")}>
              {renderSessionState(currentSession.sessionState, tr)}
            </Descriptions.Item>
            <Descriptions.Item label={t("user")}>{currentSession.username}</Descriptions.Item>
            <Descriptions.Item label={t("organizationId")}>
              {orgNameMap.get(currentSession.organizationId) || currentSession.organizationSlug}
            </Descriptions.Item>
            <Descriptions.Item label={t("application")}>
              {currentSession.applicationSlug}
            </Descriptions.Item>
            <Descriptions.Item label={t("deviceSummary")}>
              {formatDeviceSummary(currentSession)}
            </Descriptions.Item>
            <Descriptions.Item label={t("deviceMetadata")}>
              {formatDeviceMetadata(currentSession)}
            </Descriptions.Item>
            <Descriptions.Item label={t("lastActiveAt")}>
              {formatDateTime(currentSession.lastSeenAt)}
            </Descriptions.Item>
            <Descriptions.Item label={t("expiresAt")}>
              {formatDateTime(currentSession.expiresAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("session.ip")}>
              {currentSession.ipAddress || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("session.userAgent")}>
              {currentSession.userAgent || "-"}
            </Descriptions.Item>
          </Descriptions>
        ) : currentSessionQuery.isLoading ? (
          <Typography.Paragraph>{t("loading")}</Typography.Paragraph>
        ) : (
          <Typography.Paragraph type={currentSessionUnavailable ? "warning" : undefined}>
            {t("noSessionCookie")}
          </Typography.Paragraph>
        )}
      </Card>

      {renderCrudListView<SessionItem, SessionParams, string>(
        {
          // ── UI ──
          title: t("sessions"),
          description: t("sessionsListDesc"),
          searchPlaceholder: t("searchSessions"),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: { permission: "session:write", resource: "sessions" },
          toolbar: (
            <Space>
              {account?.isAdmin && (
                <OrganizationSelect
                  value={orgFilter}
                  onChange={(v) => {
                    setOrgFilter(v);
                    list.serverPagination.onChange(1, list.pagination.pageSize);
                  }}
                  style={{ width: 180 }}
                  allowClear
                />
              )}
              {toolbar}
            </Space>
          ),
          columns: [
            {
              title: t("statusLabel"),
              dataIndex: "sessionState",
              key: "sessionState",
              render: (value: string) => renderSessionState(value, tr),
            },
            {
              title: t("user"),
              dataIndex: "username",
              key: "username",
              fixed: "left",
              render: (value: string, record: SessionItem) =>
                value && record.organizationSlug ? (
                  <Link to={`/users/${record.organizationSlug}/${value}`}>{value}</Link>
                ) : (
                  value || "-"
                ),
            },
            {
              title: t("organizationId"),
              dataIndex: "organizationId",
              key: "organizationId",
              render: (v: string) => (v ? orgNameMap.get(v) || v : "-"),
            },
            { title: t("application"), dataIndex: "applicationSlug", key: "applicationSlug" },
            {
              title: t("device"),
              key: "deviceSummary",
              render: (_: unknown, record: SessionItem) => renderSessionDeviceSummary(record),
            },
            {
              title: t("lastActiveAt"),
              dataIndex: "lastSeenAt",
              key: "lastSeenAt",
              render: formatDateTime,
            },
            {
              title: t("expiresAt"),
              dataIndex: "expiresAt",
              key: "expiresAt",
              render: formatDateTime,
            },
            { title: tr("session.ip"), dataIndex: "ipAddress", key: "ipAddress" },
            {
              title: t("actions"),
              width: 160,
              key: "actions",
              fixed: "right",
              render: (_: unknown, record: SessionItem) => (
                <PermissionGuard action="session:write" resource="sessions" fallback={null}>
                  <Popconfirm
                    title={t("revokeSession")}
                    description={`${record.username} / ${record.applicationSlug}`}
                    okText={t("revoke")}
                    cancelText={t("cancel")}
                    okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                    onConfirm={() => {
                      list.deleteMutation.mutate(record.id);
                    }}
                  >
                    <Button type="link" danger size="small">
                      {t("revoke")}
                    </Button>
                  </Popconfirm>
                </PermissionGuard>
              ),
            },
          ],
        },
        list,
      )}
    </div>
  );
}

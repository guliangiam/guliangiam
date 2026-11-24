import { useState } from "react";
import { App, Button, Popconfirm, Space, Tag, Typography } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { useSessionStore } from "@/features/session/session-store";
import {
  deleteToken,
  getTokensPaginated,
  revokeToken,
  revokeTokensByOrganization,
  type TokenItem,
} from "@/shared/api/iam/token-extended";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

function formatDateTime(value?: string | null) {
  if (!value) {
    return "-";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }

  return parsed.toLocaleString();
}

function getTokenStatus(record: TokenItem) {
  if (record.revokedAt) {
    return "revoked";
  }

  const expiresAt = new Date(record.expiresAt);
  if (!Number.isNaN(expiresAt.getTime()) && expiresAt.getTime() <= Date.now()) {
    return "expired";
  }

  return "active";
}

function renderTokenStatus(status: string) {
  const colorMap: Record<string, string> = { active: "green", expired: "orange", revoked: "red" };
  return <Tag color={colorMap[status] ?? "default"}>{status}</Tag>;
}

type TokenParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
};

export default function TokensPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const account = useSessionStore((state) => state.account);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");
  const { nameMap: orgNameMap } = useOrganizationNames();

  const list = useCrudList<TokenItem, TokenParams, string>({
    enabled: organization !== null,
    buildQueryKey: (p) =>
      ["tokens", "auth", p.organizationId, p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
    }),
    paginatedFn: getTokensPaginated,
    deleteFn: deleteToken,
    invalidateKeysOnDelete: [["tokens", "auth"]],
    onDeleteSuccess: () => message.success(t("tokenDeleted")),
    onDeleteError: (error) => message.error(localizedError(error, t, "deleteTokenFailed")),
  });

  const revokeMutation = useMutation({
    mutationFn: async (tokenId: string) => {
      await revokeToken(tokenId);
    },
    onSuccess: async () => {
      message.success(t("tokenRevoked"));
      await queryClient.invalidateQueries({ queryKey: ["tokens", "auth"] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "revokeTokenFailed"));
    },
  });

  const revokeAllMutation = useMutation({
    mutationFn: async () => {
      if (!organization?.id) {
        throw new Error(t("orgContextUnavailable"));
      }

      return revokeTokensByOrganization(organization.id);
    },
    onSuccess: async (result) => {
      message.success(
        t("revokedTokens", { defaultValue: `Revoked ${result.revokedCount} tokens` }),
      );
      await queryClient.invalidateQueries({ queryKey: ["tokens", "auth"] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "batchRevokeFailed"));
    },
  });

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      {renderCrudListView<TokenItem, TokenParams, string>(
        {
          // ── UI ──
          title: t("tokens"),
          description: t("tokensListDesc"),
          searchPlaceholder: t("searchTokens"),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: { permission: "token:delete", resource: "tokens" },
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
              <PermissionGuard action="token:write" resource="tokens" fallback={null}>
                <Popconfirm
                  title={t("revokeAllTokens")}
                  description={
                    organization?.displayName ??
                    organization?.name ??
                    organization?.id ??
                    t("currentOrg")
                  }
                  okText={t("revokeAll")}
                  cancelText={t("cancel")}
                  okButtonProps={{ loading: revokeAllMutation.isPending }}
                  onConfirm={() => {
                    revokeAllMutation.mutate();
                  }}
                >
                  <Button disabled={!organization?.id}>{t("revokeAllOrgTokens")}</Button>
                </Popconfirm>
              </PermissionGuard>
              <PermissionGuard action="token:write" resource="tokens" fallback={null}>
                <Button type="primary" onClick={() => navigate("/tokens/new")}>
                  {t("create")}
                </Button>
              </PermissionGuard>
            </Space>
          ),
          columns: [
            {
              title: t("prefix"),
              dataIndex: "tokenPrefix",
              key: "tokenPrefix",
              width: 200,
              fixed: "left",
              render: (value: string | null | undefined, record: TokenItem) =>
                value ? (
                  record.id ? (
                    <Link to={`/tokens/${record.id}`}>
                      <Typography.Text copyable={{ text: value }}>{value}</Typography.Text>
                    </Link>
                  ) : (
                    <Typography.Text copyable={{ text: value }}>{value}</Typography.Text>
                  )
                ) : (
                  "-"
                ),
            },
            {
              title: t("statusLabel"),
              key: "status",
              render: (_: unknown, record: TokenItem) => renderTokenStatus(getTokenStatus(record)),
            },
            { title: t("user"), dataIndex: "username", key: "username" },
            {
              title: t("organizationId"),
              dataIndex: "organizationId",
              key: "organizationId",
              render: (v: string) => (v ? orgNameMap.get(v) || v : "-"),
            },
            { title: t("application"), dataIndex: "applicationSlug", key: "applicationSlug" },
            {
              title: tr("token.scopes"),
              key: "scopes",
              render: (_: unknown, record: TokenItem) => record.scopes.join(", ") || "-",
            },
            {
              title: tr("token.audience"),
              key: "audience",
              render: (_: unknown, record: TokenItem) => record.audience.join(", ") || "-",
            },
            {
              title: t("expiresAt"),
              dataIndex: "expiresAt",
              key: "expiresAt",
              render: (value: string) => formatDateTime(value),
            },
            {
              title: t("actions"),
              key: "actions",
              width: 160,
              fixed: "right",
              render: (_, record) => (
                <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
                  <LinkButton type="link" size="small" to={`/tokens/${record.id}`}>
                    {t("detail")}
                  </LinkButton>
                  {!record.revokedAt ? (
                    <PermissionGuard action="token:write" resource="tokens" fallback={null}>
                      <Popconfirm
                        title={t("revokeToken")}
                        description={record.id}
                        okText={t("revoke")}
                        cancelText={t("cancel")}
                        okButtonProps={{ loading: revokeMutation.isPending }}
                        onConfirm={() => {
                          revokeMutation.mutate(record.id);
                        }}
                      >
                        <Button type="link" size="small">
                          {t("revoke")}
                        </Button>
                      </Popconfirm>
                    </PermissionGuard>
                  ) : null}
                  <PermissionGuard action="token:delete" resource="tokens" fallback={null}>
                    <Popconfirm
                      title={t("deleteConfirm")}
                      description={record.id}
                      okText={t("delete")}
                      cancelText={t("cancel")}
                      okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                      onConfirm={() => {
                        list.deleteMutation.mutate(record.id);
                      }}
                    >
                      <Button type="link" danger size="small">
                        {t("delete")}
                      </Button>
                    </Popconfirm>
                  </PermissionGuard>
                </div>
              ),
            },
          ],
        },
        list,
      )}
    </div>
  );
}

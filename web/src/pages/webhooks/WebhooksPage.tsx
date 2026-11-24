import { App, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { deleteIamWebhook, type IamWebhookItem } from "@/shared/api/iam";
import { iamListWithPagination } from "@/shared/api/iam/client";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { WEBHOOKS } from "@/shared/api/paths";
import { LinkButton } from "@/shared/ui/LinkButton";

type WebhookRow = IamWebhookItem;

type WebhookParams = {
  page: number;
  pageSize: number;
  search?: string;
  isEnabled?: string;
  method?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function WebhooksPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);

  return renderCrudListPage<WebhookRow, WebhookParams, string>({
    // ── UI ──
    title: t("webhook.title"),
    description: t("webhook.description"),
    searchPlaceholder: t("webhook.searchPlaceholder"),
    rowKey: "id",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
      { dataIndex: "url", label: "URL" },
    ],
    filters: [
      {
        // ListWebhooksRequest reads `isEnabled` as a bool. Sending `status=enabled`
        // left the dropdown a silent no-op in both directions.
        key: "isEnabled",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "true" },
          { label: tc("disabled"), value: "false" },
        ],
      },
      {
        key: "method",
        label: t("webhook.method"),
        options: [
          { label: "POST", value: "POST" },
          { label: "PUT", value: "PUT" },
          { label: "GET", value: "GET" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="webhook:write" resource="webhooks" fallback={null}>
        <LinkButton type="primary" to="/webhooks/new">
          {t("webhook.newWebhook")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left" as const,
        render: (v: string, record: WebhookRow) => <Link to={`/webhooks/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        title: "URL",
        dataIndex: "url",
        key: "url",
        render: (v: string) =>
          v ? (
            <a href={v} target="_blank" rel="noopener noreferrer">
              {v}
            </a>
          ) : (
            "-"
          ),
      },
      {
        title: t("webhook.method"),
        dataIndex: "method",
        key: "method",
        render: (v: string) => <Tag>{v || "POST"}</Tag>,
      },
      {
        title: t("webhook.contentType"),
        dataIndex: "contentType",
        key: "contentType",
        width: 160,
        render: (v: string) => v || "-",
      },
      {
        title: t("webhook.isError"),
        dataIndex: "isError",
        key: "isError",
        width: 110,
        render: (v: boolean) => (v ? <Tag color="red">{t("webhook.hasError")}</Tag> : "-"),
      },
      {
        title: tc("statusLabel"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? <Tag color="green">{tc("enabled")}</Tag> : <Tag color="red">{tc("disabled")}</Tag>,
      },
      {
        title: tc("createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        width: 130,
        render: (v: string | undefined) => formatDate(v),
      },
      {
        title: tc("actions"),
        width: 160,
        key: "actions",
        fixed: "right" as const,
        render: (_, record) => (
          <LinkButton type="link" size="small" to={`/webhooks/${record.id}`}>
            {tc("edit")}
          </LinkButton>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "webhooks",
        "iam",
        organization?.id ?? "",
        p.page,
        p.pageSize,
        p.search,
        p.isEnabled,
        p.method,
      ] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      filters: {
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
        ...((filters.method as string | undefined) !== undefined ? { method: filters.method } : {}),
      },
    }),
    paginatedFn: async (p: WebhookParams) =>
      iamListWithPagination(WEBHOOKS, {
        page: p.page,
        pageSize: p.pageSize,
        search: p.search,
        organizationId: organization?.id ?? undefined,
        filters: {
          ...(p.isEnabled ? { isEnabled: p.isEnabled } : {}),
          ...(p.method ? { method: p.method } : {}),
        },
      }),
    deleteFn: deleteIamWebhook,
    invalidateKeysOnDelete: [["webhooks"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

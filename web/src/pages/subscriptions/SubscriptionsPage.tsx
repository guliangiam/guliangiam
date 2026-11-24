import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamSubscriptionsPaginated, deleteIamSubscription } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { SUBSCRIPTION_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

type SubscriptionRow = {
  id: string;
  name: string;
  displayName?: string;
  user?: string;
  plan?: string;
  startAt?: string | null;
  endAt?: string | null;
  state: string;
  createdAt?: string;
};

type SubscriptionParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  state?: string;
};

function formatDate(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function SubscriptionsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<SubscriptionRow, SubscriptionParams, string>({
    // ── UI ──
    title: t("subscription.title"),
    description: t("subscription.description"),
    searchPlaceholder: t("subscription.searchPlaceholder"),
    rowKey: "name",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        key: "state",
        label: tc("statusLabel"),
        options: stateOptions(SUBSCRIPTION_STATES, tc),
      },
    ],
    toolbar: (
      <PermissionGuard action="subscription:write" resource="subscriptions" fallback={null}>
        <LinkButton type="primary" to="/subscriptions/new">
          {t("subscription.newSubscription")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: true,
        render: (v: string, record) => <Link to={`/subscriptions/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      // SubscriptionListItem carries user/plan/startAt/endAt; period, group,
      // payment and autoRenew have no list field and stay rejected.
      { title: t("subscription.user"), dataIndex: "user", key: "user", width: 140 },
      { title: t("subscription.plan"), dataIndex: "plan", key: "plan", width: 150 },
      {
        title: t("subscription.startTime"),
        dataIndex: "startAt",
        key: "startAt",
        width: 120,
        render: (v: string | null | undefined) => formatDate(v),
      },
      {
        title: t("subscription.endTime"),
        dataIndex: "endAt",
        key: "endAt",
        width: 120,
        render: (v: string | null | undefined) => formatDate(v),
      },
      {
        title: tc("statusLabel"),
        dataIndex: "state",
        key: "state",
        render: (v: string) => <StateTag value={v} />,
      },
      {
        title: tc("createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        width: 120,
        render: (v: string | undefined) => formatDate(v),
      },
      {
        title: tc("actions"),
        key: "actions",
        width: 160,
        fixed: "right",
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/subscriptions/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              onConfirm={() =>
                deleteIamSubscription(record.id)
                  .then(() => message.success(tc("deleteSuccess")))
                  .catch((e) => message.error(localizedError(e, tc, "common:error")))
              }
            >
              <Button type="link" size="small" danger>
                {tc("delete")}
              </Button>
            </Popconfirm>
          </div>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "subscriptions",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.state,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.state as string | undefined) !== undefined ? { state: filters.state } : {}),
      },
    }),
    paginatedFn: getIamSubscriptionsPaginated,
    deleteFn: deleteIamSubscription,
    invalidateKeysOnDelete: [queryKeys.subscriptions.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

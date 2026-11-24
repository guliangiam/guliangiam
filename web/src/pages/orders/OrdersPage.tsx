import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamOrdersPaginated, deleteIamOrder } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { ORDER_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";

type OrderRow = {
  id: string;
  name: string;
  displayName?: string;
  user?: string;
  price?: number;
  currency?: string;
  state: string;
  createdAt?: string;
};

type OrderParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  state?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function OrdersPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<OrderRow, OrderParams, string>({
    // ── UI ──
    title: t("order.title"),
    description: t("order.description"),
    searchPlaceholder: t("order.searchPlaceholder"),
    rowKey: "name",
    // 删除 API 只认 UUID id；rowKey 是 name，批删必须映射。
    rowKeyToDeleteKey: (record) => record.id,
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
        options: stateOptions(ORDER_STATES, tc),
      },
    ],
    toolbar: (
      <PermissionGuard action="order:write" resource="orders" fallback={null}>
        <LinkButton type="primary" to="/orders/new">
          {t("order.newOrder")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: (list) => [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: true,
        render: (v: string, record: OrderRow) => <Link to={`/orders/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      // `type` was never on OrderListItem (nor on Casdoor's OrderListPage);
      // user/price are real list fields and match the reference columns.
      { title: t("order.user"), dataIndex: "user", key: "user", width: 140 },
      {
        title: t("order.price"),
        dataIndex: "price",
        key: "price",
        width: 130,
        render: (v: number | undefined, record: OrderRow) =>
          `${v ?? 0} ${record.currency ?? ""}`.trim(),
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
        fixed: "right",
        width: 160,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/orders/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
              onConfirm={() => list.deleteMutation.mutate(record.id)}
            >
              <PermissionGuard action="order:delete" resource="orders" fallback={null}>
                <Button type="link" size="small" danger>
                  {tc("delete")}
                </Button>
              </PermissionGuard>
            </Popconfirm>
          </div>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "orders",
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
    paginatedFn: getIamOrdersPaginated,
    deleteFn: deleteIamOrder,
    invalidateKeysOnDelete: [queryKeys.orders.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "order:delete", resource: "orders" },
  });
}

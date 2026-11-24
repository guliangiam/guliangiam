import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamPaymentsPaginated, deleteIamPayment } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { PAYMENT_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

type PaymentRow = {
  id: string;
  name: string;
  displayName?: string;
  provider?: string;
  type?: string;
  user?: string;
  price?: number;
  currency?: string;
  state: string;
  createdAt?: string;
};

type PaymentParams = {
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

export default function PaymentsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<PaymentRow, PaymentParams, string>({
    // ── UI ──
    title: t("payment.title"),
    description: t("payment.description"),
    searchPlaceholder: t("payment.searchPlaceholder"),
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
        options: stateOptions(PAYMENT_STATES, tc),
      },
    ],
    toolbar: (
      <PermissionGuard action="payment:write" resource="payments" fallback={null}>
        <LinkButton type="primary" to="/payments/new">
          {t("payment.newPayment")}
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
        render: (v: string, record: PaymentRow) => <Link to={`/payments/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      // PaymentListItem carries provider/user/price; Casdoor's PaymentListPage
      // shows the same refs. `products` stays detail-only.
      { title: t("payment.paymentMethod"), dataIndex: "provider", key: "provider", width: 150 },
      { title: tc("type"), dataIndex: "type", key: "type", width: 120 },
      { title: t("payment.user"), dataIndex: "user", key: "user", width: 140 },
      {
        title: t("payment.price"),
        dataIndex: "price",
        key: "price",
        width: 130,
        render: (v: number | undefined, record: PaymentRow) =>
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
            <LinkButton type="link" size="small" to={`/payments/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              onConfirm={() =>
                deleteIamPayment(record.id)
                  .then(() => message.success(tc("deleteSuccess")))
                  .catch((e) => message.error(localizedError(e, tc, "common:error")))
              }
            >
              <PermissionGuard action="payment:delete" resource="payments" fallback={null}>
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
        "payments",
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
    paginatedFn: getIamPaymentsPaginated,
    deleteFn: deleteIamPayment,
    invalidateKeysOnDelete: [queryKeys.payments.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "payment:delete", resource: "payments" },
  });
}

import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamTransactionsPaginated, deleteIamTransaction } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { TRANSACTION_STATUSES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";

type TransactionRow = {
  id: string;
  transactionType?: string;
  userId?: string;
  amount?: number;
  currency?: string;
  providerType?: string;
  status: string;
  createdAt?: string;
};

type TransactionParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  status?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function TransactionsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<TransactionRow, TransactionParams, string>({
    // ── UI ──
    title: t("transaction.title"),
    description: t("transaction.description"),
    searchPlaceholder: t("transaction.searchPlaceholder"),
    rowKey: "id",
    searchableColumns: [{ dataIndex: "transactionType", label: t("transaction.transactionType") }],
    filters: [
      {
        key: "status",
        label: tc("statusLabel"),
        options: stateOptions(TRANSACTION_STATUSES, tc),
      },
    ],
    toolbar: (
      <PermissionGuard action="transaction:write" resource="transactions" fallback={null}>
        <LinkButton type="primary" to="/transactions/new">
          {t("transaction.newTransaction")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: (list) => [
      {
        title: tc("id"),
        dataIndex: "id",
        key: "id",
        width: 200,
        fixed: "left",
        render: (v: string, record) => <Link to={`/transactions/${record.id}`}>{v}</Link>,
      },
      {
        title: t("transaction.transactionType"),
        dataIndex: "transactionType",
        key: "transactionType",
      },
      // TransactionListItem carries userId/providerType; application and tag
      // have no field on either DTO and stay rejected.
      { title: t("transaction.userId"), dataIndex: "userId", key: "userId", width: 140 },
      {
        title: t("transaction.amount"),
        key: "amount",
        render: (_, record) => `${record.amount ?? 0} ${record.currency ?? ""}`.trim(),
      },
      {
        title: t("transaction.paymentMethod"),
        dataIndex: "providerType",
        key: "providerType",
        width: 140,
      },
      {
        title: tc("statusLabel"),
        dataIndex: "status",
        key: "status",
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
            <LinkButton type="link" size="small" to={`/transactions/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
              onConfirm={() => list.deleteMutation.mutate(record.id)}
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
        "transactions",
        "iam",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.status,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.status as string | undefined) !== undefined ? { status: filters.status } : {}),
      },
    }),
    paginatedFn: getIamTransactionsPaginated,
    deleteFn: deleteIamTransaction,
    invalidateKeysOnDelete: [["transactions"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

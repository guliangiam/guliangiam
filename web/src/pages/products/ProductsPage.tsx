import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamProductsPaginated, deleteIamProduct } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { PRODUCT_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";

type ProductRow = {
  id: string;
  name: string;
  displayName?: string;
  image?: string;
  currency?: string;
  price?: number;
  state: string;
  createdAt?: string;
};

type ProductParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  state?: string;
  isRecharge?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function ProductsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<ProductRow, ProductParams, string>({
    // ── UI ──
    title: t("product.title"),
    description: t("product.description"),
    searchPlaceholder: t("product.searchPlaceholder"),
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
        options: stateOptions(PRODUCT_STATES, tc),
      },
      {
        // ProductListItem has no `isRecharge`, so it stays a server-side
        // filter (ListProductsQuery carries it) instead of a list column.
        key: "isRecharge",
        label: t("product.isRecharge"),
        options: [
          { label: tc("yes"), value: "true" },
          { label: tc("no"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="product:write" resource="products" fallback={null}>
        <LinkButton type="primary" to="/products/new">
          {t("product.newProduct")}
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
        sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
        render: (v: string, record: ProductRow) => <Link to={`/products/${record.id}`}>{v}</Link>,
      },
      {
        title: tc("displayName"),
        dataIndex: "displayName",
        key: "displayName",
      },
      {
        title: t("product.image"),
        dataIndex: "image",
        key: "image",
        width: 100,
        render: (v: string | undefined, record: ProductRow) =>
          v ? (
            <img
              src={v}
              alt={record.displayName || record.name}
              style={{ height: 32, maxWidth: 84, objectFit: "contain" }}
            />
          ) : (
            "-"
          ),
      },
      {
        title: t("product.currency"),
        dataIndex: "currency",
        key: "currency",
      },
      {
        title: t("product.price"),
        dataIndex: "price",
        key: "price",
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
            <LinkButton type="link" size="small" to={`/products/${record.id}`}>
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
        "products",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.state,
        p.isRecharge,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.state as string | undefined) !== undefined ? { state: filters.state } : {}),
        ...((filters.isRecharge as string | undefined) !== undefined
          ? { isRecharge: filters.isRecharge }
          : {}),
      },
    }),
    paginatedFn: getIamProductsPaginated,
    deleteFn: deleteIamProduct,
    invalidateKeysOnDelete: [queryKeys.products.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

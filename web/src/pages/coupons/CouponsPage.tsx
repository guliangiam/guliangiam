import { App, Button, Popconfirm } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamCouponsPaginated, deleteIamCoupon } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { COUPON_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";

type CouponRow = {
  id: string;
  name: string;
  displayName?: string;
  code?: string;
  discountType?: string;
  discount?: number;
  state: string;
  createdAt?: string;
};

type CouponParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  state?: string;
  discountType?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function CouponsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<CouponRow, CouponParams, string>({
    // ── UI ──
    title: t("coupon.title"),
    description: t("coupon.description"),
    searchPlaceholder: t("coupon.searchPlaceholder"),
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
        options: stateOptions(COUPON_STATES, tc),
      },
      {
        key: "discountType",
        label: t("coupon.discountType"),
        options: [
          { label: t("coupon.percent"), value: "percentage" },
          { label: t("coupon.fixed"), value: "fixed" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="coupon:write" resource="coupons" fallback={null}>
        <LinkButton type="primary" to="/coupons/new">
          {t("coupon.newCoupon")}
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
        render: (v: string, record: CouponRow) => <Link to={`/coupons/${record.id}`}>{v}</Link>,
      },
      {
        title: tc("displayName"),
        dataIndex: "displayName",
        key: "displayName",
      },
      {
        title: t("coupon.code"),
        dataIndex: "code",
        key: "code",
      },
      {
        title: t("coupon.discountType"),
        dataIndex: "discountType",
        key: "discountType",
      },
      {
        // CouponListItem has discountType + discount but not currency (detail
        // only), so percentage coupons render `%` and fixed ones stay numeric.
        title: t("coupon.discount"),
        dataIndex: "discount",
        key: "discount",
        render: (v: number | undefined, record: CouponRow) =>
          record.discountType === "percentage" ? `${v ?? 0}%` : `${v ?? 0}`,
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
            <LinkButton type="link" size="small" to={`/coupons/${record.id}`}>
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
        "coupons",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.state,
        p.discountType,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.state as string | undefined) !== undefined ? { state: filters.state } : {}),
        ...((filters.discountType as string | undefined) !== undefined
          ? { discountType: filters.discountType }
          : {}),
      },
    }),
    paginatedFn: getIamCouponsPaginated,
    deleteFn: deleteIamCoupon,
    invalidateKeysOnDelete: [queryKeys.coupons.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

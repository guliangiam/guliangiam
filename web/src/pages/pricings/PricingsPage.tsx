import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamPricingsPaginated, deleteIamPricing } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

type PricingRow = {
  id: string;
  name: string;
  displayName?: string;
  isEnabled: boolean;
  trialDuration: number;
  createdAt?: string;
};

type PricingParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  isEnabled?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function PricingsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<PricingRow, PricingParams, string>({
    // ── UI ──
    title: t("pricing.title"),
    description: t("pricing.description"),
    searchPlaceholder: t("pricing.searchPlaceholder"),
    rowKey: "name",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        // `application` is detail-only and the list handler does not read an
        // application filter; `isEnabled` is the filter PricingListItem backs.
        key: "isEnabled",
        label: tc("enabled"),
        options: [
          { label: tc("yes"), value: "true" },
          { label: tc("no"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="pricing:write" resource="pricings" fallback={null}>
        <LinkButton type="primary" to="/pricings/new">
          {t("pricing.newPricing")}
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
        render: (v: string, record) => <Link to={`/pricings/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        title: tc("enabled"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? <Tag color="green">{tc("yes")}</Tag> : <Tag color="default">{tc("no")}</Tag>,
      },
      {
        title: t("pricing.trialDurationDays"),
        dataIndex: "trialDuration",
        key: "trialDuration",
        width: 150,
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
            <LinkButton type="link" size="small" to={`/pricings/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              onConfirm={() =>
                deleteIamPricing(record.id)
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
        "pricings",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.isEnabled,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
      },
    }),
    paginatedFn: getIamPricingsPaginated,
    deleteFn: deleteIamPricing,
    invalidateKeysOnDelete: [["pricings"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

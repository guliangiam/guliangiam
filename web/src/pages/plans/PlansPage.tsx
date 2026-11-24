import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamPlansPaginated, deleteIamPlan } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { PLAN_STATES, stateOptions } from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";

type PlanRow = {
  id: string;
  name: string;
  displayName?: string;
  period?: string;
  price?: number;
  currency?: string;
  state: string;
  createdAt?: string;
};

type PlanParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  state?: string;
  isExclusive?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function PlansPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<PlanRow, PlanParams, string>({
    // ── UI ──
    title: t("plan.title"),
    description: t("plan.description"),
    searchPlaceholder: t("plan.searchPlaceholder"),
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
        options: stateOptions(PLAN_STATES, tc),
      },
      {
        // PlanListItem has no `isExclusive` (detail-only), so the flag stays a
        // server-side filter (ListPlansQuery carries it) instead of a column.
        key: "isExclusive",
        label: t("plan.isExclusive"),
        options: [
          { label: tc("yes"), value: "true" },
          { label: tc("no"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="plan:write" resource="plans" fallback={null}>
        <LinkButton type="primary" to="/plans/new">
          {t("plan.newPlan")}
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
        render: (v: string, record) => <Link to={`/plans/${record.id}`}>{v}</Link>,
      },
      {
        title: tc("displayName"),
        dataIndex: "displayName",
        key: "displayName",
      },
      {
        title: t("plan.period"),
        dataIndex: "period",
        key: "period",
        render: (v: string) => <Tag color="blue">{v}</Tag>,
      },
      {
        title: t("plan.price"),
        dataIndex: "price",
        key: "price",
        width: 130,
        render: (v: number | undefined, record: PlanRow) =>
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
        width: 160,
        fixed: "right",
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/plans/${record.id}`}>
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
        "plans",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.state,
        p.isExclusive,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.state as string | undefined) !== undefined ? { state: filters.state } : {}),
        ...((filters.isExclusive as string | undefined) !== undefined
          ? { isExclusive: filters.isExclusive }
          : {}),
      },
    }),
    paginatedFn: getIamPlansPaginated,
    deleteFn: deleteIamPlan,
    invalidateKeysOnDelete: [queryKeys.plans.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}

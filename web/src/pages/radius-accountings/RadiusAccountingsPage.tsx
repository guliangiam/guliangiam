import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamRadiusAccountingsPaginated } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";

type RadiusAccountingRow = {
  id: string;
  name: string;
  username: string;
  acctSessionId?: string;
  acctStartTime?: string;
  createdAt?: string;
};

type RadiusAccountingParams = {
  page: number;
  pageSize: number;
  search?: string;
};

function formatDate(v?: string) {
  return v ? new Date(v).toLocaleString() : "-";
}

export default function RadiusAccountingsPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<RadiusAccountingRow, RadiusAccountingParams, string>({
    // ── UI ──
    title: tr("radius.accountingsTitle", "RADIUS Accounting"),
    description: tr(
      "radius.accountingsDesc",
      "View RADIUS accounting records, track network access sessions.",
    ),
    searchPlaceholder: tr("radius.accountingsSearch", "Filter by name or username"),
    rowKey: "id",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "username", label: tr("radius.username", "Username") },
    ],
    columns: [
      {
        title: tr("radius.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
      },
      { title: tr("radius.username", "Username"), dataIndex: "username", key: "username" },
      {
        title: tr("radius.sessionId", "Session ID"),
        dataIndex: "acctSessionId",
        key: "acctSessionId",
      },
      {
        title: tr("radius.sessionStart", "Session Start"),
        dataIndex: "acctStartTime",
        key: "acctStartTime",
        render: (v: string) => formatDate(v),
      },
      {
        title: tr("radius.createdAt", "Created At"),
        dataIndex: "createdAt",
        key: "createdAt",
        render: (v: string) => formatDate(v),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      ["radius-accountings", "paginated", p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: getIamRadiusAccountingsPaginated,
  });
}

import { Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getGatewaySitesPaginated } from "@/shared/api/iam/gateway";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type SiteRow = {
  id: string;
  name: string;
  host: string;
  status: string;
  sslMode?: string;
  lastHealthStatus?: string;
  organizationId: string;
};

type SiteParams = {
  page: number;
  pageSize: number;
  search?: string;
};

export default function SitesPage() {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<SiteRow, SiteParams, string>({
    // ── UI ──
    title: tr("gateway.sites.title", "Site Management"),
    description: tr(
      "gateway.sites.description",
      "Manage site configuration, define domains, hosts and runtime status.",
    ),
    searchPlaceholder: tr("gateway.sites.searchPlaceholder", "Filter by name"),
    rowKey: "id",
    toolbar: (
      <PermissionGuard action="site:write" resource="sites" fallback={null}>
        <LinkButton type="primary" to="/sites/new">
          {tr("gateway.sites.newSite", "New Site")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: tr("gateway.sites.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        render: (v: string, record: SiteRow) => (
          <Link to={`/sites/${record.organizationId}/${record.name}`}>{v}</Link>
        ),
      },
      { title: tr("gateway.sites.host", "Host"), dataIndex: "host", key: "host" },
      {
        title: tc("sslMode"),
        dataIndex: "sslMode",
        key: "sslMode",
        width: 130,
        render: (v?: string) => v || "-",
      },
      {
        title: tr("site.health", "Health"),
        dataIndex: "lastHealthStatus",
        key: "lastHealthStatus",
        width: 120,
        render: (v?: string) => {
          if (!v) return "-";
          const color = v === "healthy" ? "green" : v === "unhealthy" ? "red" : "default";
          return <Tag color={color}>{v}</Tag>;
        },
      },
      {
        title: tr("gateway.sites.status", "Status"),
        dataIndex: "status",
        key: "status",
        render: (v: string) =>
          v === "active" ? (
            <Tag color="green">{tr("gateway.sites.active", "Active")}</Tag>
          ) : (
            <Tag color="default">{v}</Tag>
          ),
      },
      {
        title: tr("gateway.sites.actions", "Actions"),
        width: 160,
        key: "actions",
        fixed: "right",
        render: (_, record) => (
          <LinkButton
            type="link"
            size="small"
            to={`/sites/${record.organizationId}/${record.name}`}
          >
            {tr("gateway.sites.edit", "Edit")}
          </LinkButton>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) => ["gateway-sites", "paginated", p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: getGatewaySitesPaginated,
  });
}

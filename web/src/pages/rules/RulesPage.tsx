import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamRulesPaginated, deleteIamRule } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

const scopeColorMap: Record<string, string> = {
  org: "blue",
  application: "green",
};

const ruleTypeColorMap: Record<string, string> = {
  IP: "volcano",
  UA: "orange",
  Rate: "gold",
  WAF: "red",
};

const actionColorMap: Record<string, string> = {
  Allow: "green",
  Block: "red",
  Captcha: "blue",
};

type RuleRow = {
  id: string;
  name: string;
  displayName?: string;
  scope?: string;
  ruleType?: string;
  action?: string;
  priority?: number;
  isEnabled: boolean;
};

type RuleParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  scope?: string;
  ruleType?: string;
  isEnabled?: string;
};

export default function RulesPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<RuleRow, RuleParams, string>({
    // ── UI ──
    title: tr("rule.title", "Rules / WAF"),
    description: tr(
      "rule.description",
      "Manage access rules and WAF policies, control IP, UA, rate limiting and security protection.",
    ),
    searchPlaceholder: tr("rule.searchPlaceholder", "Filter by rule name"),
    rowKey: (record: RuleRow) => String(record.id ?? ""),
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        key: "scope",
        label: tr("rule.scope", "Scope"),
        options: [
          { label: "Organization", value: "org" },
          { label: "Application", value: "application" },
        ],
      },
      {
        key: "ruleType",
        label: tr("rule.ruleType", "Rule Type"),
        options: [
          { label: "IP", value: "IP" },
          { label: "UA", value: "UA" },
          { label: "Rate", value: "Rate" },
          { label: "WAF", value: "WAF" },
        ],
      },
      {
        key: "isEnabled",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "true" },
          { label: tc("disabled"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="rules:write" resource="rules" fallback={null}>
        <LinkButton type="primary" to="/rules/new">
          {tr("rule.newRule", "New Rule")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: tr("rule.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
        render: (v: string, record) => <Link to={`/rules/${record.id}`}>{v}</Link>,
      },
      {
        title: tr("rule.scope", "Scope"),
        dataIndex: "scope",
        key: "scope",
        render: (v: string) => <Tag color={scopeColorMap[v] ?? "default"}>{v}</Tag>,
      },
      {
        title: tr("rule.ruleType", "Rule Type"),
        dataIndex: "ruleType",
        key: "ruleType",
        render: (v: string) => <Tag color={ruleTypeColorMap[v] ?? "default"}>{v}</Tag>,
      },
      {
        title: tr("rule.action", "Action"),
        dataIndex: "action",
        key: "action",
        render: (v: string) => <Tag color={actionColorMap[v] ?? "default"}>{v}</Tag>,
      },
      {
        title: tr("rule.priority", "Priority"),
        dataIndex: "priority",
        key: "priority",
        width: 80,
      },
      {
        title: tr("rule.status", "Status"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? (
            <Tag color="green">{tr("rule.enabled", "Enabled")}</Tag>
          ) : (
            <Tag color="red">{tr("rule.disabled", "Disabled")}</Tag>
          ),
      },
      {
        title: tr("rule.actions", "Actions"),
        key: "actions",
        fixed: "right",
        width: 160,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/rules/${record.id}`}>
              {tr("rule.edit", "Edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              onConfirm={() =>
                deleteIamRule(record.id)
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
        "rules",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.scope,
        p.ruleType,
        p.isEnabled,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.scope as string | undefined) !== undefined ? { scope: filters.scope } : {}),
        ...((filters.ruleType as string | undefined) !== undefined
          ? { ruleType: filters.ruleType }
          : {}),
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
      },
    }),
    paginatedFn: getIamRulesPaginated,
    deleteFn: deleteIamRule,
    invalidateKeysOnDelete: [["rules"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "rules:delete", resource: "rules" },
  });
}

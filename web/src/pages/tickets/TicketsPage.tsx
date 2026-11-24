import { App, Button, Popconfirm, Tag } from "antd";
import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamTicketsPaginated, deleteIamTicket } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import {
  TICKET_STATUSES,
  stateOptions,
  ticketPriorityLabel,
  ticketTypeLabel,
  ticketTypeOptions,
} from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

// Ticket priority has no registry set: it is a queue ordering hint rather than a
// lifecycle, so it carries no transition table and no server `CHECK`. It is still
// declared once, here, because the filter and the column read from it.
const PRIORITY_COLORS: Record<string, string> = {
  Low: "default",
  Medium: "blue",
  High: "orange",
  Urgent: "red",
};

// 提交目标的标签色：组织请求是站内流转，供应商反馈会出组织边界。
const TYPE_COLORS: Record<string, string> = {
  org_request: "blue",
  vendor_feedback: "purple",
};

type TicketRow = {
  id: string;
  organizationId?: string;
  name: string;
  displayName?: string;
  type?: string;
  status: string;
  priority?: string;
  assignee?: string;
  creator?: string;
  createdAt?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

/** 详情页跳转始终携带行的组织：平台管理员跨组织访问靠它过可信租户裁决。 */
function ticketDetailPath(record: TicketRow): string {
  const base = `/tickets/${record.id}`;
  return record.organizationId
    ? `${base}?organizationId=${encodeURIComponent(record.organizationId)}`
    : base;
}

type TicketParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  status?: string;
  priority?: string;
  type?: string;
};

export default function TicketsPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const sessionOrgId = useSessionStore((state) => state.organization?.id);
  // 平台管理员判定与创建页一致：会话组织属于 built-in 组织。
  const isPlatformAdmin = account?.owner === "built-in";
  const { nameMap: orgNameMap } = useOrganizationNames();
  const [deletingKey, setDeletingKey] = useState<string | null>(null);

  const list = useCrudList<TicketRow, TicketParams, string>({
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "tickets",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.status,
        p.priority,
        p.type,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.status as string | undefined) !== undefined ? { status: filters.status } : {}),
        ...((filters.priority as string | undefined) !== undefined
          ? { priority: filters.priority }
          : {}),
        ...((filters.type as string | undefined) !== undefined ? { type: filters.type } : {}),
      },
    }),
    paginatedFn: getIamTicketsPaginated,
    // 批量删除的 key 编码了行所属组织（id|orgId）：平台管理员跨组织删除时
    // 服务端可信租户裁决需要显式 organizationId，单靠 id 会落到会话组织。
    deleteFn: (key: string) => {
      const [id, orgId] = key.split("|");
      return deleteIamTicket(id, orgId || undefined);
    },
    invalidateKeysOnDelete: [queryKeys.tickets.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
    defaultSortField: "name",
    defaultSortOrder: "ascend",
  });

  const rows = list.query.data ?? [];
  // 组织列出现条件：平台管理员的跨组织视图，或行里出现了非会话组织的工单。
  const showOrganizationColumn =
    isPlatformAdmin ||
    rows.some((row) => row.organizationId && row.organizationId !== sessionOrgId);

  return renderCrudListView<TicketRow, TicketParams, string>(
    {
      title: tr("ticket.title", "Tickets"),
      description: tr(
        "ticket.description",
        "Manage tickets, track issue resolution and user feedback.",
      ),
      searchPlaceholder: tr("ticket.searchPlaceholder", "Filter by ticket name or display name"),
      rowKey: "id",
      searchableColumns: [
        { dataIndex: "name", label: tc("name") },
        { dataIndex: "displayName", label: tc("displayName") },
      ],
      filters: [
        {
          key: "status",
          label: tr("ticket.status", "Status"),
          options: stateOptions(TICKET_STATUSES, tc),
        },
        {
          key: "priority",
          label: tr("ticket.priority", "Priority"),
          options: Object.keys(PRIORITY_COLORS).map((v) => ({
            label: ticketPriorityLabel(v, tr),
            value: v,
          })),
        },
        {
          key: "type",
          label: tr("ticket.type", "Type"),
          options: ticketTypeOptions(tr),
        },
      ],
      toolbar: (
        <PermissionGuard action="ticket:write" resource="tickets" fallback={null}>
          <LinkButton type="primary" to="/tickets/new">
            {tr("ticket.newTicket", "New Ticket")}
          </LinkButton>
        </PermissionGuard>
      ),
      columns: [
        {
          title: tr("ticket.name", "Name"),
          dataIndex: "name",
          key: "name",
          width: 200,
          fixed: "left",
          sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
          render: (v: string, record) => <Link to={ticketDetailPath(record)}>{v}</Link>,
        },
        {
          title: tr("ticket.displayName", "Display Name"),
          dataIndex: "displayName",
          key: "displayName",
        },
        {
          title: tr("ticket.type", "Type"),
          dataIndex: "type",
          key: "type",
          render: (v?: string) =>
            v ? <Tag color={TYPE_COLORS[v] ?? "default"}>{ticketTypeLabel(v, tr)}</Tag> : "-",
        },
        {
          title: tr("ticket.status", "Status"),
          dataIndex: "status",
          key: "status",
          render: (v: string) => <StateTag value={v} />,
        },
        {
          title: tr("ticket.priority", "Priority"),
          dataIndex: "priority",
          key: "priority",
          render: (v: string) => (
            <Tag color={PRIORITY_COLORS[v] ?? "default"}>{ticketPriorityLabel(v, tr)}</Tag>
          ),
        },
        ...(showOrganizationColumn
          ? [
              {
                title: tc("organization"),
                dataIndex: "organizationId",
                key: "organizationId",
                width: 180,
                render: (v?: string) => (v ? orgNameMap.get(v) || v : "-"),
              },
            ]
          : []),
        {
          title: tr("ticket.assignee", "Assignee"),
          dataIndex: "assignee",
          key: "assignee",
          width: 150,
          render: (v?: string) => v || "-",
        },
        {
          title: tr("ticket.creator", "Creator"),
          dataIndex: "creator",
          key: "creator",
          width: 150,
          render: (v?: string) => v || "-",
        },
        {
          title: tc("createdAt"),
          dataIndex: "createdAt",
          key: "createdAt",
          width: 130,
          render: (v?: string) => formatDate(v),
        },
        {
          title: tr("ticket.actions", "Actions"),
          key: "actions",
          width: 160,
          fixed: "right",
          render: (_, record) => {
            const rowKey = `${record.id}|${record.organizationId ?? ""}`;
            return (
              <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
                <LinkButton type="link" size="small" to={ticketDetailPath(record)}>
                  {tr("ticket.edit", "Edit")}
                </LinkButton>
                <Popconfirm
                  okButtonProps={{
                    loading: deletingKey === rowKey,
                    disabled: deletingKey !== null,
                  }}
                  title={tc("confirmDelete")}
                  onConfirm={async () => {
                    setDeletingKey(rowKey);
                    try {
                      await deleteIamTicket(record.id, record.organizationId);
                      message.success(tc("deleteSuccess"));
                    } catch (e) {
                      message.error(localizedError(e, tc, "common:error"));
                    } finally {
                      setDeletingKey(null);
                    }
                  }}
                >
                  <Button type="link" size="small" danger disabled={deletingKey !== null}>
                    {tc("delete")}
                  </Button>
                </Popconfirm>
              </div>
            );
          },
        },
      ],
      enableBulkDelete: true,
      rowKeyToDeleteKey: (record: TicketRow) => `${record.id}|${record.organizationId ?? ""}`,
    },
    list,
  );
}

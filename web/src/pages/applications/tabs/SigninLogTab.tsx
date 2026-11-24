import { Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useMemo, useState } from "react";
import { getSessionsPaginated, type SessionItem } from "@/shared/api/iam/session";
import { ResourceListCard, type ServerPagination } from "@/shared/ui/ResourceListCard";
import type { ColumnsType } from "antd/es/table";

interface SigninLogTabProps {
  applicationId: string;
}

/**
 * Sign-in history for one application.
 *
 * This renders sessions rather than a bespoke sign-in log: a session row *is* a
 * successful sign-in for an application, and `sessions` already carries
 * `application_id` with a dedicated index (idx_sessions_org_app_created). The
 * previous implementation called `GET /api/v1/applications/{id}/signin-log`,
 * which no handler serves, so the tab was permanently empty.
 */
export default function SigninLogTab({ applicationId }: SigninLogTabProps) {
  const { t: tr } = useTranslation("resource");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const query = useQuery({
    queryKey: ["application-signin-log", applicationId, search, page, pageSize],
    queryFn: () =>
      getSessionsPaginated({
        applicationId,
        search: search || undefined,
        page,
        pageSize,
      }),
    enabled: !!applicationId,
  });

  const serverPagination: ServerPagination = {
    page,
    pageSize,
    total: query.data?.total ?? 0,
    onChange: (p, ps) => {
      setPage(p);
      setPageSize(ps);
    },
  };

  // ResourceListCard expects UseQueryResult<SessionItem[], Error>, but our
  // query returns { items, total }. Wrap it so `query.data` reads as SessionItem[].
  const listQuery = useMemo(
    () =>
      ({
        ...query,
        data: query.data?.items ?? [],
      }) as unknown as UseQueryResult<SessionItem[], Error>,
    [query],
  );

  const columns: ColumnsType<SessionItem> = useMemo(
    () => [
      {
        title: tr("application.signinLogSection.username"),
        dataIndex: "username",
        key: "username",
        width: 200,
        fixed: "left",
      },
      {
        title: tr("application.signinLogSection.clientIp"),
        dataIndex: "ipAddress",
        key: "ipAddress",
        render: (v: string) => <span style={{ fontFamily: "monospace" }}>{v}</span>,
      },
      {
        title: tr("application.signinLogSection.userAgent"),
        dataIndex: "userAgent",
        key: "userAgent",
        ellipsis: true,
      },
      {
        title: tr("application.signinLogSection.result"),
        dataIndex: "sessionState",
        key: "sessionState",
        render: (v: string) =>
          v === "active" ? (
            <Tag color="green">{tr("user.statusActive")}</Tag>
          ) : (
            <Tag color="red">{v}</Tag>
          ),
      },
      {
        title: tr("application.signinLogSection.createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        render: (value: string) => {
          if (!value) return "-";
          const parsed = new Date(value);
          return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
        },
      },
    ],
    [tr],
  );

  return (
    <ResourceListCard
      title={tr("application.tabs.signinLog")}
      query={listQuery}
      columns={columns}
      rowKey="id"
      tableId="application-signin-log"
      serverPagination={serverPagination}
      onSearch={(value) => {
        setPage(1);
        setSearch(value);
      }}
      showRefresh={false}
      hideSearch={false}
      tableScrollX="max-content"
      searchPlaceholder={tr("application.signinLogSection.username")}
    />
  );
}

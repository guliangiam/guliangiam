import { Empty, Tag, Typography } from "antd";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { listPrivilegedSessions, type PrivilegedSessionItem } from "@/shared/api/iam/iga";
import { ResourceListCard, type ServerPagination } from "@/shared/ui/ResourceListCard";
import { useSessionStore } from "@/features/session/session-store";

function formatDateTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export default function IgaPrivilegedSessionsTab() {
  const { t: tc } = useTranslation("common");
  const organizationId = useSessionStore((state) => state.organization?.id ?? undefined);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const query = useQuery({
    queryKey: ["iga", "privileged-sessions", page, pageSize],
    queryFn: () => listPrivilegedSessions({ page, pageSize }),
    enabled: !!organizationId,
  });

  const listQuery = {
    ...query,
    data: query.data?.items ?? [],
  } as unknown as UseQueryResult<PrivilegedSessionItem[], Error>;

  const serverPagination: ServerPagination = {
    page,
    pageSize,
    total: query.data?.page?.total ?? 0,
    onChange: (nextPage, nextPageSize) => {
      setPage(nextPage);
      setPageSize(nextPageSize);
    },
  };

  return (
    <ResourceListCard<PrivilegedSessionItem>
      title={tc("nav.iga.privilegedSessions")}
      description={tc("iga.requests.sessionDescription")}
      query={listQuery}
      rowKey="sessionId"
      serverPagination={serverPagination}
      tableId="iga-privileged-sessions"
      tableScrollX="max-content"
      emptyContent={
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={tc("iga.requests.emptySessions")}
        />
      }
      columns={[
        {
          title: tc("iga.requests.sessionId"),
          dataIndex: "sessionId",
          key: "sessionId",
          width: 220,
          render: (value: string) => (
            <Typography.Text copyable={{ text: value }} ellipsis style={{ maxWidth: 200 }}>
              {value}
            </Typography.Text>
          ),
        },
        {
          title: tc("iga.requests.privileged"),
          dataIndex: "privileged",
          key: "privileged",
          width: 130,
          render: (value: boolean) =>
            value ? (
              <Tag color="green">{tc("iga.requests.privileged")}</Tag>
            ) : (
              <Tag>{tc("iga.requests.notPrivileged")}</Tag>
            ),
        },
        {
          title: tc("iga.requests.sessionState"),
          dataIndex: "sessionState",
          key: "sessionState",
          width: 140,
          render: (value: string) => <Tag>{value}</Tag>,
        },
        {
          title: tc("iga.requests.ticketId"),
          dataIndex: "ticketId",
          key: "ticketId",
          width: 200,
          ellipsis: true,
        },
        {
          title: tc("iga.requests.lifecycle"),
          dataIndex: "lifecycleId",
          key: "lifecycleId",
          width: 200,
          ellipsis: true,
          render: (value: string) => value || "-",
        },
        {
          title: tc("expiresAt"),
          dataIndex: "expiresAt",
          key: "expiresAt",
          width: 170,
          render: (value: string) => formatDateTime(value),
        },
      ]}
    />
  );
}

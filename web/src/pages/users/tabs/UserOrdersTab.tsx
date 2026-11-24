import { useState } from "react";
import { Tag } from "antd";
import { ResourceListCard, type ServerPagination } from "@/shared/ui/ResourceListCard";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getUserOrdersByUserId, type IamOrderItem } from "@/shared/api/iam/order";
import type { UseQueryResult } from "@tanstack/react-query";

type Props = {
  userId: string;
  organizationId?: string;
};

const STATE_COLORS: Record<string, string> = {
  Created: "blue",
  Pending: "orange",
  Paid: "green",
  Completed: "green",
  Cancelled: "red",
  Refunded: "purple",
  Failed: "red",
  payment_pending: "gold",
};

export default function UserOrdersTab({ userId, organizationId }: Props) {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const query = useQuery({
    queryKey: ["user-orders", userId, organizationId, page, pageSize],
    queryFn: () => getUserOrdersByUserId(userId, { organizationId, page, pageSize }),
    enabled: !!userId,
  });

  const total = query.data?.page?.total ?? 0;

  const serverPagination: ServerPagination = {
    page,
    pageSize,
    total,
    onChange: (p, ps) => {
      setPage(p);
      setPageSize(ps);
    },
  };

  const listQuery = {
    ...query,
    data: query.data?.items ?? [],
  } as unknown as UseQueryResult<IamOrderItem[], Error>;

  const columns = [
    {
      title: t("common:name"),
      dataIndex: "name",
      width: 160,
      render: (name: string) => name || "-",
    },
    {
      title: t("common:displayName"),
      dataIndex: "displayName",
      width: 160,
      render: (name: string) => name || "-",
    },
    {
      title: t("common:state"),
      dataIndex: "state",
      width: 120,
      render: (state: string) =>
        state ? <Tag color={STATE_COLORS[state] ?? "default"}>{state}</Tag> : "-",
    },
    {
      title: t("common:createdAt"),
      dataIndex: "createdAt",
      width: 140,
      render: (v: string) => (v ? new Date(v).toLocaleString() : "-"),
    },
  ];

  return (
    <ResourceListCard
      query={listQuery}
      columns={columns}
      rowKey="id"
      tableId="user-orders"
      serverPagination={serverPagination}
      embedded
      showRefresh={false}
      tableScrollX="max-content"
    />
  );
}

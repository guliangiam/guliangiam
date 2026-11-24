import { Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listAccessLifecycle, type AccessLifecycleItem } from "@/shared/api/iam/iga";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";

const statusColor: Record<string, string> = {
  active: "green",
  expired: "default",
  revoked: "red",
};

export default function IgaAccessLifecycleTab() {
  const { t: tc } = useTranslation("common");

  const query = useQuery({
    queryKey: ["iga", "lifecycle"],
    queryFn: listAccessLifecycle,
    select: (data) => data?.items ?? [],
  });

  return (
    <ResourceListCard<AccessLifecycleItem>
      title={tc("nav.iga.lifecycle", "Access Lifecycle")}
      description={tc(
        "iga.lifecycle.description",
        "Time-bound role grants issued by approved access requests.",
      )}
      query={query}
      rowKey="id"
      tableId="iga-access-lifecycle"
      columns={[
        { title: tc("user"), dataIndex: "userId", key: "userId" },
        { title: tc("iga.role", "Role"), dataIndex: "roleId", key: "roleId" },
        {
          title: tc("status", "Status"),
          dataIndex: "status",
          key: "status",
          render: (v: string) => <Tag color={statusColor[v] ?? "blue"}>{v}</Tag>,
        },
        {
          title: tc("iga.grantedAt", "Granted At"),
          dataIndex: "grantedAt",
          key: "grantedAt",
          render: (v: string) => new Date(v).toLocaleString(),
        },
        {
          title: tc("expiresAt", "Expires At"),
          dataIndex: "expiresAt",
          key: "expiresAt",
          render: (v: string) => new Date(v).toLocaleString(),
        },
      ]}
    />
  );
}

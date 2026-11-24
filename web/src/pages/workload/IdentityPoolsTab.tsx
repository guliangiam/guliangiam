import { useState } from "react";
import { Button, Empty } from "antd";
import PlusOutlined from "@ant-design/icons/PlusOutlined";
import { useTranslation } from "react-i18next";
import { useIdentityPools } from "@/shared/hooks/use-identity-pools";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import type { IdentityPoolItem } from "@/shared/api/iam/identity-pools";
import { PoolFormModal } from "./PoolFormModal";
import { formatWorkloadDate } from "./workload-utils";

type IdentityPoolsTabProps = {
  onSelectPool: (id: string) => void;
  activePoolId?: string;
};

export function IdentityPoolsTab({ onSelectPool, activePoolId }: IdentityPoolsTabProps) {
  const { t: tc } = useTranslation("common");
  const [createOpen, setCreateOpen] = useState(false);
  const query = useIdentityPools();

  return (
    <>
      <ResourceListCard<IdentityPoolItem>
        title={tc("nav.workload.pools", { defaultValue: "Identity Pools" })}
        description={tc("workload.pools.description", {
          defaultValue: "Select a pool to configure its federation trusts and service accounts.",
        })}
        query={query}
        tableId="workload-identity-pools"
        emptyContent={
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tc("noData")}>
            <PermissionGuard action="identity-pools:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("workload.createPool", { defaultValue: "Create Identity Pool" })}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="identity-pools:write" fallback={null}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              {tc("workload.createPool", { defaultValue: "Create Identity Pool" })}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("name"), dataIndex: "name", key: "name" },
          {
            title: tc("displayName", { defaultValue: "Display Name" }),
            dataIndex: "displayName",
            key: "displayName",
            render: (value: string) => value || "-",
          },
          {
            title: tc("description"),
            dataIndex: "description",
            key: "description",
            render: (value: string) => value || "-",
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: formatWorkloadDate,
          },
        ]}
        rowKey="id"
        tableScrollX="max-content"
        rowClassName={(record) => (record.id === activePoolId ? "ant-table-row-selected" : "")}
        onRow={(record) => ({
          onClick: () => onSelectPool(record.id),
          style: { cursor: "pointer" },
        })}
      />
      <PoolFormModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(pool) => onSelectPool(pool.id)}
      />
    </>
  );
}

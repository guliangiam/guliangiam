import { useState } from "react";
import { Alert, App, Button, Empty, Popconfirm, Space } from "antd";
import PlusOutlined from "@ant-design/icons/PlusOutlined";
import { useTranslation } from "react-i18next";
import {
  useDeleteFederationTrust,
  useFederationTrusts,
  useSetFederationTrustStatus,
} from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import type { FederationTrustItem } from "@/shared/api/iam/identity-pools";
import { TrustFormModal } from "./TrustFormModal";
import { TrustDetailDrawer } from "./TrustDetailDrawer";
import { StringTags, WorkloadStatusTag } from "./WorkloadValues";

type FederationTrustsTabProps = { poolId?: string };

export function FederationTrustsTab({ poolId }: FederationTrustsTabProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedTrustId, setSelectedTrustId] = useState<string>();
  const query = useFederationTrusts(poolId);
  const statusMutation = useSetFederationTrustStatus(poolId);
  const deleteMutation = useDeleteFederationTrust(poolId);

  const changeStatus = async (trust: FederationTrustItem) => {
    const status = trust.status === "active" ? "disabled" : "active";
    try {
      await statusMutation.mutateAsync({ id: trust.id, status });
      message.success(tc("workload.statusUpdated", { defaultValue: "Status updated" }));
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.statusUpdateFailed", { defaultValue: "Failed to update status" }),
      );
    }
  };

  const removeTrust = async (trust: FederationTrustItem) => {
    try {
      await deleteMutation.mutateAsync(trust.id);
      if (selectedTrustId === trust.id) setSelectedTrustId(undefined);
      message.success(tc("workload.trustDeleted", { defaultValue: "Federation trust deleted" }));
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.trustDeleteFailed", { defaultValue: "Failed to delete federation trust" }),
      );
    }
  };

  if (!poolId) {
    return (
      <Alert
        type="info"
        showIcon
        title={tc("workload.selectPoolFirst", {
          defaultValue: "Select an Identity Pool first",
        })}
      />
    );
  }

  return (
    <>
      <ResourceListCard<FederationTrustItem>
        title={tc("nav.workload.trusts", { defaultValue: "Federation Trusts" })}
        description={tc("workload.trusts.description", {
          defaultValue: "OIDC issuers and explicit subject patterns trusted by this pool.",
        })}
        query={query}
        tableId="workload-federation-trusts"
        emptyContent={
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tc("noData")}>
            <PermissionGuard action="identity-pools:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("workload.createTrust", { defaultValue: "Create Federation Trust" })}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="identity-pools:write" fallback={null}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              {tc("workload.createTrust", { defaultValue: "Create Federation Trust" })}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("name"), dataIndex: "name", key: "name" },
          {
            title: tc("workload.issuer", { defaultValue: "Issuer" }),
            dataIndex: "issuer",
            key: "issuer",
          },
          {
            title: tc("workload.audience", { defaultValue: "Audience" }),
            dataIndex: "audience",
            key: "audience",
            render: (values: string[]) => <StringTags values={values} />,
          },
          {
            title: tc("workload.subjects", { defaultValue: "Subject patterns" }),
            dataIndex: "subjects",
            key: "subjects",
            render: (values: string[]) => <StringTags values={values} />,
          },
          {
            title: tc("status", { defaultValue: "Status" }),
            dataIndex: "status",
            key: "status",
            render: (_value, trust) => <WorkloadStatusTag status={trust.status} />,
          },
          {
            title: tc("actions", { defaultValue: "Actions" }),
            key: "actions",
            width: 270,
            render: (_value, trust) => (
              <Space onClick={(event) => event.stopPropagation()}>
                <Button type="link" size="small" onClick={() => setSelectedTrustId(trust.id)}>
                  {tc("details", { defaultValue: "Details" })}
                </Button>
                <PermissionGuard action="identity-pools:write" fallback={null}>
                  <Popconfirm
                    title={tc("workload.confirmTrustStatus", {
                      defaultValue: "Change federation trust status?",
                    })}
                    onConfirm={() => changeStatus(trust)}
                    okButtonProps={{
                      loading:
                        statusMutation.isPending && statusMutation.variables?.id === trust.id,
                    }}
                  >
                    <Button type="link" size="small">
                      {trust.status === "active"
                        ? tc("disable", { defaultValue: "Disable" })
                        : tc("enable", { defaultValue: "Enable" })}
                    </Button>
                  </Popconfirm>
                  <Popconfirm
                    title={tc("workload.confirmDeleteTrust", {
                      defaultValue: "Delete this federation trust?",
                    })}
                    okText={tc("delete", { defaultValue: "Delete" })}
                    okButtonProps={{
                      danger: true,
                      loading: deleteMutation.isPending && deleteMutation.variables === trust.id,
                    }}
                    onConfirm={() => removeTrust(trust)}
                  >
                    <Button type="link" size="small" danger>
                      {tc("delete", { defaultValue: "Delete" })}
                    </Button>
                  </Popconfirm>
                </PermissionGuard>
              </Space>
            ),
          },
        ]}
        rowKey="id"
        tableScrollX={1400}
        onRow={(trust) => ({
          onClick: () => setSelectedTrustId(trust.id),
          style: { cursor: "pointer" },
        })}
      />
      <TrustFormModal open={createOpen} poolId={poolId} onClose={() => setCreateOpen(false)} />
      {selectedTrustId && (
        <TrustDetailDrawer
          open
          poolId={poolId}
          trustId={selectedTrustId}
          onClose={() => setSelectedTrustId(undefined)}
        />
      )}
    </>
  );
}

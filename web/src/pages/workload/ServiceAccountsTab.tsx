import { useState } from "react";
import { Alert, App, Button, Empty, Popconfirm, Space, Tag } from "antd";
import PlusOutlined from "@ant-design/icons/PlusOutlined";
import { useTranslation } from "react-i18next";
import {
  useRotateServiceAccountSecret,
  useServiceAccounts,
  useSetServiceAccountStatus,
} from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import type {
  ServiceAccountCredentials,
  ServiceAccountItem,
} from "@/shared/api/iam/identity-pools";
import { ServiceAccountFormModal } from "./ServiceAccountFormModal";
import { ServiceAccountDetailDrawer } from "./ServiceAccountDetailDrawer";
import { SecretModal } from "./SecretModal";
import { AccountDeleteModal } from "./AccountDeleteModal";
import { formatWorkloadDate, getServiceAccountCredentials } from "./workload-utils";
import { StringTags, WorkloadStatusTag } from "./WorkloadValues";

type ServiceAccountsTabProps = { poolId?: string };

export function ServiceAccountsTab({ poolId }: ServiceAccountsTabProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedAccountId, setSelectedAccountId] = useState<string>();
  const [deleteAccount, setDeleteAccount] = useState<ServiceAccountItem>();
  const [credentials, setCredentials] = useState<ServiceAccountCredentials | null>(null);
  const query = useServiceAccounts(poolId);
  const statusMutation = useSetServiceAccountStatus(poolId);
  const rotateMutation = useRotateServiceAccountSecret(poolId);

  const changeStatus = async (account: ServiceAccountItem) => {
    const status = account.status === "active" ? "disabled" : "active";
    try {
      await statusMutation.mutateAsync({ id: account.id, status, revision: account.revision });
      message.success(tc("workload.statusUpdated", { defaultValue: "Status updated" }));
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.statusUpdateFailed", { defaultValue: "Failed to update status" }),
      );
    }
  };

  const rotateSecret = async (account: ServiceAccountItem) => {
    try {
      const rotated = await rotateMutation.mutateAsync({
        id: account.id,
        revision: account.revision,
      });
      const nextCredentials = getServiceAccountCredentials(rotated);
      rotateMutation.reset();
      setCredentials(nextCredentials);
      message.success(tc("workload.secretRotated", { defaultValue: "Client secret rotated" }));
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.secretRotateFailed", { defaultValue: "Failed to rotate client secret" }),
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
      <ResourceListCard<ServiceAccountItem>
        title={tc("nav.workload.serviceAccounts", { defaultValue: "Service Accounts" })}
        description={tc("workload.serviceAccounts.description", {
          defaultValue: "Issue scoped, short-lived workload credentials for this pool.",
        })}
        query={query}
        tableId="workload-service-accounts"
        emptyContent={
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tc("noData")}>
            <PermissionGuard action="identity-pools:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("workload.createAccount", { defaultValue: "Create Service Account" })}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="identity-pools:write" fallback={null}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              {tc("workload.createAccount", { defaultValue: "Create Service Account" })}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("name"), dataIndex: "name", key: "name" },
          {
            title: tc("workload.scopes", { defaultValue: "Scopes" }),
            dataIndex: "scopes",
            key: "scopes",
            render: (values: string[]) => <StringTags values={values} />,
          },
          {
            title: tc("workload.accessTokenTtlSeconds", { defaultValue: "TTL (seconds)" }),
            dataIndex: "accessTokenTtlSeconds",
            key: "accessTokenTtlSeconds",
          },
          {
            title: tc("workload.lastRotatedAt", { defaultValue: "Last rotated" }),
            dataIndex: "lastRotatedAt",
            key: "lastRotatedAt",
            render: formatWorkloadDate,
          },
          {
            title: tc("workload.secretSet", { defaultValue: "Secret set" }),
            dataIndex: "secretSet",
            key: "secretSet",
            render: (value: boolean) => (
              <Tag color={value ? "green" : "default"}>
                {value ? tc("yes", { defaultValue: "Yes" }) : tc("no", { defaultValue: "No" })}
              </Tag>
            ),
          },
          {
            title: tc("status", { defaultValue: "Status" }),
            dataIndex: "status",
            key: "status",
            render: (_value, account) => <WorkloadStatusTag status={account.status} />,
          },
          {
            title: tc("actions", { defaultValue: "Actions" }),
            key: "actions",
            width: 350,
            render: (_value, account) => (
              <Space onClick={(event) => event.stopPropagation()}>
                <Button type="link" size="small" onClick={() => setSelectedAccountId(account.id)}>
                  {tc("details", { defaultValue: "Details" })}
                </Button>
                <PermissionGuard action="identity-pools:write" fallback={null}>
                  <Popconfirm
                    title={tc("workload.confirmAccountStatus", {
                      defaultValue: "Change service account status?",
                    })}
                    onConfirm={() => changeStatus(account)}
                    okButtonProps={{
                      loading:
                        statusMutation.isPending && statusMutation.variables?.id === account.id,
                    }}
                  >
                    <Button type="link" size="small">
                      {account.status === "active"
                        ? tc("disable", { defaultValue: "Disable" })
                        : tc("enable", { defaultValue: "Enable" })}
                    </Button>
                  </Popconfirm>
                  <Popconfirm
                    title={tc("workload.confirmRotateSecret", {
                      defaultValue: "Rotate this client secret? Existing tokens will be revoked.",
                    })}
                    okText={tc("workload.rotate", { defaultValue: "Rotate" })}
                    onConfirm={() => rotateSecret(account)}
                    okButtonProps={{
                      loading:
                        rotateMutation.isPending && rotateMutation.variables?.id === account.id,
                    }}
                  >
                    <Button type="link" size="small">
                      {tc("workload.rotate", { defaultValue: "Rotate" })}
                    </Button>
                  </Popconfirm>
                  <Button type="link" size="small" danger onClick={() => setDeleteAccount(account)}>
                    {tc("delete", { defaultValue: "Delete" })}
                  </Button>
                </PermissionGuard>
              </Space>
            ),
          },
        ]}
        rowKey="id"
        tableScrollX={1500}
        onRow={(account) => ({
          onClick: () => setSelectedAccountId(account.id),
          style: { cursor: "pointer" },
        })}
      />
      <ServiceAccountFormModal
        open={createOpen}
        poolId={poolId}
        onClose={() => setCreateOpen(false)}
        onCreated={setCredentials}
      />
      <SecretModal credentials={credentials} onClose={() => setCredentials(null)} />
      {selectedAccountId && (
        <ServiceAccountDetailDrawer
          open
          poolId={poolId}
          id={selectedAccountId}
          onClose={() => setSelectedAccountId(undefined)}
        />
      )}
      {deleteAccount && (
        <AccountDeleteModal
          open
          poolId={poolId}
          account={deleteAccount}
          onClose={() => setDeleteAccount(undefined)}
          onDeleted={() => {
            if (selectedAccountId === deleteAccount.id) setSelectedAccountId(undefined);
            setDeleteAccount(undefined);
          }}
        />
      )}
    </>
  );
}

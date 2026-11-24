import { Alert, Button, Descriptions, Drawer, Empty, Skeleton, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useServiceAccount } from "@/shared/hooks/use-identity-pools";
import { formatWorkloadDate } from "./workload-utils";
import { StringTags, WorkloadStatusTag } from "./WorkloadValues";
import { localizedError } from "@/shared/lib/server-error";

type ServiceAccountDetailDrawerProps = {
  open: boolean;
  poolId: string;
  id: string;
  onClose: () => void;
};

export function ServiceAccountDetailDrawer({
  open,
  poolId,
  id,
  onClose,
}: ServiceAccountDetailDrawerProps) {
  const { t: tc } = useTranslation("common");
  const query = useServiceAccount(poolId, id);
  const account = query.data;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="min(640px, 100vw)"
      title={tc("workload.accountDetails", { defaultValue: "Service account details" })}
      destroyOnHidden
    >
      {query.isLoading ? (
        <Skeleton active />
      ) : query.isError ? (
        <Alert
          type="error"
          showIcon
          title={tc("loadFailed")}
          description={localizedError(query.error, tc, "common:error")}
          action={
            <Button onClick={() => void query.refetch()}>
              {tc("retry", { defaultValue: "Retry" })}
            </Button>
          }
        />
      ) : account ? (
        <Descriptions bordered size="small" column={1}>
          <Descriptions.Item label={tc("id", { defaultValue: "ID" })}>
            {account.id}
          </Descriptions.Item>
          <Descriptions.Item label={tc("organizationId", { defaultValue: "Organization ID" })}>
            {account.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.poolId", { defaultValue: "Pool ID" })}>
            {account.poolId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("name")}>{account.name}</Descriptions.Item>
          <Descriptions.Item label={tc("description")}>
            {account.description || "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.clientId", { defaultValue: "Client ID" })}>
            {account.clientId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.scopes", { defaultValue: "Scopes" })}>
            <StringTags values={account.scopes} />
          </Descriptions.Item>
          <Descriptions.Item
            label={tc("workload.accessTokenTtlSeconds", {
              defaultValue: "Access token TTL (seconds)",
            })}
          >
            {account.accessTokenTtlSeconds}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.lastRotatedAt", { defaultValue: "Last rotated" })}>
            {formatWorkloadDate(account.lastRotatedAt)}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.secretSet", { defaultValue: "Secret set" })}>
            <Tag color={account.secretSet ? "green" : "default"}>
              {account.secretSet
                ? tc("yes", { defaultValue: "Yes" })
                : tc("no", { defaultValue: "No" })}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label={tc("status", { defaultValue: "Status" })}>
            <WorkloadStatusTag status={account.status} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("createdAt")}>
            {formatWorkloadDate(account.createdAt)}
          </Descriptions.Item>
          <Descriptions.Item label={tc("updatedAt")}>
            {formatWorkloadDate(account.updatedAt)}
          </Descriptions.Item>
          <Descriptions.Item label={tc("revision", { defaultValue: "Revision" })}>
            {account.revision}
          </Descriptions.Item>
        </Descriptions>
      ) : (
        <Empty description={tc("noData")} />
      )}
    </Drawer>
  );
}

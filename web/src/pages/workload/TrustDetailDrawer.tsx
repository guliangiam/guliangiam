import { Alert, Button, Descriptions, Drawer, Empty, Skeleton } from "antd";
import { useTranslation } from "react-i18next";
import { useFederationTrust } from "@/shared/hooks/use-identity-pools";
import { formatWorkloadDate } from "./workload-utils";
import { StringTags, WorkloadStatusTag } from "./WorkloadValues";
import { localizedError } from "@/shared/lib/server-error";

type TrustDetailDrawerProps = {
  open: boolean;
  poolId: string;
  trustId: string;
  onClose: () => void;
};

export function TrustDetailDrawer({ open, poolId, trustId, onClose }: TrustDetailDrawerProps) {
  const { t: tc } = useTranslation("common");
  const query = useFederationTrust(poolId, trustId);
  const trust = query.data;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="min(640px, 100vw)"
      title={tc("workload.trustDetails", { defaultValue: "Federation trust details" })}
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
      ) : trust ? (
        <Descriptions bordered size="small" column={1}>
          <Descriptions.Item label={tc("id", { defaultValue: "ID" })}>{trust.id}</Descriptions.Item>
          <Descriptions.Item label={tc("organizationId", { defaultValue: "Organization ID" })}>
            {trust.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.poolId", { defaultValue: "Pool ID" })}>
            {trust.poolId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("name")}>{trust.name}</Descriptions.Item>
          <Descriptions.Item label={tc("workload.issuer", { defaultValue: "Issuer" })}>
            {trust.issuer}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.jwksUri", { defaultValue: "JWKS URI" })}>
            {trust.jwksUri}
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.audience", { defaultValue: "Audience" })}>
            <StringTags values={trust.audience} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("workload.subjects", { defaultValue: "Subject patterns" })}>
            <StringTags values={trust.subjects} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("status", { defaultValue: "Status" })}>
            <WorkloadStatusTag status={trust.status} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("createdAt")}>
            {formatWorkloadDate(trust.createdAt)}
          </Descriptions.Item>
          <Descriptions.Item label={tc("updatedAt")}>
            {formatWorkloadDate(trust.updatedAt)}
          </Descriptions.Item>
          <Descriptions.Item label={tc("revision", { defaultValue: "Revision" })}>
            {trust.revision}
          </Descriptions.Item>
        </Descriptions>
      ) : (
        <Empty description={tc("noData")} />
      )}
    </Drawer>
  );
}

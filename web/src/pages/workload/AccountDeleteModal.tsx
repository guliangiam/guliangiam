import { Alert, App, Button, Descriptions, Modal, Skeleton } from "antd";
import { useTranslation } from "react-i18next";
import type { ServiceAccountItem } from "@/shared/api/iam/identity-pools";
import {
  useDeleteServiceAccount,
  useServiceAccountDeletePrecheck,
  useSetServiceAccountStatus,
} from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { StringTags, WorkloadStatusTag } from "./WorkloadValues";
import { localizedError } from "@/shared/lib/server-error";

type AccountDeleteModalProps = {
  open: boolean;
  poolId: string;
  account: ServiceAccountItem;
  onClose: () => void;
  onDeleted: () => void;
};

export function AccountDeleteModal({
  open,
  poolId,
  account,
  onClose,
  onDeleted,
}: AccountDeleteModalProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const precheckQuery = useServiceAccountDeletePrecheck(poolId, account.id, open);
  const statusMutation = useSetServiceAccountStatus(poolId);
  const deleteMutation = useDeleteServiceAccount(poolId);
  const precheck = precheckQuery.data;

  const disableAndRefresh = async () => {
    if (!precheck) return;
    try {
      await statusMutation.mutateAsync({
        id: account.id,
        status: "disabled",
        revision: precheck.revision,
      });
      await precheckQuery.refetch();
      message.success(
        tc("workload.accountDisabledPrecheck", {
          defaultValue: "Account disabled; delete precheck refreshed",
        }),
      );
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.statusUpdateFailed", { defaultValue: "Failed to update status" }),
      );
    }
  };

  const remove = async () => {
    if (!precheck?.canDelete) return;
    try {
      await deleteMutation.mutateAsync({ id: account.id, revision: precheck.revision });
      message.success(tc("workload.accountDeleted", { defaultValue: "Service account deleted" }));
      onDeleted();
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.accountDeleteFailed", { defaultValue: "Failed to delete service account" }),
      );
    }
  };

  return (
    <Modal
      open={open}
      title={tc("workload.deleteAccount", { defaultValue: "Delete Service Account" })}
      okText={tc("delete", { defaultValue: "Delete" })}
      cancelText={tc("cancel", { defaultValue: "Cancel" })}
      okButtonProps={{
        danger: true,
        disabled: !precheck?.canDelete || precheckQuery.isFetching,
      }}
      confirmLoading={deleteMutation.isPending}
      onOk={() => void remove()}
      onCancel={onClose}
      destroyOnHidden
    >
      {precheckQuery.isLoading ? (
        <Skeleton active />
      ) : precheckQuery.isError ? (
        <Alert
          type="error"
          showIcon
          title={tc("workload.precheckFailed", { defaultValue: "Delete precheck failed" })}
          description={localizedError(precheckQuery.error, tc, "common:error")}
          action={
            <Button size="small" onClick={() => void precheckQuery.refetch()}>
              {tc("retry", { defaultValue: "Retry" })}
            </Button>
          }
        />
      ) : precheck ? (
        <>
          {precheck.activeTokenCount > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              title={tc("workload.activeTokensBlockDelete", {
                defaultValue: "Active tokens prevent deletion. Disable the account first.",
              })}
              action={
                <Button
                  size="small"
                  onClick={() => void disableAndRefresh()}
                  loading={statusMutation.isPending}
                >
                  {tc("workload.disableFirst", { defaultValue: "Disable first" })}
                </Button>
              }
            />
          )}
          {!precheck.canDelete && precheck.activeTokenCount === 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              title={tc("workload.cannotDeleteAccount", {
                defaultValue: "This account cannot be deleted in its current state.",
              })}
            />
          )}
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label={tc("name")}>{account.name}</Descriptions.Item>
            <Descriptions.Item
              label={tc("workload.activeTokenCount", { defaultValue: "Active tokens" })}
            >
              {precheck.activeTokenCount}
            </Descriptions.Item>
            <Descriptions.Item label={tc("workload.scopes", { defaultValue: "Scopes" })}>
              <StringTags values={precheck.scopes} />
            </Descriptions.Item>
            <Descriptions.Item label={tc("status", { defaultValue: "Status" })}>
              <WorkloadStatusTag status={precheck.status} />
            </Descriptions.Item>
            <Descriptions.Item label={tc("workload.canDelete", { defaultValue: "Can delete" })}>
              {precheck.canDelete
                ? tc("yes", { defaultValue: "Yes" })
                : tc("no", { defaultValue: "No" })}
            </Descriptions.Item>
            <Descriptions.Item label={tc("revision", { defaultValue: "Revision" })}>
              {precheck.revision}
            </Descriptions.Item>
          </Descriptions>
        </>
      ) : null}
    </Modal>
  );
}

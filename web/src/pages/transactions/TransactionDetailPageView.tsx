import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Popconfirm,
  Result,
  Select,
  Skeleton,
} from "antd";
import { isHttpError } from "@/shared/api/http";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { NotFoundResult } from "@/shared/ui/QueryErrorResult";
import { FormSection } from "@/shared/ui/FormSection";
import { StateTag } from "@/shared/ui/StateTag";
import type { TransactionFormValues } from "./useTransactionDetailPageController";
import { useTransactionDetailPageController } from "./useTransactionDetailPageController";

type Props = { ctrl: ReturnType<typeof useTransactionDetailPageController> };

export function TransactionDetailPageView({ ctrl }: Props) {
  const {
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    detailQuery,
    updateMutation,
    deleteMutation,
    TRANSACTION_STATUSES,
    TRANSACTION_TRANSITIONS,
    allowedTargets,
    stateCaptionKey,
  } = ctrl;

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
    if (isHttpError(detailQuery.error) && detailQuery.error.status === 404) {
      return <NotFoundResult title={tc("notFound")} description={tc("notFoundDesc")} />;
    }
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert type="error" showIcon title={tc("notFound")} description={tc("notFound")} />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/transactions")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("transaction.detailTitle", { name: data.id })}
      description={tr("transaction.detailDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/transactions")}
      onFinish={async (values: TransactionFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tr("transaction.deleteConfirmTitle")}
          description={tr("transaction.deleteConfirmDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.id}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tr("transaction.transactionType")}>
            {tc(stateCaptionKey(data.transactionType), { defaultValue: data.transactionType })}
          </Descriptions.Item>
          <Descriptions.Item label={tr("transaction.currentStatus")}>
            <StateTag value={data.status} />
          </Descriptions.Item>
          <Descriptions.Item label={tr("transaction.amount")}>
            {data.amount} {data.currency}
          </Descriptions.Item>
          <Descriptions.Item label={tr("transaction.linkedOrder")}>
            {data.orderId || tc("no")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("user")}>{data.userId || tc("no")}</Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tc("editSection", "Edit")}>
        <Form.Item label={tc("statusLabel")} name="status">
          <Select
            options={allowedTargets(TRANSACTION_STATUSES, data.status, TRANSACTION_TRANSITIONS, tc)}
          />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

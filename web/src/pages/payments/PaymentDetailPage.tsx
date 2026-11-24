import { useEffect, useState, type ReactNode } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";

const { Text } = Typography;
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  FileTextOutlined,
  ExperimentOutlined,
  MinusCircleOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnsType, TablePaginationConfig } from "antd/es/table";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamPayment, getIamPayment, patchIamPayment, invoicePayment } from "@/shared/api/iam";
import {
  listAttemptsByPayment,
  listRefundsByPayment,
  type AttemptListItem,
  type RefundListItem,
} from "@/shared/api/payments-admin";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { StateTag } from "@/shared/ui/StateTag";
import { PAYMENT_STATES } from "@/shared/constants/enums";
import { PAYMENT_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

type PaymentFormValues = {
  displayName: string;
  state: string;
};

type InvoiceFormValues = {
  personName: string;
  personIdCard: string;
  personEmail: string;
  personPhone: string;
  invoiceType: string;
  invoiceTitle: string;
  invoiceTaxId: string;
  invoiceRemark: string;
};

export default function PaymentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PaymentFormValues>();
  const [invoiceForm] = Form.useForm<InvoiceFormValues>();
  // Sub-resource lists are paged by the server; the console owns the page
  // cursor so the request and the table always describe the same slice.
  const [attemptsPage, setAttemptsPage] = useState(1);
  const [attemptsPageSize, setAttemptsPageSize] = useState(20);
  const [refundsPage, setRefundsPage] = useState(1);
  const [refundsPageSize, setRefundsPageSize] = useState(20);
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.payments.detail(id!),
    queryFn: () => getIamPayment(id!),
    enabled: !!id,
  });

  // Attempts timeline + refunds history — 设计文档 G5 payments 生命周期查询
  const attemptsQ = useQuery({
    queryKey: ["commerce", "attempts", id!, attemptsPage, attemptsPageSize],
    queryFn: () => listAttemptsByPayment(id!, { page: attemptsPage, pageSize: attemptsPageSize }),
    enabled: !!id,
  });
  const refundsQ = useQuery({
    queryKey: ["commerce", "refunds", id!, refundsPage, refundsPageSize],
    queryFn: () => listRefundsByPayment(id!, { page: refundsPage, pageSize: refundsPageSize }),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        state: detailQuery.data.state,
      });
      invoiceForm.setFieldsValue({
        personName: detailQuery.data.personName,
        personIdCard: detailQuery.data.personIdCard,
        personEmail: detailQuery.data.personEmail,
        personPhone: detailQuery.data.personPhone,
        invoiceType: detailQuery.data.invoiceType,
        invoiceTitle: detailQuery.data.invoiceTitle,
        invoiceTaxId: detailQuery.data.invoiceTaxId,
        invoiceRemark: detailQuery.data.invoiceRemark,
      });
    }
  }, [detailQuery.data, form, invoiceForm]);

  const updateMutation = useMutation({
    mutationFn: async (values: PaymentFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamPayment(detailQuery.data.id, {
        displayName: values.displayName,
        state: values.state,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.payments.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamPayment(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.payments.all });
      navigate("/payments");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const invoiceMutation = useMutation({
    mutationFn: async (values: InvoiceFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return invoicePayment(detailQuery.data.id, {
        ...values,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("invoiceIssuedSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.payments.all });
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(
          invoiceForm,
          error,
          buildServerFieldResolver<InvoiceFormValues>(invoiceForm),
          tc,
        );
      }
      message.error(localizedError(error, tc, "error"));
    },
  });

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
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
        <Alert
          type="error"
          showIcon
          title={tc("paymentNotFound")}
          description={tc("paymentNotFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/payments")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tc("paymentDetailTitle", { name: data.displayName || data.name })}
      description={tc("editPaymentDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/payments")}
      onFinish={async (values) => {
        // The shared shell owns validation, first-error focus, the submit lock and
        // field-error mapping; awaiting the mutation keeps the lock for the request.
        await updateMutation.mutateAsync(values as PaymentFormValues);
      }}
      submitPermission={{ permission: "payment:write", resource: "payments" }}
      extra={
        <PermissionGuard action="payment:delete" resource="payments" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tc("paymentDeleteDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("type")}>{data.type}</Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={data.state} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("user")}>{data.user || tc("none")}</Descriptions.Item>
          <Descriptions.Item label={tc("paymentMethod")}>
            {data.providerName || data.provider || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("product")}>
            {data.productName || data.product || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("price")}>
            {data.price} {data.currency}
          </Descriptions.Item>
          <Descriptions.Item label={tc("payUrl")} span={{ xs: 1, sm: 2 }}>
            {data.payUrl || tc("none")}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tc("editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="state">
          {/* Only the transitions the registry declares are offered: a payment is
              written by the gateway, so the console may not walk it backwards. */}
          <Select options={allowedTargets(PAYMENT_STATES, data.state, PAYMENT_TRANSITIONS, tc)} />
        </Form.Item>
      </FormSection>

      <FormSection title={tc("invoiceInfo")} collapsible defaultCollapsed>
        <div>
          <Form
            form={invoiceForm}
            layout="vertical"
            onFinish={async (values: InvoiceFormValues) => {
              try {
                await invoiceMutation.mutateAsync(values);
              } catch {
                /* handled by mutation onError */
              }
            }}
          >
            <Form.Item label={tc("personName")} name="personName">
              <Input placeholder={tc("personNamePlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("personIdCard")} name="personIdCard">
              <Input placeholder={tc("personIdCardPlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("personEmail")} name="personEmail">
              <Input placeholder={tc("personEmailPlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("personPhone")} name="personPhone">
              <Input placeholder={tc("personPhonePlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("invoiceType")} name="invoiceType">
              <Select
                allowClear
                placeholder={tc("invoiceTypePlaceholder")}
                options={[
                  { label: tc("invoiceTypeIndividual"), value: "Individual" },
                  { label: tc("invoiceTypeEnterprise"), value: "Enterprise" },
                  { label: tc("invoiceTypeSpecial"), value: "Special" },
                ]}
              />
            </Form.Item>
            <Form.Item label={tc("invoiceTitle")} name="invoiceTitle">
              <Input placeholder={tc("invoiceTitlePlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("invoiceTaxId")} name="invoiceTaxId">
              <Input placeholder={tc("invoiceTaxIdPlaceholder")} />
            </Form.Item>
            <Form.Item label={tc("invoiceRemark")} name="invoiceRemark">
              <Input.TextArea rows={2} placeholder={tc("invoiceRemarkPlaceholder")} />
            </Form.Item>
            <Form.Item>
              <PermissionGuard action="payment:write" resource="payments" fallback={null}>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={invoiceMutation.isPending}
                  icon={<FileTextOutlined />}
                >
                  {tc("issueInvoice")}
                </Button>
              </PermissionGuard>
              {data.invoiceUrl && (
                <Button style={{ marginLeft: 8 }} href={data.invoiceUrl} target="_blank">
                  {tc("downloadInvoice")}
                </Button>
              )}
            </Form.Item>
          </Form>
        </div>
      </FormSection>

      {/* ── Attempts Timeline (G5 payments 生命周期查询) ───────────── */}
      <FormSection
        title={tc("payments.attemptsTimeline", "Attempts Timeline")}
        badge={
          attemptsQ.data?.page && attemptsQ.data.page.total > 0
            ? attemptsQ.data.page.total
            : undefined
        }
        collapsible
        defaultCollapsed
      >
        <div>
          <AttemptsTable
            attempts={attemptsQ.data?.items ?? []}
            loading={attemptsQ.isLoading}
            pagination={{
              current: attemptsPage,
              pageSize: attemptsPageSize,
              total: attemptsQ.data?.page?.total ?? 0,
              onChange: (page, pageSize) => {
                setAttemptsPage(page);
                setAttemptsPageSize(pageSize);
              },
            }}
          />
        </div>
      </FormSection>

      {/* ── Refunds History ────────────────────────────────────────── */}
      <FormSection
        title={tc("payments.refundsHistory", "Refunds History")}
        badge={
          refundsQ.data?.page && refundsQ.data.page.total > 0 ? refundsQ.data.page.total : undefined
        }
        collapsible
        defaultCollapsed
      >
        <div>
          <RefundsTable
            refunds={refundsQ.data?.items ?? []}
            loading={refundsQ.isLoading}
            pagination={{
              current: refundsPage,
              pageSize: refundsPageSize,
              total: refundsQ.data?.page?.total ?? 0,
              onChange: (page, pageSize) => {
                setRefundsPage(page);
                setRefundsPageSize(pageSize);
              },
            }}
          />
        </div>
      </FormSection>
    </ResourceFormPage>
  );
}

// =============================================================================
// Sub components
// =============================================================================

function AttemptsTable({
  attempts,
  loading,
  pagination,
}: {
  attempts: AttemptListItem[];
  loading: boolean;
  pagination: TablePaginationConfig;
}) {
  const [tc] = useTranslation("common");

  if (loading) {
    return <Skeleton active paragraph={{ rows: 3 }} />;
  }
  if (attempts.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={tc("payments.noAttempts", "No channel attempts recorded for this payment")}
      />
    );
  }

  const stateTag = (s: string) => {
    const map: Record<string, { color: string; icon: ReactNode }> = {
      succeeded: { color: "green", icon: <CheckCircleOutlined /> },
      failed: { color: "red", icon: <CloseCircleOutlined /> },
      pending: { color: "gold", icon: <ClockCircleOutlined /> },
      processing: { color: "blue", icon: <SyncOutlined spin /> },
      unknown: { color: "default", icon: <MinusCircleOutlined /> },
    };
    const cfg = map[s] ?? { color: "default", icon: <ExperimentOutlined /> };
    return (
      <Tag color={cfg.color}>
        {cfg.icon} {s}
      </Tag>
    );
  };

  const columns: ColumnsType<AttemptListItem> = [
    {
      title: tc("payments.attemptNo", "#"),
      dataIndex: "attemptNo",
      width: 60,
      align: "center",
      render: (n: number) => <strong>#{n}</strong>,
    },
    {
      title: tc("payments.state", "State"),
      dataIndex: "state",
      width: 120,
      render: (s: string) => stateTag(s),
    },
    {
      title: tc("payments.channel", "Channel"),
      dataIndex: "channel",
      width: 140,
    },
    {
      title: tc("payments.channelOrderNo", "Channel Order No."),
      dataIndex: "channelOrderNo",
      width: 200,
      render: (v: string | null) => v ?? <Text type="secondary">—</Text>,
    },
    {
      title: tc("payments.amount", "Amount"),
      width: 120,
      align: "right",
      render: (_, r) =>
        r.amount !== null && r.amount !== undefined
          ? `${r.amount.toFixed(2)} ${r.currency ?? ""}`
          : "—",
    },
    {
      title: tc("payments.error", "Error"),
      width: 200,
      render: (_, r) =>
        r.errorCode ? (
          <Tooltip title={r.errorMessage ?? ""}>
            <Tag color="red">{r.errorCode}</Tag>
          </Tooltip>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
    {
      title: tc("payments.latencyMs", "Latency"),
      width: 100,
      align: "right",
      render: (v: number | null) => (v !== null ? `${v}ms` : "—"),
    },
    {
      title: tc("payments.occurredAt", "Occurred"),
      dataIndex: "occurredAt",
      width: 180,
      render: (d: string) => (
        <Space>
          <ClockCircleOutlined />
          <Text style={{ fontSize: 12 }}>{new Date(d).toLocaleString()}</Text>
        </Space>
      ),
    },
  ];

  return (
    <Table
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={attempts}
      scroll={{ x: 1120 }}
      pagination={{ ...pagination, showSizeChanger: true }}
    />
  );
}

function RefundsTable({
  refunds,
  loading,
  pagination,
}: {
  refunds: RefundListItem[];
  loading: boolean;
  pagination: TablePaginationConfig;
}) {
  const [tc] = useTranslation("common");

  if (loading) {
    return <Skeleton active paragraph={{ rows: 3 }} />;
  }
  if (refunds.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={tc("payments.noRefunds", "No refunds recorded for this payment")}
      />
    );
  }

  const stateTag = (s: string) => {
    const map: Record<string, { color: string; icon: ReactNode }> = {
      succeeded: { color: "green", icon: <CheckCircleOutlined /> },
      processing: { color: "blue", icon: <SyncOutlined spin /> },
      requested: { color: "gold", icon: <ClockCircleOutlined /> },
      failed: { color: "red", icon: <CloseCircleOutlined /> },
      cancelled: { color: "default", icon: <MinusCircleOutlined /> },
    };
    const cfg = map[s] ?? { color: "default", icon: <ExperimentOutlined /> };
    return (
      <Tag color={cfg.color}>
        {cfg.icon} {s}
      </Tag>
    );
  };

  const columns: ColumnsType<RefundListItem> = [
    {
      title: tc("payments.outRefundNo", "Out Refund No."),
      dataIndex: "outRefundNo",
      width: 180,
    },
    {
      title: tc("payments.channel", "Channel"),
      dataIndex: "channel",
      width: 140,
    },
    {
      title: tc("payments.state", "State"),
      dataIndex: "state",
      width: 120,
      render: (s: string) => stateTag(s),
    },
    {
      title: tc("payments.amount", "Amount"),
      width: 120,
      align: "right",
      render: (_, r) => (
        <Text type="danger">
          -{r.amount.toFixed(2)} {r.currency}
        </Text>
      ),
    },
    {
      title: tc("payments.reason", "Reason"),
      dataIndex: "reason",
      width: 160,
      render: (v: string) => v || <Text type="secondary">—</Text>,
    },
    {
      title: tc("payments.error", "Error"),
      width: 200,
      render: (_, r) =>
        r.errorCode ? (
          <Tooltip title={r.errorMessage ?? ""}>
            <Tag color="red">{r.errorCode}</Tag>
          </Tooltip>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
    {
      title: tc("payments.requestedAt", "Requested"),
      dataIndex: "requestedAt",
      width: 180,
      render: (d: string) => new Date(d).toLocaleString(),
    },
    {
      title: tc("payments.completedAt", "Completed"),
      dataIndex: "completedAt",
      width: 180,
      render: (d: string | null) =>
        d ? new Date(d).toLocaleString() : <Text type="secondary">—</Text>,
    },
  ];

  return (
    <Table
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={refunds}
      scroll={{ x: 1280 }}
      pagination={{ ...pagination, showSizeChanger: true }}
    />
  );
}

import {
  App,
  Button,
  DatePicker,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  Timeline,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useState } from "react";
import dayjs, { type Dayjs } from "dayjs";
import { useTranslation } from "react-i18next";
import {
  approveAccessRequest,
  createAccessRequest,
  getAccessRequest,
  listAccessRequests,
  provisionAccessRequest,
  rejectAccessRequest,
  startPrivilegedSession,
  submitAccessRequest,
  withdrawAccessRequest,
  type AccessRequestDetail,
  type AccessRequestItem,
  type AccessRequestStage,
  type AccessRequestStatus,
} from "@/shared/api/iam/iga";
import { getApplicationOptions } from "@/shared/api/iam/application";
import { getRoleOptions } from "@/shared/api/iam/role";
import { queryKeys } from "@/shared/api/query-keys";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { ResourceListCard, type ServerPagination } from "@/shared/ui/ResourceListCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useSessionStore } from "@/features/session/session-store";

type ActionType = "submit" | "withdraw" | "approve" | "reject" | "provision" | "elevate";

type PendingAction = {
  type: ActionType;
  record: AccessRequestItem;
};

type CreateFormValues = {
  roleId: string;
  applicationId: string;
  enforcerId?: string;
  resource: string;
  action: string;
  reason?: string;
  window: [Dayjs, Dayjs];
};

const STATUS_COLORS: Record<string, string> = {
  submitted: "default",
  pending_manager: "gold",
  pending_sap: "orange",
  approved: "blue",
  provisioned: "green",
  rejected: "red",
  withdrawn: "default",
};

const STATUS_LABEL_KEYS: Record<AccessRequestStatus, string> = {
  submitted: "iga.requests.statusSubmitted",
  pending_manager: "iga.requests.statusPendingManager",
  pending_sap: "iga.requests.statusPendingSap",
  approved: "iga.requests.statusApproved",
  provisioned: "iga.requests.statusProvisioned",
  rejected: "iga.requests.statusRejected",
  withdrawn: "iga.requests.statusWithdrawn",
};

const ACTION_LABEL_KEYS: Record<ActionType, string> = {
  submit: "iga.requests.submit",
  withdraw: "iga.requests.withdraw",
  approve: "iga.requests.approve",
  reject: "iga.requests.reject",
  provision: "iga.requests.provision",
  elevate: "iga.requests.elevate",
};

const ACTION_TITLE_KEYS: Record<ActionType, string> = {
  submit: "iga.requests.actionTitleSubmit",
  withdraw: "iga.requests.actionTitleWithdraw",
  approve: "iga.requests.actionTitleApprove",
  reject: "iga.requests.actionTitleReject",
  provision: "iga.requests.actionTitleProvision",
  elevate: "iga.requests.actionTitleElevate",
};

const ACTION_CONFIRM_KEYS: Record<ActionType, string> = {
  submit: "iga.requests.confirmSubmit",
  withdraw: "iga.requests.confirmWithdraw",
  approve: "iga.requests.confirmApprove",
  reject: "iga.requests.confirmReject",
  provision: "iga.requests.confirmProvision",
  elevate: "iga.requests.confirmElevate",
};

const STAGE_LABEL_KEYS: Record<AccessRequestStage, string> = {
  manager: "iga.requests.stageManager",
  sap: "iga.requests.stageSap",
};

const STATUS_OPTIONS = (Object.keys(STATUS_LABEL_KEYS) as AccessRequestStatus[]).map((status) => ({
  value: status,
  labelKey: STATUS_LABEL_KEYS[status],
}));

function formatDateTime(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export default function IgaAccessRequestsTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const organizationId = useSessionStore((state) => state.organization?.id ?? undefined);
  // Stable account id from `/auth/account`; used to decide whether the signed-in
  // user owns an access request (withdraw / submit / elevate).
  const currentAccountId = useSessionStore((state) => state.account?.id);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<string | undefined>();
  const [createOpen, setCreateOpen] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const [comment, setComment] = useState("");
  const [form] = Form.useForm<CreateFormValues>();

  const query = useQuery({
    queryKey: ["iga", "access-requests", page, pageSize, status],
    queryFn: () => listAccessRequests({ page, pageSize, status: status as AccessRequestStatus }),
    enabled: !!organizationId,
  });

  const listQuery = {
    ...query,
    data: query.data?.items ?? [],
  } as unknown as UseQueryResult<AccessRequestItem[], Error>;

  const serverPagination: ServerPagination = {
    page,
    pageSize,
    total: query.data?.page?.total ?? 0,
    onChange: (nextPage, nextPageSize) => {
      setPage(nextPage);
      setPageSize(nextPageSize);
    },
  };

  const detailQuery = useQuery({
    queryKey: ["iga", "access-request", detailId],
    queryFn: () => getAccessRequest(detailId as string),
    enabled: !!detailId,
  });

  const roleOptions = useQuery({
    queryKey: queryKeys.roles.options(organizationId),
    queryFn: () => getRoleOptions(organizationId),
    enabled: !!organizationId,
  });

  const applicationOptions = useQuery({
    queryKey: queryKeys.applications.options(organizationId),
    queryFn: () => getApplicationOptions(organizationId),
    enabled: !!organizationId,
  });

  const invalidateRequests = () =>
    queryClient.invalidateQueries({ queryKey: ["iga", "access-requests"] });

  const invalidateDetail = (id: string) =>
    queryClient.invalidateQueries({ queryKey: ["iga", "access-request", id] });

  const createMutation = useMutation({
    mutationFn: createAccessRequest,
    onSuccess: () => {
      void invalidateRequests();
      message.success(tc("iga.requests.createSuccess"));
      setCreateOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<CreateFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const actionMutation = useMutation({
    mutationFn: async ({
      type,
      record,
      comment: actionComment,
    }: PendingAction & { comment: string }) => {
      switch (type) {
        case "submit":
          return submitAccessRequest(record.id, {
            revision: record.revision,
            comment: actionComment,
          });
        case "withdraw":
          return withdrawAccessRequest(record.id, {
            revision: record.revision,
            comment: actionComment,
          });
        case "approve":
          return approveAccessRequest(record.id, {
            stage: record.status === "pending_manager" ? "manager" : "sap",
            revision: record.revision,
            comment: actionComment,
          });
        case "reject":
          return rejectAccessRequest(record.id, {
            stage: record.status === "pending_manager" ? "manager" : "sap",
            revision: record.revision,
            comment: actionComment,
          });
        case "provision":
          return provisionAccessRequest(record.id, { revision: record.revision });
        case "elevate":
          return startPrivilegedSession({ requestId: record.id, revision: record.revision });
      }
    },
    onSuccess: (_, variables) => {
      void invalidateRequests();
      void invalidateDetail(variables.record.id);
      void queryClient.invalidateQueries({ queryKey: ["iga", "privileged-sessions"] });
      message.success(tc("iga.requests.actionSuccess"));
      setPendingAction(null);
      setComment("");
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const openAction = (type: ActionType, record: AccessRequestItem) => {
    setComment("");
    setPendingAction({ type, record });
  };

  const submitCreate = () => {
    void form.validateFields().then((values) => {
      const [start, end] = values.window;
      createMutation.mutate({
        roleId: values.roleId,
        applicationId: values.applicationId,
        enforcerId: values.enforcerId || undefined,
        resource: values.resource,
        action: values.action,
        reason: values.reason,
        startsAt: start.toISOString(),
        expiresAt: end.toISOString(),
      });
      return undefined;
    });
  };

  const detail: AccessRequestDetail | undefined = detailQuery.data;
  const detailRequest = detail?.request;

  return (
    <>
      <ResourceListCard<AccessRequestItem>
        title={tc("nav.iga.requests")}
        description={tc("iga.requests.description")}
        query={listQuery}
        rowKey="id"
        serverPagination={serverPagination}
        tableId="iga-access-requests"
        tableScrollX="max-content"
        filters={[
          {
            key: "status",
            label: tc("status"),
            options: STATUS_OPTIONS.map((option) => ({
              value: option.value,
              label: tc(option.labelKey),
            })),
          },
        ]}
        filterValues={{ status: status ?? "" }}
        onFilterChange={(next) => {
          setStatus(next.status || undefined);
          setPage(1);
        }}
        emptyContent={
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{tc("iga.requests.emptyTitle")}</Typography.Text>
                <Typography.Text type="secondary">
                  {tc("iga.requests.emptyDescription")}
                </Typography.Text>
              </Space>
            }
          >
            <PermissionGuard action="iga:write" resource="iga" fallback={null}>
              <Button type="primary" onClick={() => setCreateOpen(true)}>
                {tc("iga.requests.createFirst")}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="iga:write" resource="iga" fallback={null}>
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              {tc("iga.requests.createTitle")}
            </Button>
          </PermissionGuard>
        }
        columns={[
          {
            title: tc("iga.requests.requester"),
            dataIndex: "requesterId",
            key: "requesterId",
            width: 180,
            ellipsis: true,
          },
          {
            title: tc("iga.requests.target"),
            key: "target",
            width: 220,
            render: (_, record) => (
              <Space orientation="vertical" size={0}>
                <Typography.Text ellipsis style={{ maxWidth: 200 }}>
                  {record.resource}
                </Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {record.action} · {record.roleId}
                </Typography.Text>
              </Space>
            ),
          },
          {
            title: tc("status"),
            dataIndex: "status",
            key: "status",
            width: 150,
            render: (value: AccessRequestStatus) => (
              <Tag color={STATUS_COLORS[value] ?? "default"}>
                {tc(STATUS_LABEL_KEYS[value] ?? STATUS_LABEL_KEYS.submitted)}
              </Tag>
            ),
          },
          {
            title: tc("iga.requests.windowColumn"),
            key: "window",
            width: 190,
            render: (_, record) => (
              <Space orientation="vertical" size={0}>
                <Typography.Text style={{ fontSize: 12 }}>
                  {formatDateTime(record.startsAt)}
                </Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {formatDateTime(record.expiresAt)}
                </Typography.Text>
              </Space>
            ),
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            width: 170,
            render: (value: string) => formatDateTime(value),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 250,
            fixed: "right",
            render: (_, record) => {
              // Fail-closed: without a stable account id we never treat the record
              // as the caller's own request (no fallback to `true`).
              const isRequester = currentAccountId
                ? record.requesterId === currentAccountId
                : false;
              const canSubmit = record.status === "submitted" && isRequester;
              const canWithdraw =
                ["submitted", "pending_manager", "pending_sap"].includes(record.status) &&
                isRequester;
              const canApprove =
                record.status === "pending_manager" || record.status === "pending_sap";
              const canProvision = record.status === "approved";
              const canElevate = record.status === "provisioned" && isRequester;

              return (
                <Space size={0} wrap>
                  <Button type="link" size="small" onClick={() => setDetailId(record.id)}>
                    {tc("iga.requests.viewDetail")}
                  </Button>
                  <PermissionGuard action="iga:write" resource="iga" fallback={null}>
                    {canSubmit && (
                      <Button type="link" size="small" onClick={() => openAction("submit", record)}>
                        {tc("iga.requests.submit")}
                      </Button>
                    )}
                    {canApprove && (
                      <>
                        <Button
                          type="link"
                          size="small"
                          onClick={() => openAction("approve", record)}
                        >
                          {tc("iga.requests.approve")}
                        </Button>
                        <Button
                          type="link"
                          size="small"
                          danger
                          onClick={() => openAction("reject", record)}
                        >
                          {tc("iga.requests.reject")}
                        </Button>
                      </>
                    )}
                    {canWithdraw && (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => openAction("withdraw", record)}
                      >
                        {tc("iga.requests.withdraw")}
                      </Button>
                    )}
                    {canProvision && (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => openAction("provision", record)}
                      >
                        {tc("iga.requests.provision")}
                      </Button>
                    )}
                    {canElevate && (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => openAction("elevate", record)}
                      >
                        {tc("iga.requests.elevate")}
                      </Button>
                    )}
                  </PermissionGuard>
                </Space>
              );
            },
          },
        ]}
      />

      <Modal
        open={!!pendingAction}
        title={pendingAction ? tc(ACTION_TITLE_KEYS[pendingAction.type]) : undefined}
        okText={pendingAction ? tc(ACTION_LABEL_KEYS[pendingAction.type]) : undefined}
        okButtonProps={{
          danger: pendingAction?.type === "reject" || pendingAction?.type === "withdraw",
        }}
        confirmLoading={actionMutation.isPending}
        onCancel={() => setPendingAction(null)}
        onOk={() => {
          if (!pendingAction) return;
          actionMutation.mutate({ ...pendingAction, comment });
        }}
      >
        {pendingAction && (
          <Space orientation="vertical" size={12} style={{ width: "100%" }}>
            <Typography.Text type="secondary">
              {tc(ACTION_CONFIRM_KEYS[pendingAction.type])}
            </Typography.Text>
            {pendingAction.type !== "provision" && pendingAction.type !== "elevate" && (
              <Input.TextArea
                rows={3}
                value={comment}
                onChange={(event) => setComment(event.target.value)}
                placeholder={tc("iga.requests.commentPlaceholder")}
                aria-label={tc("iga.requests.commentLabel")}
              />
            )}
          </Space>
        )}
      </Modal>

      <Modal
        open={createOpen}
        title={tc("iga.requests.createTitle")}
        width={640}
        okText={tc("create")}
        confirmLoading={createMutation.isPending}
        onCancel={() => {
          setCreateOpen(false);
          form.resetFields();
        }}
        onOk={submitCreate}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label={tc("iga.requests.roleLabel")}
            name="roleId"
            rules={[{ required: true }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={(roleOptions.data ?? []).map((role) => ({
                label: role.displayName || role.name,
                value: role.id,
              }))}
              loading={roleOptions.isLoading}
            />
          </Form.Item>
          <Form.Item
            label={tc("iga.requests.applicationLabel")}
            name="applicationId"
            rules={[{ required: true }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={(applicationOptions.data ?? []).map((application) => ({
                label: application.displayName || application.slug,
                value: application.id,
              }))}
              loading={applicationOptions.isLoading}
            />
          </Form.Item>
          <Form.Item label={tc("iga.requests.enforcerLabel")} name="enforcerId">
            <Input placeholder={tc("iga.requests.enforcerPlaceholder")} />
          </Form.Item>
          <Form.Item
            label={tc("iga.requests.resourceLabel")}
            name="resource"
            rules={[{ required: true }]}
          >
            <Input placeholder={tc("iga.requests.resourcePlaceholder")} />
          </Form.Item>
          <Form.Item
            label={tc("iga.requests.actionLabel")}
            name="action"
            rules={[{ required: true }]}
          >
            <Input placeholder={tc("iga.requests.actionPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("iga.requests.reasonLabel")} name="reason">
            <Input.TextArea rows={3} placeholder={tc("iga.requests.reasonPlaceholder")} />
          </Form.Item>
          <Form.Item
            label={tc("iga.requests.windowLabel")}
            name="window"
            rules={[
              { required: true, message: tc("iga.requests.windowRequired") },
              {
                validator: async (_, value?: [Dayjs, Dayjs]) => {
                  if (!value?.[1]) return;
                  if (value[0] && !value[1].isAfter(value[0])) {
                    throw new Error(tc("iga.requests.windowHint"));
                  }
                  if (!value[1].isAfter(dayjs())) {
                    throw new Error(tc("iga.requests.windowHint"));
                  }
                },
              },
            ]}
            extra={tc("iga.requests.windowHint")}
          >
            <DatePicker.RangePicker showTime style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        open={!!detailId}
        title={tc("iga.requests.detailTitle")}
        size={560}
        onClose={() => setDetailId(null)}
        loading={detailQuery.isLoading}
      >
        {detailRequest && (
          <Space orientation="vertical" size={20} style={{ width: "100%" }}>
            <Descriptions
              bordered
              size="small"
              column={1}
              items={[
                {
                  key: "status",
                  label: tc("status"),
                  children: (
                    <Tag color={STATUS_COLORS[detailRequest.status] ?? "default"}>
                      {tc(STATUS_LABEL_KEYS[detailRequest.status] ?? STATUS_LABEL_KEYS.submitted)}
                    </Tag>
                  ),
                },
                {
                  key: "requester",
                  label: tc("iga.requests.requester"),
                  children: detailRequest.requesterId,
                },
                {
                  key: "target",
                  label: tc("iga.requests.target"),
                  children: `${detailRequest.resource} · ${detailRequest.action}`,
                },
                {
                  key: "role",
                  label: tc("iga.requests.roleLabel"),
                  children: detailRequest.roleId,
                },
                {
                  key: "application",
                  label: tc("iga.requests.applicationLabel"),
                  children: detailRequest.applicationId,
                },
                {
                  key: "window",
                  label: tc("iga.requests.windowLabel"),
                  children: `${formatDateTime(detailRequest.startsAt)} → ${formatDateTime(
                    detailRequest.expiresAt,
                  )}`,
                },
                {
                  key: "reason",
                  label: tc("iga.requests.reasonLabel"),
                  children: detailRequest.reason || "-",
                },
                {
                  key: "approvalChain",
                  label: tc("iga.requests.approvalChain"),
                  children: (
                    <Space size={4} wrap>
                      {(detail?.approvalChain ?? []).map((stage) => (
                        <Tag key={stage}>{tc(STAGE_LABEL_KEYS[stage])}</Tag>
                      ))}
                    </Space>
                  ),
                },
                {
                  key: "approvers",
                  label: tc("iga.requests.approvers"),
                  children: (
                    <Space orientation="vertical" size={0}>
                      <Typography.Text style={{ fontSize: 12 }}>
                        {tc("iga.requests.stageManager")}: {detailRequest.managerApproverId || "-"}
                      </Typography.Text>
                      <Typography.Text style={{ fontSize: 12 }}>
                        {tc("iga.requests.stageSap")}: {detailRequest.sapApproverId || "-"}
                      </Typography.Text>
                    </Space>
                  ),
                },
              ]}
            />
            <div>
              <Typography.Title level={5}>{tc("iga.requests.transcript")}</Typography.Title>
              {detail?.messages?.length ? (
                <Timeline
                  items={detail.messages.map((entry) => ({
                    key: entry.id,
                    children: (
                      <Space orientation="vertical" size={2}>
                        <Typography.Text strong>{entry.action}</Typography.Text>
                        {entry.content && (
                          <Typography.Text type="secondary">{entry.content}</Typography.Text>
                        )}
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          {entry.authorId} · {formatDateTime(entry.createdAt)}
                        </Typography.Text>
                      </Space>
                    ),
                  }))}
                />
              ) : (
                <Typography.Text type="secondary">{tc("iga.requests.noMessages")}</Typography.Text>
              )}
            </div>
          </Space>
        )}
      </Drawer>
    </>
  );
}

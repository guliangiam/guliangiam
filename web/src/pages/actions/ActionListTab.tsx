import { App, Button, Empty, Form, Input, Modal, Popconfirm, Select, Space, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  createAction,
  listActions,
  updateActionStatus,
  runAction,
  type ActionItem,
  type ActionStatus,
  type CreateActionRequest,
} from "@/shared/api/iam/actions";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";

const statusColor: Record<string, string> = {
  draft: "default",
  active: "green",
  disabled: "orange",
  deleted: "red",
};

const TRIGGER_TYPES = ["pre_login", "pre_token", "post_registration", "consent", "mfa"] as const;

type ActionFormValues = CreateActionRequest;

export default function ActionListTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [runTarget, setRunTarget] = useState<ActionItem | null>(null);
  const [form] = Form.useForm<ActionFormValues>();
  const [runForm] = Form.useForm<{ input?: string }>();

  const query = useQuery({
    queryKey: ["actions", "list"],
    queryFn: listActions,
    select: (data) => data?.items ?? [],
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["actions"] });

  const createMutation = useMutation({
    mutationFn: (values: ActionFormValues) => createAction(values),
    onSuccess: () => {
      void invalidate();
      message.success(tc("created"));
      setCreateOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<ActionFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: ActionStatus }) =>
      updateActionStatus(id, status),
    onSuccess: () => {
      void invalidate();
      message.success(tc("updateSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const runMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input?: Record<string, unknown> }) =>
      runAction(id, { input }),
    onSuccess: () => {
      void invalidate();
      message.success(tc("identityActions.runQueued"));
      setRunTarget(null);
      runForm.resetFields();
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const submitRun = () => {
    void runForm
      .validateFields()
      .then((values) => {
        let input: Record<string, unknown> | undefined;
        if (values.input?.trim()) input = JSON.parse(values.input) as Record<string, unknown>;
        runMutation.mutate({ id: runTarget?.id ?? "", input });
        return undefined;
      })
      .catch(() => undefined);
  };

  return (
    <>
      <ResourceListCard<ActionItem>
        title={tc("nav.actions.list", "Identity Actions")}
        description={tc(
          "actions.list.description",
          "Sandboxed identity automation actions — create, version, and run custom identity workflows.",
        )}
        query={query}
        rowKey="id"
        tableId="actions-list"
        emptyContent={
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tc("noData")}>
            <PermissionGuard action="actions:write" resource="actions" fallback={null}>
              <Button type="primary" onClick={() => setCreateOpen(true)}>
                {tc("create")}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="actions:write" resource="actions" fallback={null}>
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              {tc("create")}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("actions.name", "Name"), dataIndex: "name", key: "name" },
          {
            title: tc("identityActions.triggerType"),
            dataIndex: "triggerType",
            key: "triggerType",
            render: (v: string) => <Tag>{v}</Tag>,
          },
          {
            title: tc("status"),
            dataIndex: "status",
            key: "status",
            render: (v: string) => <Tag color={statusColor[v] ?? "default"}>{v}</Tag>,
          },
          {
            title: tc("identityActions.version"),
            dataIndex: "revision",
            key: "revision",
            render: (v: number) => `v${v ?? 0}`,
          },
          {
            title: tc("updatedAt", "Updated"),
            dataIndex: "updatedAt",
            key: "updatedAt",
            render: (v: string) => new Date(v).toLocaleString(),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 240,
            render: (_, record) => (
              <PermissionGuard action="actions:write" resource="actions" fallback={null}>
                <Space size={0} wrap>
                  <Button type="link" size="small" onClick={() => setRunTarget(record)}>
                    {tc("identityActions.run")}
                  </Button>
                  {record.status === "active" ? (
                    <Popconfirm
                      title={tc("identityActions.deactivateConfirm")}
                      okText={tc("identityActions.deactivate")}
                      cancelText={tc("cancel")}
                      okButtonProps={{ danger: true }}
                      onConfirm={() => statusMutation.mutate({ id: record.id, status: "disabled" })}
                    >
                      <Button type="link" size="small" danger>
                        {tc("identityActions.deactivate")}
                      </Button>
                    </Popconfirm>
                  ) : (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => statusMutation.mutate({ id: record.id, status: "active" })}
                    >
                      {tc("identityActions.activate")}
                    </Button>
                  )}
                </Space>
              </PermissionGuard>
            ),
          },
        ]}
      />

      <Modal
        open={createOpen}
        title={tc("identityActions.createTitle")}
        onCancel={() => setCreateOpen(false)}
        onOk={() => {
          void form.validateFields().then((values) => createMutation.mutate(values));
        }}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("name")} name="name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            label={tc("identityActions.triggerType")}
            name="triggerType"
            rules={[{ required: true }]}
          >
            <Select options={TRIGGER_TYPES.map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item label={tc("identityActions.code")} name="code" rules={[{ required: true }]}>
            <Input.TextArea rows={6} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={!!runTarget}
        title={tc("identityActions.runTitle")}
        onCancel={() => setRunTarget(null)}
        onOk={submitRun}
        confirmLoading={runMutation.isPending}
      >
        <Space orientation="vertical" style={{ width: "100%" }} size={8}>
          <span>{runTarget?.name}</span>
          <Form form={runForm} layout="vertical">
            <Form.Item
              label={tc("identityActions.runInput")}
              name="input"
              rules={[
                {
                  validator: (_, value: string | undefined) => {
                    if (!value?.trim()) return Promise.resolve();
                    let parsed: unknown;
                    try {
                      parsed = JSON.parse(value);
                    } catch {
                      return Promise.reject(new Error(tc("identityActions.runInputInvalidJson")));
                    }
                    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
                      return Promise.reject(new Error(tc("identityActions.runInputInvalidObject")));
                    }
                    return Promise.resolve();
                  },
                },
              ]}
            >
              <Input.TextArea rows={4} placeholder={tc("identityActions.runInputPlaceholder")} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </>
  );
}

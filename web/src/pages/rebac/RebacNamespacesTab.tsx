import { App, Button, Empty, Form, Input, Modal, Popconfirm, Space, Typography } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  createReBACNamespace,
  deleteReBACNamespace,
  listReBACNamespaces,
  type CreateReBACNamespaceRequest,
  type ReBACNamespaceItem,
} from "@/shared/api/iam/rebac";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { jsonObjectFieldRule } from "@/shared/lib/json-field-rules";
import { formatDateTime } from "@/shared/lib/date-time";

type NamespaceFormValues = {
  name: string;
  displayName?: string;
  relationDefs?: string;
};

export default function RebacNamespacesTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<NamespaceFormValues>();

  const query = useQuery({
    queryKey: ["rebac", "namespaces"],
    queryFn: listReBACNamespaces,
    select: (data) => data?.items ?? [],
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["rebac", "namespaces"] });

  const createMutation = useMutation({
    mutationFn: (input: CreateReBACNamespaceRequest) => createReBACNamespace(input),
    onSuccess: () => {
      void invalidate();
      message.success(tc("created"));
      setOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<NamespaceFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteReBACNamespace(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["rebac", "namespaces"] });
      message.success(tc("deleteSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const submit = () => {
    void form.validateFields().then((values) => {
      let relationDefs: Record<string, unknown> | undefined;
      if (values.relationDefs?.trim()) {
        relationDefs = JSON.parse(values.relationDefs) as Record<string, unknown>;
      }
      createMutation.mutate({
        name: values.name,
        displayName: values.displayName,
        relationDefs,
      });
      return undefined;
    });
  };

  return (
    <>
      <ResourceListCard<ReBACNamespaceItem>
        title={tc("rebac.namespaces.title")}
        description={tc("rebac.namespaces.description")}
        searchPlaceholder={tc("rebac.searchPlaceholder")}
        searchableColumns={[{ dataIndex: "name", label: tc("name") }]}
        query={query}
        rowKey="id"
        emptyContent={
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{tc("noData")}</Typography.Text>
                <Typography.Text type="secondary">
                  {tc("rebac.namespaces.description")}
                </Typography.Text>
              </Space>
            }
          >
            <PermissionGuard action="rebac:write" resource="rebac" fallback={null}>
              <Button type="primary" onClick={() => setOpen(true)}>
                {tc("create")}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="rebac:write" resource="rebac" fallback={null}>
            <Button type="primary" onClick={() => setOpen(true)}>
              {tc("create")}
            </Button>
          </PermissionGuard>
        }
        columns={[
          {
            title: tc("name"),
            dataIndex: "name",
            key: "name",
            sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
          },
          {
            title: tc("rebac.displayName"),
            dataIndex: "displayName",
            key: "displayName",
            render: (v?: string) => v || "-",
          },
          {
            title: tc("rebac.relations"),
            dataIndex: "relationDefs",
            key: "relationDefs",
            render: (v?: Record<string, unknown>) =>
              v && Object.keys(v).length ? Object.keys(v).join(", ") : "-",
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v?: string) => formatDateTime(v),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 100,
            render: (_, record) => (
              <PermissionGuard action="rebac:write" resource="rebac" fallback={null}>
                <Popconfirm
                  title={tc("confirmDelete")}
                  okText={tc("delete")}
                  cancelText={tc("cancel")}
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteMutation.mutate(record.id)}
                >
                  <Button
                    type="link"
                    size="small"
                    danger
                    loading={deleteMutation.isPending && deleteMutation.variables === record.id}
                  >
                    {tc("delete")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={tc("rebac.namespaces.createTitle")}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("name")} name="name" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.namespaces.namePlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.displayName")} name="displayName">
            <Input />
          </Form.Item>
          <Form.Item
            label={tc("rebac.relationDefs")}
            name="relationDefs"
            rules={[jsonObjectFieldRule(tc("invalidJson"))]}
          >
            <Input.TextArea rows={4} placeholder={tc("rebac.relationDefsPlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

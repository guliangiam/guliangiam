import {
  App,
  Button,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  checkReBAC,
  createReBACTuple,
  deleteReBACTuple,
  expandReBAC,
  listReBACNamespaces,
  listReBACTuples,
  type CreateReBACTupleRequest,
  type ReBACExpandNode,
  type ReBACTupleItem,
} from "@/shared/api/iam/rebac";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { formatDateTime } from "@/shared/lib/date-time";

const EMPTY_TUPLE: CreateReBACTupleRequest = {
  subject: "",
  relation: "",
  object: "",
};

function renderExpandTree(node: ReBACExpandNode, depth = 0): ReactNode {
  return (
    <Space
      key={`${depth}-${node.subject}`}
      orientation="vertical"
      size={0}
      style={{ paddingLeft: 12 }}
    >
      <Typography.Text>
        {node.subject}
        {node.subjectRelation ? `#${node.subjectRelation}` : ""}
      </Typography.Text>
      {(node.children ?? []).map((child) => renderExpandTree(child, depth + 1))}
    </Space>
  );
}

export default function RebacTuplesTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [namespaceId, setNamespaceId] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [checkOpen, setCheckOpen] = useState(false);
  const [expandOpen, setExpandOpen] = useState(false);
  const [checkResult, setCheckResult] = useState<boolean | null>(null);
  const [expandResult, setExpandResult] = useState<ReBACExpandNode | null>(null);
  const [form] = Form.useForm<CreateReBACTupleRequest>();
  const [checkForm] = Form.useForm<{ subject: string; relation: string; object: string }>();
  const [expandForm] = Form.useForm<{ relation: string; object: string }>();

  const namespacesQuery = useQuery({
    queryKey: ["rebac", "namespaces"],
    queryFn: listReBACNamespaces,
  });

  const tuplesQuery = useQuery({
    queryKey: ["rebac", "tuples", namespaceId],
    queryFn: () => listReBACTuples(namespaceId!),
    enabled: !!namespaceId,
    select: (data) => data?.items ?? [],
  });

  const createMutation = useMutation({
    mutationFn: (input: CreateReBACTupleRequest) =>
      namespaceId ? createReBACTuple(namespaceId, input) : Promise.reject("no namespace"),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["rebac", "tuples"] });
      message.success(tc("created"));
      setOpen(false);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(
          form,
          err,
          buildServerFieldResolver<CreateReBACTupleRequest>(form),
          tc,
        );
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteReBACTuple(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["rebac", "tuples"] });
      message.success(tc("deleteSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const checkMutation = useMutation({
    mutationFn: (input: { subject: string; relation: string; object: string }) =>
      checkReBAC(namespaceId!, input),
    onSuccess: (result) => setCheckResult(result?.allowed ?? false),
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const expandMutation = useMutation({
    mutationFn: (input: { relation: string; object: string }) => expandReBAC(namespaceId!, input),
    onSuccess: (result) => setExpandResult(result?.root ?? null),
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const namespaceOptions =
    namespacesQuery.data?.items.map((ns) => ({
      value: ns.id,
      label: ns.displayName ? `${ns.name} (${ns.displayName})` : ns.name,
    })) ?? [];

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <span>{tc("rebac.selectNamespace")}:</span>
        <Select
          style={{ width: 300 }}
          placeholder={tc("rebac.selectNamespacePlaceholder")}
          value={namespaceId ?? undefined}
          onChange={setNamespaceId}
          options={namespaceOptions}
          loading={namespacesQuery.isLoading}
        />
        {namespaceId && (
          <PermissionGuard action="rebac:write" resource="rebac" fallback={null}>
            <Button type="primary" onClick={() => setOpen(true)}>
              {tc("rebac.createTuple")}
            </Button>
          </PermissionGuard>
        )}
        {namespaceId && (
          <PermissionGuard action="rebac:check" resource="rebac" fallback={null}>
            <Button
              onClick={() => {
                setCheckResult(null);
                setCheckOpen(true);
              }}
            >
              {tc("rebac.check.title")}
            </Button>
            <Button
              onClick={() => {
                setExpandResult(null);
                setExpandOpen(true);
              }}
            >
              {tc("rebac.expand.title")}
            </Button>
          </PermissionGuard>
        )}
      </Space>

      {namespaceId ? (
        <ResourceListCard<ReBACTupleItem>
          title={tc("rebac.tuples.title")}
          description={tc("rebac.tuples.description")}
          query={tuplesQuery}
          rowKey="id"
          emptyContent={
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                <Space orientation="vertical" size={4}>
                  <Typography.Text strong>{tc("noData")}</Typography.Text>
                  <Typography.Text type="secondary">
                    {tc("rebac.tuples.description")}
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
          columns={[
            { title: tc("rebac.subject"), dataIndex: "subject", key: "subject" },
            {
              title: tc("rebac.subjectRelation"),
              dataIndex: "subjectRelation",
              key: "subjectRelation",
              render: (v?: string | null) => v || "-",
            },
            { title: tc("rebac.relation"), dataIndex: "relation", key: "relation" },
            { title: tc("rebac.object"), dataIndex: "object", key: "object" },
            {
              title: tc("createdAt"),
              dataIndex: "createdAt",
              key: "createdAt",
              render: (v: string) => formatDateTime(v),
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
      ) : null}

      <Modal
        open={open}
        title={tc("rebac.createTupleTitle")}
        onCancel={() => setOpen(false)}
        onOk={() => {
          void form
            .validateFields()
            .then((values) => createMutation.mutate({ ...EMPTY_TUPLE, ...values }));
        }}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("rebac.subject")} name="subject" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.subjectPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.relation")} name="relation" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.relationPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.object")} name="object" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.objectPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.subjectRelation")} name="subjectRelation">
            <Input placeholder={tc("rebac.subjectRelationPlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={checkOpen}
        title={tc("rebac.check.title")}
        onCancel={() => setCheckOpen(false)}
        onOk={() => {
          void checkForm.validateFields().then((values) => checkMutation.mutate(values));
        }}
        confirmLoading={checkMutation.isPending}
      >
        <Form form={checkForm} layout="vertical">
          <Form.Item label={tc("rebac.subject")} name="subject" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.subjectPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.relation")} name="relation" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.relationPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.object")} name="object" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.objectPlaceholder")} />
          </Form.Item>
        </Form>
        {checkResult !== null && (
          <Tag color={checkResult ? "green" : "red"}>
            {checkResult ? tc("rebac.check.allowed") : tc("rebac.check.denied")}
          </Tag>
        )}
      </Modal>

      <Modal
        open={expandOpen}
        title={tc("rebac.expand.title")}
        onCancel={() => setExpandOpen(false)}
        onOk={() => {
          void expandForm.validateFields().then((values) => expandMutation.mutate(values));
        }}
        confirmLoading={expandMutation.isPending}
      >
        <Form form={expandForm} layout="vertical">
          <Form.Item label={tc("rebac.relation")} name="relation" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.relationPlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("rebac.object")} name="object" rules={[{ required: true }]}>
            <Input placeholder={tc("rebac.objectPlaceholder")} />
          </Form.Item>
        </Form>
        {expandResult && (
          <div style={{ marginTop: 12, maxHeight: 240, overflow: "auto" }}>
            {renderExpandTree(expandResult)}
          </div>
        )}
      </Modal>
    </>
  );
}

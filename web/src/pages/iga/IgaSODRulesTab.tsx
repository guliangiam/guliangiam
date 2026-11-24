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
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  createSODRule,
  deleteSODRule,
  listSODRules,
  updateSODRule,
  type CreateSODRuleRequest,
  type SODRuleItem,
} from "@/shared/api/iam/iga";
import { getRoleOptions } from "@/shared/api/iam/role";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { useSessionStore } from "@/features/session/session-store";

type SodFormValues = CreateSODRuleRequest;

export default function IgaSODRulesTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const organizationId = useSessionStore((state) => state.organization?.id ?? undefined);
  const [editing, setEditing] = useState<SODRuleItem | null>(null);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<SodFormValues>();

  const query = useQuery({
    queryKey: ["iga", "sod-rules"],
    queryFn: listSODRules,
    select: (data) => data?.items ?? [],
  });
  const roleOptions = useQuery({
    queryKey: ["roles", "options", organizationId],
    queryFn: () => getRoleOptions(organizationId),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["iga", "sod-rules"] });

  const saveMutation = useMutation({
    mutationFn: (values: SodFormValues) =>
      editing ? updateSODRule(editing.id, values) : createSODRule(values),
    onSuccess: () => {
      void invalidate();
      message.success(editing ? tc("updateSuccess") : tc("created"));
      closeModal();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<SodFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:operationFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteSODRule(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["iga", "sod-rules"] });
      message.success(tc("deleteSuccess"));
    },
    onError: (err) => message.error(serverErrorMessage(err, tc, "common:operationFailed")),
  });

  const options = (roleOptions.data ?? []).map((role) => ({
    label: `${role.displayName || role.name}`,
    value: role.id,
  }));

  function closeModal() {
    setOpen(false);
    setEditing(null);
    form.resetFields();
  }

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (record: SODRuleItem) => {
    setEditing(record);
    form.setFieldsValue({ name: record.name, roleAId: record.roleAId, roleBId: record.roleBId });
    setOpen(true);
  };

  return (
    <>
      <ResourceListCard<SODRuleItem>
        title={tc("nav.iga.sodRules")}
        description={tc("iga.sodRules.description")}
        query={query}
        rowKey="id"
        emptyContent={
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{tc("noData")}</Typography.Text>
                <Typography.Text type="secondary">{tc("iga.sodRules.description")}</Typography.Text>
              </Space>
            }
          >
            <PermissionGuard action="iga:write" resource="iga" fallback={null}>
              <Button type="primary" onClick={openCreate}>
                {tc("create")}
              </Button>
            </PermissionGuard>
          </Empty>
        }
        toolbar={
          <PermissionGuard action="iga:write" resource="iga" fallback={null}>
            <Button type="primary" onClick={openCreate}>
              {tc("create")}
            </Button>
          </PermissionGuard>
        }
        columns={[
          { title: tc("name"), dataIndex: "name", key: "name" },
          { title: tc("iga.sod.roleA"), dataIndex: "roleAId", key: "roleAId" },
          {
            title: tc("iga.sod.roleB"),
            dataIndex: "roleBId",
            key: "roleBId",
            render: (v: string) => <Tag color="red">{v}</Tag>,
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v: string) => new Date(v).toLocaleDateString(),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 160,
            render: (_, record) => (
              <PermissionGuard action="iga:write" resource="iga" fallback={null}>
                <Space size={0} wrap>
                  <Button type="link" size="small" onClick={() => openEdit(record)}>
                    {tc("edit")}
                  </Button>
                  <Popconfirm
                    title={tc("confirmDelete")}
                    okText={tc("delete")}
                    cancelText={tc("cancel")}
                    okButtonProps={{ danger: true }}
                    onConfirm={() => deleteMutation.mutate(record.id)}
                  >
                    <Button type="link" size="small" danger loading={deleteMutation.isPending}>
                      {tc("delete")}
                    </Button>
                  </Popconfirm>
                </Space>
              </PermissionGuard>
            ),
          },
        ]}
      />

      <Modal
        forceRender
        open={open}
        title={editing ? tc("iga.sodRules.editTitle") : tc("iga.sodRules.createTitle")}
        onCancel={closeModal}
        onOk={() => {
          void form.validateFields().then((values) => saveMutation.mutate(values));
        }}
        confirmLoading={saveMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item label={tc("name")} name="name" rules={[{ required: true }]}>
            <Input placeholder={tc("iga.sodRules.namePlaceholder")} />
          </Form.Item>
          <Form.Item label={tc("iga.sod.roleA")} name="roleAId" rules={[{ required: true }]}>
            <Select options={options} showSearch optionFilterProp="label" />
          </Form.Item>
          <Form.Item
            label={tc("iga.sod.roleB")}
            name="roleBId"
            dependencies={["roleAId"]}
            rules={[
              { required: true },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  // 与后端 service.go 的 roleAId != roleBId 校验对齐，前置交叉校验避免提交后才 422。
                  if (!value || getFieldValue("roleAId") !== value) return Promise.resolve();
                  return Promise.reject(
                    new Error(tc("iga.sod.rolesMustDiffer", "Role A and Role B must differ")),
                  );
                },
              }),
            ]}
          >
            <Select options={options} showSearch optionFilterProp="label" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

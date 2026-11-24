import { useEffect } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Result,
  Skeleton,
  Switch,
  Table,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamMcpServer, getIamMcpServer, patchIamMcpServer } from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { ApplicationSelect } from "@/shared/ui/ApplicationSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import {
  buildMcpServerFormValues,
  buildMcpServerPatchPayload,
  type McpServerFormValues,
} from "./mcp-server-patch-payload";
import { localizedError } from "@/shared/lib/server-error";

export default function McpServerDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<McpServerFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.mcpServers.detail(id!),
    queryFn: () => getIamMcpServer(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      // 凭据不回显（读接口只返回 tokenSet/tokenMasked）：token 输入框不预填
      form.setFieldsValue(buildMcpServerFormValues(detailQuery.data));
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: McpServerFormValues) => {
      if (!detailQuery.data) throw new Error(tr("mcp.dataNotLoaded"));
      return patchIamMcpServer(
        detailQuery.data.id,
        buildMcpServerPatchPayload(values, detailQuery.data.revision),
      );
    },
    onSuccess: () => {
      message.success(tr("mcp.updated"));
      void queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "mcp.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("mcp.dataNotLoaded"));
      return deleteIamMcpServer(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("mcp.deleted"));
      void queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      navigate("/ai/mcp-servers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "mcp.deleteFailed"));
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
        <Alert type="error" showIcon title={tc("notFound")} description={tc("notFound")} />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/ai/mcp-servers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("mcp.detailTitle", { name: detailQuery.data.displayName || detailQuery.data.name })}
      description={tr("mcp.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/ai/mcp-servers")}
      onFinish={async (values: McpServerFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      submitPermission={{ permission: "mcp-server:write", resource: "mcp-servers" }}
      extra={
        <PermissionGuard action="mcp-server:write" resource="mcp-servers" fallback={null}>
          <Popconfirm
            title={tr("mcp.deleteConfirmTitle")}
            description={tr("mcp.deleteConfirmDesc")}
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
        <Form.Item label={tr("mcp.name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tc("organization")}>
          <span>
            {orgNameMap.get(detailQuery.data.organizationId) || detailQuery.data.organizationId}
          </span>
        </Form.Item>
        <Form.Item label={tr("mcp.enabled")}>
          {detailQuery.data.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
      </FormSection>

      <FormSection title={tr("mcpServer.editSection", "Edit")}>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tr("mcp.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="url">
          <GlFormRow label="URL">
            <Input placeholder="https://mcp-server.example.com/sse" />
          </GlFormRow>
        </Form.Item>
        <Form.Item label={tr("mcp.authToken")}>
          <Tag color={detailQuery.data.tokenSet ? "green" : "default"}>
            {detailQuery.data.tokenSet ? tr("mcp.tokenConfigured") : tr("mcp.tokenNotConfigured")}
          </Tag>
          {detailQuery.data.tokenMasked ? (
            <span style={{ marginLeft: 8 }}>{detailQuery.data.tokenMasked}</span>
          ) : null}
        </Form.Item>
        <Form.Item name="token">
          <GlFormRow label={tr("mcp.authToken")}>
            <Input.Password placeholder={tc("leaveEmptyToKeepCurrent")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="application">
          <GlFormRow label={tr("mcp.application")}>
            <ApplicationSelect allowClear />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("mcp.toolsList", "Tools")}>
        <Table
          size="small"
          pagination={false}
          dataSource={detailQuery.data.tools || []}
          rowKey="name"
          columns={[
            { title: tr("mcp.toolName"), dataIndex: "name", key: "name" },
            { title: tr("mcp.toolDescription"), dataIndex: "description", key: "description" },
            {
              title: tr("mcp.toolAllowed"),
              dataIndex: "isAllowed",
              key: "isAllowed",
              render: (v: boolean) =>
                v ? (
                  <Tag color="green">{tr("mcp.allowed")}</Tag>
                ) : (
                  <Tag color="red">{tr("mcp.denied")}</Tag>
                ),
            },
          ]}
        />
      </FormSection>
    </ResourceFormPage>
  );
}

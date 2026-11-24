import { App, Form, Input, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamMcpServer } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type McpServerFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  url?: string;
  token?: string;
  application?: string;
  isEnabled?: boolean;
};

export default function McpServerCreatePage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<McpServerFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: McpServerFormValues) => {
      return createIamMcpServer({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        url: values.url,
        token: values.token,
        application: values.application,
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers.all });
      message.success(tr("mcpServer.created", "MCP server created"));
      navigate("/ai/mcp-servers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "mcpServer.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("mcpServer.createTitle", "New MCP Server")}
      description={tr(
        "mcpServer.createDesc",
        "Create a new MCP server, configure address, authentication and application binding.",
      )}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/ai/mcp-servers")}
      onFinish={async (values: McpServerFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tr("mcpServer.orgId", "Organization ID")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("mcpServer.nameRequired", "Please enter server name") },
        ]}
      >
        <GlFormRow label={tr("mcpServer.name", "Name")} required>
          <Input placeholder="e.g. mcp-server-1" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          {
            required: true,
            message: tr("mcpServer.displayNameRequired", "Please enter display name"),
          },
        ]}
      >
        <GlFormRow label={tr("mcpServer.displayName", "Display Name")} required>
          <Input placeholder="e.g. Tool Server" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="url">
        <GlFormRow label={tr("mcpServer.serverUrl", "Server URL")}>
          <Input placeholder={tr("mcpServer.serverUrlPlaceholder", "Server address")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="token">
        <GlFormRow label={tr("mcpServer.authToken", "Auth Token")}>
          <Input.Password
            placeholder={tr("mcpServer.authTokenPlaceholder", "Authentication token")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="application">
        <GlFormRow label={tr("mcpServer.application", "Application")}>
          <Input placeholder={tr("mcpServer.appPlaceholder", "Associated application")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tr("mcpServer.enabled", "Enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

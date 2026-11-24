import { App, Form, Input, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createGatewayServer } from "@/shared/api/iam/gateway";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type ServerFormValues = {
  organizationId: string;
  name: string;
  endpoint: string;
  protocol: string;
  status: string;
};

export default function ServerCreatePage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<ServerFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: ServerFormValues) => {
      return createGatewayServer({
        organizationId: values.organizationId,
        name: values.name,
        endpoint: values.endpoint,
        protocol: values.protocol,
        status: values.status,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["gateway-servers"] });
      message.success(tr("gateway.serverCreate.created", "Server created"));
      navigate("/servers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "gateway.serverCreate.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("gateway.serverCreate.title", "New MCP Server")}
      description={tr(
        "gateway.serverCreate.description",
        "Configure MCP Server endpoint, define name, protocol address and runtime status.",
      )}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/servers")}
      onFinish={async (values: ServerFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tr("gateway.serverCreate.orgId", "Organization ID")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("gateway.serverCreate.nameRequired", "Please enter name") },
        ]}
      >
        <GlFormRow label={tr("gateway.serverCreate.name", "Name")} required>
          <Input placeholder="my-mcp-server" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="endpoint"
        rules={[
          {
            required: true,
            message: tr("gateway.serverCreate.endpointRequired", "Please enter endpoint address"),
          },
        ]}
      >
        <GlFormRow label={tr("gateway.serverCreate.endpoint", "Endpoint")} required>
          <Input placeholder="https://mcp.example.com/sse" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="protocol"
        rules={[
          {
            required: true,
            message: tr("gateway.serverCreate.protocolRequired", "Please select protocol"),
          },
        ]}
      >
        <GlFormRow label={tr("gateway.serverCreate.protocol", "Protocol")} required>
          <Select placeholder={tr("gateway.serverCreate.selectProtocol", "Select protocol")}>
            <Select.Option value="sse">SSE</Select.Option>
            <Select.Option value="streamable-http">Streamable HTTP</Select.Option>
            <Select.Option value="stdio">Stdio</Select.Option>
          </Select>
        </GlFormRow>
      </Form.Item>
      <Form.Item name="status" initialValue="active">
        <GlFormRow label={tr("gateway.serverCreate.status", "Status")}>
          <Select>
            <Select.Option value="active">
              {tr("gateway.serverCreate.active", "Active")}
            </Select.Option>
            <Select.Option value="disabled">
              {tr("gateway.serverCreate.disabled", "Disabled")}
            </Select.Option>
          </Select>
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

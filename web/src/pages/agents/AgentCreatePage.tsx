import { App, Form, Input, InputNumber, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamAgent } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type AgentFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  port: number;
};

const AGENT_TYPE_OPTIONS = [
  { label: "gateway-proxy", value: "gateway-proxy" },
  { label: "sync-agent", value: "sync-agent" },
  { label: "protocol-endpoint", value: "protocol-endpoint" },
];

export default function AgentCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<AgentFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: AgentFormValues) => {
      return createIamAgent({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        host: values.host,
        port: values.port,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.agents.all });
      message.success(tr("agent.created"));
      navigate("/ai/agents");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "agent.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("agent.createTitle")}
      description={tr("agent.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/ai/agents")}
      onFinish={async (values: AgentFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("agent.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("agent.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. agent-server" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("agent.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder="e.g. Server Agent" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="type"
        initialValue="gateway-proxy"
        rules={[{ required: true, message: tc("required") }]}
      >
        <GlFormRow label={tc("type")} required>
          <Select options={AGENT_TYPE_OPTIONS} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="host" rules={[{ required: true, message: tr("agent.hostRequired") }]}>
        <GlFormRow label={tr("agent.host")} required>
          <Input placeholder="e.g. agent.example.com" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="port">
        <GlFormRow label={tr("agent.port")}>
          <InputNumber
            placeholder="e.g. 8376"
            min={1}
            max={65535}
            precision={0}
            style={{ width: "100%" }}
          />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

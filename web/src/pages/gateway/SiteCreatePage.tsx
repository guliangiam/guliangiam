import { App, Form, Input, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createGatewaySite } from "@/shared/api/iam/gateway";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type SiteFormValues = {
  organizationId: string;
  name: string;
  host: string;
  status: string;
};

export default function SiteCreatePage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<SiteFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: SiteFormValues) => {
      return createGatewaySite({
        organizationId: values.organizationId,
        name: values.name,
        host: values.host,
        status: values.status,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["gateway-sites"] });
      message.success(tr("gateway.siteCreate.created", "Site created"));
      navigate("/sites");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "gateway.siteCreate.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("gateway.siteCreate.title", "New Site")}
      description={tr(
        "gateway.siteCreate.description",
        "Configure site, define name, host domain and runtime status.",
      )}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/sites")}
      onFinish={async (values: SiteFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tr("gateway.siteCreate.orgId", "Organization ID")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("gateway.siteCreate.nameRequired", "Please enter name") },
        ]}
      >
        <GlFormRow label={tr("gateway.siteCreate.name", "Name")} required>
          <Input placeholder="my-site" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="host"
        rules={[
          {
            required: true,
            message: tr("gateway.siteCreate.hostRequired", "Please enter host domain"),
          },
        ]}
      >
        <GlFormRow label={tr("gateway.siteCreate.host", "Host")} required>
          <Input placeholder="example.com" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="status" initialValue="active">
        <GlFormRow label={tr("gateway.siteCreate.status", "Status")}>
          <Select>
            <Select.Option value="active">
              {tr("gateway.siteCreate.active", "Active")}
            </Select.Option>
            <Select.Option value="disabled">
              {tr("gateway.siteCreate.disabled", "Disabled")}
            </Select.Option>
          </Select>
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

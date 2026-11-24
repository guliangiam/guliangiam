import { App, Form, Input, InputNumber, Select } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamPricing, getIamPlans, getApplicationOptions } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";

type PricingFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  application?: string;
  description?: string;
  plans?: string[];
  trialDuration?: number;
};

export default function PricingCreatePage() {
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PricingFormValues>();

  const plansQuery = useQuery({
    queryKey: ["plans", "iam", "options"],
    // 100 is the server's MaxPageSize; a larger value is rejected with 422.
    queryFn: () => getIamPlans({ pageSize: 100 }),
  });

  const appOptionsQuery = useQuery({
    queryKey: ["application-options"],
    queryFn: () => getApplicationOptions(),
  });

  const mutation = useMutation({
    mutationFn: async (values: PricingFormValues) => {
      return createIamPricing({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        application: values.application ?? "",
        description: values.description,
        plans: values.plans,
        trialDuration: values.trialDuration,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.pricings.all });
      message.success(tr("pricing.created", "Pricing created"));
      navigate("/pricings");
    },
  });

  const fieldLabels: Record<string, string> = {
    organizationId: tr("pricing.orgId", "Organization ID"),
    name: tr("pricing.name", "Name"),
    displayName: tr("pricing.displayName", "Display Name"),
    application: tr("pricing.application", "Application"),
    description: tr("pricing.description", "Description"),
    plans: tr("pricing.plans", "Plans"),
    trialDuration: tr("pricing.trialDays", "Trial Days"),
  };

  return (
    <ResourceFormPage
      title={tr("pricing.createTitle", "New Pricing")}
      description={tr(
        "pricing.createDesc",
        "Create a new pricing plan, associate plans and trial duration.",
      )}
      saving={mutation.isPending}
      form={form}
      fieldLabels={fieldLabels}
      onCancel={() => navigate("/pricings")}
      onFinish={async (values: PricingFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[
          { required: true, message: tr("pricing.orgIdRequired", "Please enter organization ID") },
        ]}
      >
        <GlFormRow label={tr("pricing.orgId", "Organization ID")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("pricing.nameRequired", "Please enter pricing name") },
        ]}
      >
        <GlFormRow label={tr("pricing.name", "Name")} required>
          <Input placeholder={tr("pricing.namePlaceholder", "e.g. pricing-1")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          {
            required: true,
            message: tr("pricing.displayNameRequired", "Please enter display name"),
          },
        ]}
      >
        <GlFormRow label={tr("pricing.displayName", "Display Name")} required>
          <Input placeholder={tr("pricing.displayNamePlaceholder", "e.g. Standard Pricing")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="application">
        <GlFormRow label={tr("pricing.application", "Application")}>
          <Select
            showSearch
            allowClear
            optionFilterProp="label"
            options={appOptionsQuery.data?.map((a) => ({
              label: a.displayName || a.slug,
              value: a.id,
            }))}
            placeholder={tr("pricing.applicationPlaceholder", "Associated application")}
            loading={appOptionsQuery.isLoading}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tr("pricing.description", "Description")}>
          <Input.TextArea
            rows={3}
            placeholder={tr("pricing.descriptionPlaceholder", "Pricing description")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="plans">
        <GlFormRow label={tr("pricing.plans", "Plans")}>
          <Select
            mode="multiple"
            optionFilterProp="label"
            options={
              Array.isArray(plansQuery.data)
                ? plansQuery.data.map((p: { displayName?: string; name: string; id: string }) => ({
                    label: p.displayName || p.name,
                    value: p.id,
                  }))
                : []
            }
            placeholder={tr("pricing.plansPlaceholder", "Select associated plans")}
            loading={plansQuery.isLoading}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="trialDuration">
        <GlFormRow label={tr("pricing.trialDays", "Trial Days")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0" />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

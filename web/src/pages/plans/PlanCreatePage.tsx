import { App, Form, Input, InputNumber, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamPlan } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { CURRENCY_OPTIONS } from "@/shared/constants/currency";
import { PLAN_PERIODS, PLAN_STATES, stateOptions } from "@/shared/constants/enums";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type PlanFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  period?: string;
  state?: string;
  isExclusive?: boolean;
  description?: string;
  price?: number;
  currency?: string;
};

export default function PlanCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PlanFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: PlanFormValues) => {
      return createIamPlan({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        period: values.period,
        state: values.state,
        isExclusive: values.isExclusive ?? false,
        description: values.description,
        price: values.price,
        currency: values.currency,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.plans.all });
      message.success(tr("plan.created"));
      navigate("/plans");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "plan.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("plan.createTitle")}
      description={tr("plan.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/plans")}
      onFinish={async (values: PlanFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("plan.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("plan.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. plan-1" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("plan.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder="e.g. Basic Plan" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="period">
        <GlFormRow label={tr("plan.period")}>
          <Select
            placeholder={tr("plan.periodPlaceholder")}
            allowClear
            options={stateOptions(PLAN_PERIODS, tc)}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="state" initialValue="draft">
        <GlFormRow label={tc("statusLabel")}>
          <Select options={stateOptions(PLAN_STATES, tc)} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isExclusive" valuePropName="checked" initialValue={false}>
        <GlFormRow label={tr("plan.isExclusive")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tc("description")}>
          <Input.TextArea rows={3} placeholder={tc("description")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="price">
        <GlFormRow label={tr("plan.price")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0.00" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="currency">
        <GlFormRow label={tr("plan.currency")}>
          <Select
            placeholder={tc("placeholder.selectType")}
            allowClear
            showSearch
            options={CURRENCY_OPTIONS}
          />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

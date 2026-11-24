import { App, Form, Input, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamSubscription } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { SUBSCRIPTION_STATES, stateOptions } from "@/shared/constants/enums";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PlanSelect } from "@/shared/ui/PlanSelect";
import { PricingSelect } from "@/shared/ui/PricingSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { UserSelect } from "@/shared/ui/UserSelect";
import { localizedError } from "@/shared/lib/server-error";

type SubscriptionFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  plan?: string;
  pricing?: string;
  state?: string;
};

export default function SubscriptionCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SubscriptionFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: SubscriptionFormValues) => {
      return createIamSubscription({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        user: values.user,
        plan: values.plan,
        pricing: values.pricing,
        state: values.state,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.subscriptions.all });
      message.success(tr("subscription.created"));
      navigate("/subscriptions");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "subscription.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("subscription.createTitle")}
      description={tr("subscription.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/subscriptions")}
      onFinish={async (values: SubscriptionFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("subscription.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("subscription.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. sub-1" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("subscription.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder="e.g. My Subscription" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="user">
        <GlFormRow label={tr("subscription.user")}>
          <UserSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="plan">
        <GlFormRow label={tr("subscription.plan")}>
          <PlanSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="pricing">
        <GlFormRow label={tr("subscription.pricing")}>
          <PricingSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="state">
        <GlFormRow label={tr("subscription.state")}>
          <Select allowClear options={stateOptions(SUBSCRIPTION_STATES, tc)} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

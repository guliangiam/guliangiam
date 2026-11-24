import { useEffect } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamPlan, getIamPlan, patchIamPlan } from "@/shared/api/iam";
import { CURRENCY_OPTIONS } from "@/shared/constants/currency";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { StateTag } from "@/shared/ui/StateTag";
import { GlFormRow } from "@/shared/ui/GlForm";
import { PLAN_PERIODS, PLAN_STATES, stateOptions } from "@/shared/constants/enums";
import { PLAN_TRANSITIONS, allowedTargets, isTerminal } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

type PlanFormValues = {
  displayName: string;
  period: string;
  state: string;
  isExclusive: boolean;
  description: string;
  price: number;
  currency: string;
  paymentProviders: string[];
};

export default function PlanDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PlanFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.plans.detail(id!),
    queryFn: () => getIamPlan(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        period: detailQuery.data.period,
        state: detailQuery.data.state,
        isExclusive: detailQuery.data.isExclusive,
        description: detailQuery.data.description,
        price: detailQuery.data.price,
        currency: detailQuery.data.currency,
        paymentProviders: detailQuery.data.paymentProviders ?? [],
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: PlanFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamPlan(detailQuery.data.id, {
        displayName: values.displayName,
        period: values.period,
        state: values.state,
        isExclusive: values.isExclusive,
        description: values.description,
        price: values.price,
        currency: values.currency,
        paymentProviders: values.paymentProviders,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.plans.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamPlan(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.plans.all });
      navigate("/plans");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 4 }} />
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
        <Alert
          type="error"
          showIcon
          title={tr("plan.notFound")}
          description={tr("plan.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/plans")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("plan.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("plan.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/plans")}
      onFinish={async (values: PlanFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("plan.deleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Form.Item label={tc("name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tc("organization")}>
          <span>
            {orgNameMap.get(detailQuery.data.organizationId) || detailQuery.data.organizationId}
          </span>
        </Form.Item>
        <Form.Item label={tr("plan.period")}>
          <Tag color="blue">{tc(`enumValues.${detailQuery.data.period}`)}</Tag>
        </Form.Item>
        <Form.Item label={tc("statusLabel")}>
          <StateTag value={detailQuery.data.state} />
        </Form.Item>
        <Form.Item label={tr("plan.exclusive")}>
          {detailQuery.data.isExclusive ? (
            <Tag color="purple">{tc("yes")}</Tag>
          ) : (
            <Tag color="default">{tc("no")}</Tag>
          )}
        </Form.Item>
      </FormSection>

      <FormSection title={tr("plan.editSection", "Edit")}>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="period">
          <GlFormRow label={tr("plan.period")}>
            <Select allowClear options={stateOptions(PLAN_PERIODS, tc)} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="state">
          <GlFormRow label={tc("statusLabel")}>
            <Select
              disabled={isTerminal(detailQuery.data.state, PLAN_TRANSITIONS)}
              options={allowedTargets(PLAN_STATES, detailQuery.data.state, PLAN_TRANSITIONS, tc)}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isExclusive" valuePropName="checked">
          <GlFormRow label={tr("plan.exclusive")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="description">
          <GlFormRow label={tc("description")}>
            <Input.TextArea rows={3} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="price">
          <GlFormRow label={tc("price")}>
            <InputNumber min={0} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="currency">
          <GlFormRow label={tr("plan.currency")}>
            <Select showSearch options={CURRENCY_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="paymentProviders">
          <GlFormRow label={tr("plan.paymentProviders")}>
            <Select
              mode="multiple"
              options={[
                { label: tr("provider.types.payment.stripe"), value: "Stripe" },
                { label: tr("provider.types.payment.alipay"), value: "Alipay" },
                { label: tr("provider.types.payment.wechatPay"), value: "WeChatPay" },
                { label: tr("provider.types.payment.paypal"), value: "PayPal" },
                { label: tr("provider.types.payment.adyen"), value: "Adyen" },
                { label: tr("provider.types.payment.airWallex"), value: "AirWallex" },
                { label: tr("provider.types.payment.paddle"), value: "Paddle" },
                { label: tr("provider.types.payment.polar"), value: "Polar" },
                { label: tr("provider.types.payment.fastSpring"), value: "FastSpring" },
                { label: tr("provider.types.payment.lemonSqueezy"), value: "LemonSqueezy" },
                { label: tr("provider.types.payment.gc"), value: "GC" },
                { label: tr("provider.types.payment.balance"), value: "Balance" },
              ]}
              placeholder={tc("selectPaymentProviders")}
            />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

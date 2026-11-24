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
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamPricing,
  getIamPricing,
  getIamPlans,
  getApplicationOptions,
  patchIamPricing,
} from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

type PricingFormValues = {
  displayName: string;
  application: string;
  description: string;
  plans: string[];
  trialDuration: number;
};

export default function PricingDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PricingFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: ["pricing", "iam", "detail", id],
    queryFn: () => getIamPricing(id!),
    enabled: !!id,
  });

  const plansQuery = useQuery({
    queryKey: ["plans", "iam", "options"],
    // 100 is the server's MaxPageSize; a larger value is rejected with 422.
    queryFn: () => getIamPlans({ pageSize: 100 }),
  });

  const appOptionsQuery = useQuery({
    queryKey: ["application-options"],
    queryFn: () => getApplicationOptions(),
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        application: detailQuery.data.application,
        description: detailQuery.data.description,
        plans: detailQuery.data.plans,
        trialDuration: detailQuery.data.trialDuration,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: PricingFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamPricing(detailQuery.data.id, {
        displayName: values.displayName,
        description: values.description,
        plans: values.plans,
        trialDuration: values.trialDuration,
        application: values.application,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["pricings"] });
      // 详情键是单数 ["pricing","iam","detail",id]，不失效它 revision 陈旧 → 二次保存 409。
      void queryClient.invalidateQueries({ queryKey: ["pricing"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamPricing(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["pricings"] });
      navigate("/pricings");
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
          title={tr("pricing.notFound")}
          description={tr("pricing.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/pricings")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("pricing.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("pricing.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/pricings")}
      onFinish={async (values: PricingFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("pricing.deleteDesc")}
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
      </FormSection>

      <FormSection title={tr("pricing.editSection", "Edit")}>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="application">
          <GlFormRow label={tc("application")}>
            <Select
              showSearch
              options={appOptionsQuery.data?.map((a) => ({
                label: a.displayName || a.slug,
                value: a.id,
              }))}
              placeholder={tc("selectApplication")}
              loading={appOptionsQuery.isLoading}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="description">
          <GlFormRow label={tc("description")}>
            <Input.TextArea rows={3} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="plans">
          <GlFormRow label={tr("pricing.plans")}>
            <Select
              mode="multiple"
              options={
                Array.isArray(plansQuery.data)
                  ? plansQuery.data.map(
                      (p: { displayName?: string; name: string; id: string }) => ({
                        label: p.displayName || p.name,
                        value: p.id,
                      }),
                    )
                  : []
              }
              placeholder={tc("selectPlans")}
              loading={plansQuery.isLoading}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="trialDuration">
          <GlFormRow label={tr("pricing.trialDurationDays")}>
            <InputNumber min={0} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

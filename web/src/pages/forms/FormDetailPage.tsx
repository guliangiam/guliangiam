import { App, Button, Card, Form, Input, Popconfirm, Result, Select, Skeleton, Switch } from "antd";
import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { getIamForm, patchIamForm, deleteIamForm } from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import type { IamFormDetail } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

type FormFormValues = {
  displayName?: string;
  type?: string;
  properties?: string;
  isEnabled?: boolean;
};

export default function FormDetailPage() {
  const { id } = useParams();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<FormFormValues>();

  const query = useQuery({
    queryKey: queryKeys.forms.detail(id!),
    queryFn: () => getIamForm(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (query.data) {
      const d = query.data as IamFormDetail;
      form.setFieldsValue({
        displayName: d.displayName,
        type: d.type,
        properties: d.properties,
        isEnabled: d.isEnabled,
      });
    }
  }, [query.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: FormFormValues) => {
      if (!query.data) throw new Error(tr("formResource.dataNotLoaded"));
      return patchIamForm(query.data.id, {
        ...values,
        revision: (query.data as IamFormDetail).revision,
      });
    },
    onSuccess: () => {
      message.success(tr("formResource.updated"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.forms.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "formResource.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!query.data) throw new Error(tr("formResource.dataNotLoaded"));
      return deleteIamForm(query.data.id);
    },
    onSuccess: () => {
      message.success(tr("formResource.deleted"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.forms.all });
      navigate("/forms");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "formResource.deleteFailed"));
    },
  });

  if (query.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (query.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(query.error)}
        extra={
          <Button type="primary" onClick={() => query.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  return (
    <ResourceFormPage
      title={tr("formResource.detailTitle", { name: query.data?.displayName || id || "" })}
      description={tr("formResource.editDescription")}
      loading={query.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/forms")}
      onFinish={async (values) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tr("formResource.deleteConfirmTitle")}
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
          <span>{query.data?.name || ""}</span>
        </Form.Item>
        <Form.Item label={tc("organization")}>
          <span>{query.data?.organizationId || ""}</span>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("formResource.editSection", "Edit")}>
        <Form.Item name="displayName">
          <GlFormRow label={tc("displayName")}>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="type">
          <GlFormRow label={tc("type")}>
            <Select
              options={[
                { label: "Registration", value: "registration" },
                { label: "Login", value: "login" },
                { label: "Profile", value: "profile" },
                { label: "Custom", value: "custom" },
              ]}
              placeholder={tc("selectFormType")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="properties"
          rules={[
            {
              validator: (_rule, value?: string) => {
                if (!value || !value.trim()) return Promise.resolve();
                try {
                  JSON.parse(value);
                  return Promise.resolve();
                } catch {
                  return Promise.reject(new Error(tr("formResource.propertiesInvalid")));
                }
              },
            },
          ]}
        >
          <GlFormRow
            label={tr("formResource.properties")}
            extra="Form field definitions in JSON format"
          >
            <Input.TextArea rows={6} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("isEnabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

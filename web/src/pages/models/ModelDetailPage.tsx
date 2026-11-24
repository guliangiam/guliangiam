import { useEffect } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Result,
  Skeleton,
  Space,
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamModel, getIamModel, patchIamModel } from "@/shared/api/iam";
import type { IamModelDetail } from "@/shared/api/iam/model";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { CasbinModelEditor } from "@/shared/ui/CasbinModelEditor";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

type ModelDetail = IamModelDetail & {
  owner?: string;
  description?: string;
};

const MODEL_TEMPLATES: Record<string, string> = {
  RBAC: `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act`,
  ABAC: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act`,
  ACL: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act`,
};

type ModelFormValues = {
  owner?: string;
  displayName: string;
  description?: string;
  modelText: string;
  isEnabled: boolean;
};

export default function ModelDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ModelFormValues>();

  const detailQuery = useQuery({
    queryKey: queryKeys.models.detail(id!),
    queryFn: () => getIamModel(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data as ModelDetail;
      form.setFieldsValue({
        owner: detail.owner ?? "",
        displayName: detail.displayName,
        description: detail.description ?? "",
        modelText: detail.modelText,
        isEnabled: detail.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: ModelFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamModel(detailQuery.data.id, {
        owner: values.owner,
        displayName: values.displayName,
        description: values.description,
        modelText: values.modelText,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["models"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamModel(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["models"] });
      navigate("/models");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
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
          title={tr("model.notFound")}
          description={tr("model.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/models")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("model.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("model.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/models")}
      onFinish={async (values: ModelFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("model.deleteDesc")}
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
        <Form.Item label={tc("statusLabel")}>
          {detailQuery.data.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
      </FormSection>
      <FormSection title={tc("modelBasicInfo")}>
        <Form.Item name="owner">
          <GlFormRow label={tc("organization")}>
            <OrganizationSelect placeholder={tc("selectOrganization")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="description">
          <GlFormRow label={tc("description")}>
            <Input.TextArea rows={2} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>
      <FormSection
        title={tc("modelText")}
        extra={
          <Space>
            {MODEL_TEMPLATES.RBAC && (
              <Button
                size="small"
                onClick={() => {
                  form.setFieldsValue({ modelText: MODEL_TEMPLATES.RBAC });
                }}
              >
                {tc("loadTemplate")}
              </Button>
            )}
          </Space>
        }
      >
        <Form.Item name="modelText" noStyle>
          <CasbinModelEditor />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

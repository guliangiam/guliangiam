import { useEffect } from "react";
import { Alert, App, Button, Card, Form, Input, Popconfirm, Result, Skeleton } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamResource, getIamResource, patchIamResource } from "@/shared/api/iam";
import { getApplication } from "@/shared/api/iam/application";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import { formatFileSize } from "@/shared/lib/file-size";
import { localizedError } from "@/shared/lib/server-error";

// Only `displayName`, `tag`, `filePath`, `fileSize` and `mimeType` are editable
// through `PATCH /api/v1/resources/{id}` (see `PatchResourceRequest`). The form
// used to carry `owner`, `application`, `user`, `parent`, `fileName`,
// `fileFormat`, `url`, `description` and `isEnabled` as well — nine fields the
// API has no counterpart for, so every edit through them was silently discarded.
type ResourceFormValues = {
  displayName: string;
  tag: string;
};

export default function ResourceDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ResourceFormValues>();

  const detailQuery = useQuery({
    queryKey: queryKeys.resources.detail(id!),
    queryFn: () => getIamResource(id!),
    enabled: !!id,
  });

  // 归属应用：按 applicationId 反查显示名，概览展示可读名称而非裸 UUID；
  // 解析失败（应用已删除/无权查看）时回退显示原值。
  const applicationQuery = useQuery({
    queryKey: ["application", detailQuery.data?.applicationId],
    queryFn: () => getApplication(detailQuery.data!.applicationId!),
    enabled: !!detailQuery.data?.applicationId,
    retry: false,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        tag: detailQuery.data.tag ?? "",
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: ResourceFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamResource(detailQuery.data.id, {
        displayName: values.displayName,
        tag: values.tag,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.resources.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamResource(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.resources.all });
      navigate("/resources");
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
          title={tc("resourceNotFound")}
          description={tr("iamResource.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/resources")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("iamResource.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("iamResource.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/resources")}
      onFinish={async (values: ResourceFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("iamResource.deleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview")}>
        <Form.Item label={tc("name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tc("type")}>
          <span>{detailQuery.data.type}</span>
        </Form.Item>
        <Form.Item label={tr("iamResource.application")}>
          <span>{applicationQuery.data?.displayName || detailQuery.data.applicationId || "-"}</span>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("basicInfo")}>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("fileInfo")}>
        <Form.Item label={tr("iamResource.filePath")}>
          <span>{detailQuery.data.filePath || "-"}</span>
        </Form.Item>
        <Form.Item label={tr("iamResource.mimeType")}>
          <span>{detailQuery.data.mimeType || "-"}</span>
        </Form.Item>
        <Form.Item label={tr("iamResource.fileSize")}>
          <span>{formatFileSize(detailQuery.data.fileSize)}</span>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("metadata")}>
        <Form.Item name="tag">
          <GlFormRow label={tr("iamResource.tag")}>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

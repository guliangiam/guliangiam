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
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamFaceIdProvider,
  getIamFaceIdProvider,
  patchIamFaceIdProvider,
} from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import {
  buildFaceIdProviderPatchPayload,
  type FaceIdProviderFormValues,
} from "./face-id-provider-patch-payload";
import { localizedError } from "@/shared/lib/server-error";

export default function FaceIdProviderDetailPage() {
  return (
    <PermissionGuard action="face-id-provider:read" resource="face-id-providers">
      <FaceIdProviderDetailContent />
    </PermissionGuard>
  );
}

function FaceIdProviderDetailContent() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<FaceIdProviderFormValues>();
  // 平台管理员跨组织打开详情时，列表链接携带 ?organizationId=…（对照 useProviderDetail）。
  const [searchParams] = useSearchParams();
  const orgIdFromQuery = searchParams.get("organizationId") || undefined;

  const detailQuery = useQuery({
    queryKey: queryKeys.faceIdProviders.detail(id!),
    queryFn: () => getIamFaceIdProvider(id!, orgIdFromQuery),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        accessKeyId: detailQuery.data.accessKeyId,
        // 凭据不回显（读接口只返回 accessKeySecretSet 标记）：留空表示保持原值
        endpoint: detailQuery.data.endpoint,
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: FaceIdProviderFormValues) => {
      if (!detailQuery.data) throw new Error(tr("faceId.dataNotLoaded"));
      return patchIamFaceIdProvider(
        detailQuery.data.id,
        buildFaceIdProviderPatchPayload(values, detailQuery.data.revision),
        detailQuery.data.organizationId,
      );
    },
    onSuccess: () => {
      message.success(tr("faceId.updated"));
      void queryClient.invalidateQueries({ queryKey: ["face-id-providers"] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "faceId.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("faceId.dataNotLoaded"));
      return deleteIamFaceIdProvider(detailQuery.data.id, detailQuery.data.organizationId);
    },
    onSuccess: () => {
      message.success(tr("faceId.deleted"));
      void queryClient.invalidateQueries({ queryKey: ["face-id-providers"] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
      navigate("/face-id-providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "faceId.deleteFailed"));
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
        <Alert type="error" showIcon title={tc("notFound")} description={tc("notFound")} />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/face-id-providers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("faceId.detailTitle", { name: detailQuery.data.name })}
      description={tr("faceId.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/face-id-providers")}
      submitPermission={{ permission: "face-id-provider:write", resource: "face-id-providers" }}
      onFinish={async (values: FaceIdProviderFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard
          action="face-id-provider:write"
          resource="face-id-providers"
          fallback={null}
        >
          <Popconfirm
            title={tr("faceId.deleteConfirmTitle")}
            description={tr("faceId.deleteConfirmDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
    >
      <Form.Item label={tr("faceId.name")}>
        <span>{detailQuery.data.name}</span>
      </Form.Item>
      <Form.Item label={tr("faceId.providerType")}>
        <span>{detailQuery.data.providerType}</span>
      </Form.Item>
      <Form.Item label={tr("faceId.status")}>
        {detailQuery.data.isEnabled ? (
          <Tag color="green">{tc("enabled")}</Tag>
        ) : (
          <Tag color="red">{tc("disabled")}</Tag>
        )}
      </Form.Item>
      <Form.Item name="accessKeyId">
        <GlFormRow label="AccessKey ID">
          <Input placeholder={tc("accessKeyId")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="accessKeySecret">
        <GlFormRow label="AccessKey Secret">
          <Input.Password placeholder={tc("accessKeySecret")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="endpoint">
        <GlFormRow label={tr("faceId.endpoint")}>
          <Input placeholder={tc("endpoint")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked">
        <GlFormRow label={tc("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}

import { useEffect, useState } from "react";
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
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamCaptchaProvider,
  getIamCaptchaProvider,
  patchIamCaptchaProvider,
} from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import { CaptchaTestModal } from "@/pages/providers/CaptchaTestModal";
import {
  buildCaptchaProviderPatchPayload,
  type CaptchaProviderFormValues,
} from "./captcha-provider-patch-payload";
import {
  CAPTCHA_CONFIG_FIELDS,
  CAPTCHA_CREDENTIAL_LABELS,
} from "@/shared/ui/ProviderFields/captcha-constants";
import { localizedError } from "@/shared/lib/server-error";

export default function CaptchaProviderDetailPage() {
  return (
    <PermissionGuard action="captcha-provider:read" resource="captcha-providers">
      <CaptchaProviderDetailContent />
    </PermissionGuard>
  );
}

function CaptchaProviderDetailContent() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CaptchaProviderFormValues>();
  const [testOpen, setTestOpen] = useState(false);
  // 平台管理员跨组织打开详情时，列表链接携带 ?organizationId=…（对照 useProviderDetail）。
  const [searchParams] = useSearchParams();
  const orgIdFromQuery = searchParams.get("organizationId") || undefined;

  const detailQuery = useQuery({
    queryKey: queryKeys.captchaProviders.detail(id!),
    queryFn: () => getIamCaptchaProvider(id!, orgIdFromQuery),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const configValues: Record<string, unknown> = {};
      for (const spec of CAPTCHA_CONFIG_FIELDS[detailQuery.data.providerType] ?? []) {
        const stored = detailQuery.data.config?.[spec.configKey];
        if (stored !== undefined && stored !== null) configValues[spec.formName] = stored;
      }
      form.setFieldsValue({
        name: detailQuery.data.name,
        siteKey: detailQuery.data.siteKey,
        // 凭据不回显（读接口只返回 secretKeySet 标记）：留空表示保持原值
        isEnabled: detailQuery.data.isEnabled,
        ...configValues,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: CaptchaProviderFormValues) => {
      if (!detailQuery.data) throw new Error(tr("captchaProvider.dataNotLoaded"));
      return patchIamCaptchaProvider(
        detailQuery.data.id,
        buildCaptchaProviderPatchPayload(
          values,
          detailQuery.data.revision,
          detailQuery.data.providerType,
        ),
        detailQuery.data.organizationId,
      );
    },
    onSuccess: () => {
      message.success(tr("captchaProvider.updated"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.captchaProviders.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "captchaProvider.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("captchaProvider.dataNotLoaded"));
      return deleteIamCaptchaProvider(detailQuery.data.id, detailQuery.data.organizationId);
    },
    onSuccess: () => {
      message.success(tr("captchaProvider.deleted"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.captchaProviders.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
      navigate("/captcha-providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "captchaProvider.deleteFailed"));
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
        subTitle={localizedError(detailQuery.error, tc, "common:error")}
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
          title={tc("notFound")}
          description={tc("notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/captcha-providers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <>
      <ResourceFormPage
        title={tr("captchaProvider.detailTitle", { name: detailQuery.data.name })}
        description={tr("captchaProvider.editDescription")}
        loading={detailQuery.isLoading}
        saving={updateMutation.isPending}
        form={form}
        onCancel={() => navigate("/captcha-providers")}
        submitPermission={{ permission: "captcha-provider:write", resource: "captcha-providers" }}
        onFinish={async (values: CaptchaProviderFormValues) => {
          await updateMutation.mutateAsync(values);
        }}
        extra={
          <>
            {/* 验证码提供商走运行时端点测试（统一 test-connection 对该类返回 422），
              原独立「验证码测试」页已融合为本页操作按钮。 */}
            <Button onClick={() => setTestOpen(true)}>{tc("captchaTest")}</Button>
            <PermissionGuard
              action="captcha-provider:write"
              resource="captcha-providers"
              fallback={null}
            >
              <Popconfirm
                title={tr("captchaProvider.deleteConfirmTitle")}
                description={tr("captchaProvider.deleteConfirmDesc")}
                onConfirm={() => deleteMutation.mutate()}
                okText={tc("delete")}
                cancelText={tc("cancel")}
                okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
              >
                <Button danger>{tc("delete")}</Button>
              </Popconfirm>
            </PermissionGuard>
          </>
        }
      >
        <Form.Item
          name="name"
          rules={[{ required: true, message: tr("captchaProvider.nameRequired") }]}
        >
          <GlFormRow label={tc("name")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item label={tr("captchaProvider.providerType")}>
          <span>{detailQuery.data.providerType}</span>
        </Form.Item>
        <Form.Item label={tc("statusLabel")}>
          {detailQuery.data.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
        <Form.Item name="siteKey">
          <GlFormRow
            label={
              CAPTCHA_CREDENTIAL_LABELS[detailQuery.data.providerType]
                ? tc(CAPTCHA_CREDENTIAL_LABELS[detailQuery.data.providerType].idLabelKey)
                : tr("captchaProvider.siteKey")
            }
          >
            <Input placeholder={tc("siteKey")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="secretKey">
          <GlFormRow
            label={
              CAPTCHA_CREDENTIAL_LABELS[detailQuery.data.providerType]
                ? tc(CAPTCHA_CREDENTIAL_LABELS[detailQuery.data.providerType].secretLabelKey)
                : tr("captchaProvider.secretKey")
            }
          >
            <Input.Password placeholder={tc("secretKey")} />
          </GlFormRow>
        </Form.Item>
        {/* 类型相关的 config 扩展字段：密钥类回显掩码（留空/回显=保持），其余明文可编辑。 */}
        {(CAPTCHA_CONFIG_FIELDS[detailQuery.data.providerType] ?? []).map((spec) => (
          <Form.Item key={spec.formName} name={spec.formName}>
            <GlFormRow label={tc(spec.labelKey)}>
              {spec.input === "number" ? (
                <InputNumber min={0} max={1} step={0.05} style={{ width: "100%" }} />
              ) : spec.input === "password" ? (
                <Input.Password autoComplete="new-password" />
              ) : spec.input === "select" ? (
                <Select
                  options={(spec.options ?? []).map((o) => ({
                    label: tc(o.labelKey),
                    value: o.value,
                  }))}
                />
              ) : (
                <Input autoComplete="off" />
              )}
            </GlFormRow>
          </Form.Item>
        ))}
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("isEnabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </ResourceFormPage>
      <CaptchaTestModal
        providerId={detailQuery.data.id}
        providerType={detailQuery.data.providerType}
        organizationId={detailQuery.data.organizationId}
        open={testOpen}
        onClose={() => setTestOpen(false)}
      />
    </>
  );
}

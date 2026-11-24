import { useEffect } from "react";
import { App, Form, Input, Modal, Select } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSessionStore } from "@/features/session/session-store";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { createProvider } from "@/shared/api/iam/provider-extended";
import {
  CONSOLE_REDIRECT_PROVIDER_CATEGORIES,
  CREATABLE_PROVIDER_CATEGORIES,
  PROVIDER_CONSOLE_REDIRECTS,
  useProviderCategories,
} from "@/shared/hooks/useProviderCategories";
import {
  buildCreatePayload,
  createDefaultProvider,
  type ProviderFormValues,
} from "@/shared/hooks/useProviderDetail";
import { DynamicProviderFields } from "@/shared/ui/ProviderFields";
import { localizedError } from "@/shared/lib/server-error";

export type ProviderInlineCreateModalProps = {
  open: boolean;
  onClose: () => void;
  /** 限定可创建的类别（如身份关联场景只允许 OAuth/OIDC/SAML）；缺省开放全部可创建类别。 */
  allowedCategories?: readonly string[];
  defaultCategory?: string;
  /** 创建成功后回传新 provider id，供调用方刷新列表或自动选中。 */
  onCreated?: (providerId: string) => void;
};

/**
 * 跨菜单页面内联新建 Provider 的模态：选类别/类型 → 填凭据 → 保存，
 * 复用统一 /api/v1/providers 创建 API 与 ProviderDetailPage 的表单逻辑
 * （useProviderCategories 类型目录 + DynamicProviderFields 凭据分区 +
 * buildCreatePayload 载荷构造）。托管 kind（captcha/face-id/ldap）由
 * providerTypeOptions 过滤保证不可选，不会被误创建。
 */
export function ProviderInlineCreateModal({
  open,
  onClose,
  allowedCategories,
  defaultCategory = "OAuth",
  onCreated,
}: ProviderInlineCreateModalProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const organization = useSessionStore((state) => state.organization);
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ProviderFormValues>();

  const watchedCategory = Form.useWatch("category", form) ?? defaultCategory;
  const { providerTypeOptions } = useProviderCategories({ category: watchedCategory });

  const orgOptionsQuery = useQuery({
    queryKey: ["organization-options"],
    queryFn: () => getOrganizationOptions(),
    enabled: open,
  });

  // Captcha 只走统一详情页（分流专用端点）：内联弹窗固定调统一 createProvider，
  // 会命中 managed kind 的 422，因此这里排除 Captcha 类别。FaceId 等专用控制台
  // 类别以重定向方式提供（选中即跳转独立创建页），与 ProviderDetailPage 一致。
  const categoryOptions = [
    ...(allowedCategories
      ? CREATABLE_PROVIDER_CATEGORIES.filter((option) => allowedCategories.includes(option.value))
      : [...CREATABLE_PROVIDER_CATEGORIES]
    ).filter((option) => option.value !== "Captcha"),
    ...CONSOLE_REDIRECT_PROVIDER_CATEGORIES.filter(
      (option) => !allowedCategories || allowedCategories.includes(option.value),
    ),
  ];

  useEffect(() => {
    if (!open) return;
    form.resetFields();
    form.setFieldsValue({
      ...createDefaultProvider(organization?.id ?? ""),
      category: categoryOptions.some((option) => option.value === defaultCategory)
        ? defaultCategory
        : categoryOptions[0]?.value,
    });
    // categoryOptions/defaultCategory 变化即重置语义已含在 open 切换里，无需追踪。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, form, organization?.id]);

  const createMutation = useMutation({
    mutationFn: async (): Promise<string> => {
      await form.validateFields();
      // 动态凭据字段没有 Form.Item，由 ProviderFieldBridge 写入 store；
      // 与 ProviderDetailPage 相同，提交前合并整份 store。
      const all = form.getFieldsValue(true) as ProviderFormValues;
      if (["OAuth", "OIDC"].includes(String(all.category)) && !String(all.clientId ?? "").trim()) {
        throw new Error(`${t("clientId")}: ${t("required")}`);
      }
      const { id } = await createProvider(buildCreatePayload(all));
      return id;
    },
    onSuccess: async (id) => {
      await queryClient.invalidateQueries({ queryKey: ["providers"] });
      message.success(tr("provider.providerCreated"));
      onCreated?.(id);
      onClose();
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "provider.saveFailed"));
    },
  });

  return (
    <Modal
      title={tr("provider.newProvider")}
      open={open}
      onCancel={onClose}
      onOk={() => createMutation.mutate()}
      okText={t("create")}
      cancelText={t("cancel")}
      confirmLoading={createMutation.isPending}
      width={640}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          label={t("organization")}
          name="organizationId"
          rules={[{ required: true, message: t("required") }]}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={orgOptionsQuery.data?.map((item) => ({
              label: item.displayName || item.slug,
              value: item.id,
            }))}
          />
        </Form.Item>
        <Form.Item
          label={t("name")}
          name="name"
          rules={[{ required: true, message: t("required") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          label={t("displayName")}
          name="displayName"
          rules={[{ required: true, message: t("required") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          label={t("category")}
          name="category"
          rules={[{ required: true, message: t("required") }]}
        >
          <Select
            options={categoryOptions}
            onChange={(value) => {
              const redirect = PROVIDER_CONSOLE_REDIRECTS[value];
              if (redirect) {
                onClose();
                navigate(redirect);
                return;
              }
              form.setFieldValue("providerType", undefined);
            }}
          />
        </Form.Item>
        <Form.Item
          label={t("type")}
          name="providerType"
          rules={[{ required: true, message: t("required") }]}
        >
          <Select showSearch optionFilterProp="value" options={providerTypeOptions} />
        </Form.Item>
        <DynamicProviderFields category={String(watchedCategory)} form={form} />
      </Form>
    </Modal>
  );
}

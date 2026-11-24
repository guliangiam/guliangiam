import { useMemo, useState } from "react";
import { Button, Form, Space, Tag, Typography } from "antd";
import type { FormInstance } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ProviderInlineCreateModal } from "@/shared/ui/ProviderInlineCreateModal";
import { ProviderItemsEditor, type ProviderSelectOption } from "@/shared/ui/ProviderItemsEditor";
import type { ProviderItem, ApplicationConfigFormValues } from "./types";

/** Bridge component that connects ProviderItemsEditor to Ant Design Form value/onChange pattern */
function ProviderItemsEditorInput({
  value,
  onChange,
  providerOptions,
  organizationId,
}: {
  value?: ProviderItem[];
  onChange?: (value: ProviderItem[]) => void;
  providerOptions: ProviderSelectOption[];
  organizationId?: string;
}) {
  return (
    <ProviderItemsEditor
      value={value ?? []}
      onChange={(v) => onChange?.(v)}
      providerOptions={providerOptions}
      organizationId={organizationId}
    />
  );
}

// signinMethods → 提供商联动建议映射：验证码由 Email/SMS 类提供商发送，
// 社交登录方式对应同名提供商类型；Password 为内置能力，不产生建议。
const CATEGORY_SUGGESTION_BY_METHOD: Record<string, string[]> = {
  "Verification code": ["Email", "SMS"],
  "Face ID": ["FaceId"],
  "Device login": ["Scan"],
};
const TYPE_SUGGESTION_METHODS = new Set(["WeChat", "DingTalk", "Lark"]);

// 稳定空数组：Form.useWatch 未加载时回退，避免每次渲染新建引用触发 memo 失效。
const EMPTY_METHODS: string[] = [];

function isProviderSuggested(
  option: { category?: string; type?: string },
  enabledMethods: string[],
): boolean {
  return enabledMethods.some((method) => {
    const categories = CATEGORY_SUGGESTION_BY_METHOD[method];
    if (categories && option.category && categories.includes(option.category)) return true;
    if (TYPE_SUGGESTION_METHODS.has(method) && option.type?.toLowerCase() === method.toLowerCase())
      return true;
    return false;
  });
}

type ProvidersTabProps = {
  form: FormInstance<ApplicationConfigFormValues>;
  providerOptions: ProviderSelectOption[];
  /** signupGroup 候选组的来源组织（应用所属组织）。 */
  organizationId?: string;
};

export function ProvidersTab({ form, providerOptions, organizationId }: ProvidersTabProps) {
  const { t } = useTranslation();
  const [createProviderOpen, setCreateProviderOpen] = useState(false);

  // 联动建议：读取「认证」分区配置的登录方式，为可支撑的提供商打标记，
  // 仅提示不过滤，避免误导只配置了部分方式的场景。
  const signinMethods =
    (Form.useWatch("signinMethods", form) as string[] | undefined) ?? EMPTY_METHODS;
  const enabledMethods = useMemo(
    () => signinMethods.map((method) => (method ?? "").trim()).filter(Boolean),
    [signinMethods],
  );
  const optionsWithSuggestion = useMemo(
    () =>
      providerOptions.map((option) => ({
        ...option,
        suggested: enabledMethods.length > 0 && isProviderSuggested(option, enabledMethods),
      })),
    [providerOptions, enabledMethods],
  );

  const validateProviderItems = async (_rule: unknown, items: ProviderItem[] = []) => {
    const rows = items ?? [];
    const names = rows.map((item) => (item.provider ?? "").trim());
    if (names.some((name) => !name)) {
      throw new Error(t("resource:application.providerEditor.required"));
    }
    const duplicate = names.find((name, index) => names.indexOf(name) !== index);
    if (duplicate) {
      throw new Error(t("resource:application.providerEditor.duplicate"));
    }
    if (providerOptions.length > 0) {
      const allowed = new Set(providerOptions.map((option) => option.value));
      const unknown = names.find((name) => !allowed.has(name));
      if (unknown) {
        throw new Error(t("resource:application.providerEditor.unknown"));
      }
    }
  };

  return (
    <Space orientation="vertical" size={8} style={{ width: "100%" }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {t("resource:application.providerEditor.hint")}
      </Typography.Paragraph>
      <Space size={8} align="center" wrap>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {enabledMethods.length > 0
            ? t("resource:application.providerEditor.enabledSigninMethods", {
                methods: enabledMethods.join(", "),
              })
            : t("resource:application.providerEditor.noSigninMethods")}
        </Typography.Text>
        {enabledMethods.map((method) => (
          <Tag key={method}>{method}</Tag>
        ))}
      </Space>
      <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
        <Link to="/providers">{t("resource:application.providerEditor.manage")}</Link>
        <PermissionGuard action="provider:write" resource="providers" fallback={null}>
          <Button
            size="small"
            type="link"
            icon={<PlusOutlined />}
            onClick={() => setCreateProviderOpen(true)}
          >
            {t("resource:provider.newProvider")}
          </Button>
        </PermissionGuard>
      </div>
      <Form.Item
        name="providerItems"
        rules={[{ validator: validateProviderItems }]}
        style={{ marginBottom: 0 }}
      >
        <ProviderItemsEditorInput
          providerOptions={optionsWithSuggestion}
          organizationId={organizationId}
        />
      </Form.Item>
      <ProviderInlineCreateModal
        open={createProviderOpen}
        onClose={() => setCreateProviderOpen(false)}
      />
    </Space>
  );
}

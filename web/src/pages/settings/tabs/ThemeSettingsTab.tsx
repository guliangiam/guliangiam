import { useEffect } from "react";
import { Form, Input, Select, message } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import { isHttpError } from "@/shared/api/http";
import { updateOrganizationTheme, type UpdateOrganizationThemePayload } from "@/shared/api/iam";
import { FormSection } from "@/shared/ui/FormSection";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

export function ThemeSettingsTab({
  orgId,
  theme,
  onSuccess,
}: {
  orgId: string;
  theme: Record<string, unknown>;
  revision: number;
  onSuccess: () => void;
}) {
  const { t } = useTranslation("resource");
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  useEffect(() => {
    form.setFieldsValue({
      themeName: theme.themeName ?? "",
      customCss: theme.customCss ?? "",
    });
  }, [form, theme.themeName, theme.customCss]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, unknown>) =>
      updateOrganizationTheme(orgId, values as UpdateOrganizationThemePayload),
    onSuccess: () => {
      message.success(t("settings.saveSuccess"));
      onSuccess();
      void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
    },
    onError: (error: Error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver(form), t);
      }
      message.error(localizedError(error, t, "settings.saveFailed"));
    },
  });

  return (
    <Form
      form={form}
      layout="vertical"
      scrollToFirstError
      onFinish={async (values) => {
        // Await the mutation so the submit button stays locked until the request
        // settles; the mutation's onError already surfaces the failure message.
        try {
          await mutation.mutateAsync(values);
        } catch {
          /* handled by mutation onError */
        }
      }}
    >
      <FormSection
        title={t("organization.themeSection")}
        description={t("organization.themeSectionDesc")}
      >
        <Form.Item label={t("settings.themeName")} name="themeName">
          <Select
            placeholder={t("settings.themeNamePlaceholder")}
            options={[
              { label: "Default", value: "default" },
              { label: "Dark", value: "dark" },
              { label: "Compact", value: "compact" },
            ]}
          />
        </Form.Item>
        <Form.Item label={t("settings.customCss")} name="customCss">
          <Input.TextArea rows={6} placeholder={t("settings.customCssPlaceholder")} />
        </Form.Item>
      </FormSection>
      <Form.Item>
        <button
          type="submit"
          className="ant-btn ant-btn-primary"
          disabled={mutation.isPending}
          aria-label={t("common:save")}
        >
          {t("settings.save")}
        </button>
      </Form.Item>
    </Form>
  );
}

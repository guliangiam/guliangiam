import { useEffect, useMemo } from "react";
import { Form, Input, Select, message } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import { isHttpError } from "@/shared/api/http";
import {
  updateOrganizationSettings,
  type UpdateOrganizationSettingsPayload,
} from "@/shared/api/iam";
import { getCountryCodeData, getCountryFlag } from "@/shared/ui/country-data";
import { toLanguageOptions } from "@/shared/constants/locale-options";
import { FormSection } from "@/shared/ui/FormSection";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

export function GeneralSettingsTab({
  orgId,
  settings,
  onSuccess,
}: {
  orgId: string;
  settings: Record<string, unknown>;
  revision: number;
  onSuccess: () => void;
}) {
  const { t } = useTranslation("resource");
  const { i18n } = useTranslation();
  const [form] = Form.useForm();
  const queryClient = useQueryClient();

  const languages = useMemo(() => (settings.languages ?? []) as string[], [settings.languages]);
  const tags = useMemo(() => (settings.tags ?? []) as string[], [settings.tags]);
  const countryCodes = useMemo(
    () => (settings.countryCodes ?? []) as string[],
    [settings.countryCodes],
  );
  const userTypes = useMemo(() => (settings.userTypes ?? []) as string[], [settings.userTypes]);

  const languageOptions = useMemo(() => toLanguageOptions(t), [t]);
  const countryOptions = useMemo(
    () =>
      getCountryCodeData(undefined, i18n.language).map((country) => ({
        value: country.code,
        label: `${getCountryFlag(country.code)} ${country.name} (+${country.phone})`,
      })),
    [i18n.language],
  );

  useEffect(() => {
    form.setFieldsValue({
      languages,
      defaultApplicationId: settings.defaultApplicationId ?? "",
      countryCodes,
      userTypes,
      tags,
    });
  }, [form, languages, settings.defaultApplicationId, countryCodes, userTypes, tags]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, unknown>) =>
      updateOrganizationSettings(orgId, values as UpdateOrganizationSettingsPayload),
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
        title={t("organization.settingsSection")}
        description={t("organization.settingsSectionDesc")}
      >
        <Form.Item label={t("settings.languages")} name="languages">
          <Select
            mode="multiple"
            allowClear
            showSearch
            optionFilterProp="label"
            options={languageOptions}
            placeholder={t("settings.languagesPlaceholder")}
          />
        </Form.Item>
        <Form.Item label={t("settings.defaultApplicationId")} name="defaultApplicationId">
          <Input placeholder={t("settings.defaultApplicationIdPlaceholder")} />
        </Form.Item>
        <Form.Item label={t("settings.countryCodes")} name="countryCodes">
          <Select
            mode="multiple"
            allowClear
            showSearch
            optionFilterProp="label"
            options={countryOptions}
            placeholder={t("settings.countryCodesPlaceholder")}
          />
        </Form.Item>
        <Form.Item label={t("settings.userTypes")} name="userTypes">
          <Select mode="tags" placeholder={t("settings.userTypesPlaceholder")} />
        </Form.Item>
        <Form.Item label={t("settings.tags")} name="tags">
          <Select mode="tags" placeholder={t("settings.tagsPlaceholder")} />
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

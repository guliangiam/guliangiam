import { useEffect } from "react";
import { Button, Form, Input, Select } from "antd";
import type { Rule } from "antd/es/form";
import { useTranslation } from "react-i18next";
import type { UserDetail, UpdateUserProfilePayload } from "@/shared/api/iam/user-types";
import type { AuthAccountContext } from "@/shared/api/iam/auth";
import type { AccountItem } from "@/shared/api/iam/organization";
import type { UseMutationResult } from "@tanstack/react-query";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { normalizeIsoCountryCode, toDialCode, toE164 } from "@/shared/ui/country-data";
import { FormSection } from "@/shared/ui/FormSection";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import {
  useAccountItems,
  isFieldVisible,
  isFieldDisabled,
  getFieldRegex,
} from "@/features/account-items/useAccountItems";

type AccountProfileSectionProps = {
  account: AuthAccountContext | undefined;
  user: UserDetail | undefined;
  profileForm: ReturnType<typeof Form.useForm>[0];
  updateProfileMutation: UseMutationResult<void, Error, UpdateUserProfilePayload>;
  /** 组织级账户项规则；缺省（未配置/无读取权限）时全部字段可见可编辑。 */
  accountItems?: AccountItem[];
};

export function AccountProfileSection({
  account,
  user,
  profileForm,
  updateProfileMutation,
  accountItems,
}: AccountProfileSectionProps) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const watchedCountryCode = Form.useWatch("countryCode", profileForm) as string | undefined;
  const dialCode = toDialCode(watchedCountryCode ?? "");
  const itemsMap = useAccountItems({ accountItems, viewerRole: "self" });

  // 组织配置的 regex 应用到表单校验；历史脏数据里的非法表达式安全忽略。
  const withPattern = (rules: Rule[], itemName: string): Rule[] => {
    const source = getFieldRegex(itemsMap, itemName);
    if (!source) return rules;
    let pattern: RegExp;
    try {
      pattern = new RegExp(source);
    } catch {
      return rules;
    }
    return [
      ...rules,
      { pattern, message: tc("form.patternInvalid", "Value does not match the required pattern") },
    ];
  };

  useEffect(() => {
    if (!user) return;
    const profile = user.profile;
    const profileCountryCode = typeof profile.countryCode === "string" ? profile.countryCode : "";
    const profilePhone = typeof profile.phone === "string" ? profile.phone : "";
    const countryCode = normalizeIsoCountryCode(profileCountryCode);
    const storedDialCode = toDialCode(countryCode);
    const storedPhone = profilePhone || user.phone || "";
    // Display the local part when the stored E.164 number matches the selected
    // country. Legacy `+86`-style country values stay untouched (and unfilled
    // in the ISO selector) instead of being guessed into a country.
    const localPhone =
      storedDialCode && storedPhone.startsWith(storedDialCode)
        ? storedPhone.slice(storedDialCode.length)
        : storedPhone;
    profileForm.setFieldsValue({
      displayName: profile.displayName ?? user.displayName ?? "",
      email: profile.email ?? user.email ?? "",
      phone: localPhone,
      countryCode,
      region: profile.region ?? "",
      location: profile.location ?? "",
      gender: profile.gender ?? "",
      birthday: profile.birthday ?? "",
      bio: profile.bio ?? "",
      language: profile.language ?? "",
    });
  }, [user, profileForm]);

  const validateCountryCode = async (_: unknown, value: string | undefined) => {
    if (!profileForm.getFieldValue("phone")) return;
    if (!normalizeIsoCountryCode(value)) {
      throw new Error(tc("auth:selectCountryCode"));
    }
  };

  const validatePhone = async (_: unknown, value: string | undefined) => {
    if (!value) return;
    const countryCode = normalizeIsoCountryCode(profileForm.getFieldValue("countryCode"));
    if (!countryCode) {
      throw new Error(tc("auth:selectCountryCode"));
    }
    if (!toE164(countryCode, value)) {
      throw new Error(tc("form.invalidPhone"));
    }
  };

  return (
    <Form
      form={profileForm}
      layout="vertical"
      scrollToFirstError
      onFinish={async (values) => {
        const profileValues = values as UpdateUserProfilePayload;
        const countryCode = normalizeIsoCountryCode(profileValues.countryCode);
        const phone = profileValues.phone ? toE164(countryCode, profileValues.phone) : "";
        if (profileValues.phone && !phone) {
          profileForm.setFields([{ name: "phone", errors: [tc("form.invalidPhone")] }]);
          return;
        }
        // Await the mutation so the submit button stays locked until the request
        // settles; the mutation's onError already surfaces the failure message.
        try {
          // Store numbers as E.164 and keep `countryCode` as ISO alpha-2.
          await updateProfileMutation.mutateAsync({ ...profileValues, countryCode, phone });
        } catch (error) {
          // 裸 Form 没有 ResourceFormPage 的集中标红，这里自行落字段红字；
          // toast 仍由 mutation onError 发出。
          if (isHttpError(error)) {
            applyServerFieldErrors(profileForm, error, buildServerFieldResolver(profileForm), tc);
          }
        }
      }}
    >
      <FormSection title={t("user.basicSection")} description={t("user.basicSectionDesc")}>
        {isFieldVisible(itemsMap, "Username") && (
          <Form.Item label={t("account.username")}>
            <span>{account?.account?.name ?? "N/A"}</span>
          </Form.Item>
        )}
        <Form.Item label={t("account.organization")}>
          <span>{account?.organization?.displayName ?? account?.account?.owner ?? "N/A"}</span>
        </Form.Item>
        {isFieldVisible(itemsMap, "Display name") && (
          <Form.Item
            label={t("account.displayName")}
            name="displayName"
            rules={withPattern(
              [{ required: true, message: t("account.displayNameRequired") }],
              "Display name",
            )}
          >
            <Input disabled={isFieldDisabled(itemsMap, "Display name")} />
          </Form.Item>
        )}
      </FormSection>
      <FormSection title={t("user.profileSection")} description={t("user.profileSectionDesc")}>
        {isFieldVisible(itemsMap, "Email") && (
          <Form.Item label={t("account.email")} name="email" rules={withPattern([], "Email")}>
            <Input type="email" disabled={isFieldDisabled(itemsMap, "Email")} />
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Phone") && (
          <Form.Item
            label={t("account.phone")}
            name="phone"
            rules={withPattern([{ validator: validatePhone }], "Phone")}
          >
            {/* prefix 常驻渲染：账号数据到达后动态增删前后缀会触发 antd 的 DOM 重排警告 */}
            <Input prefix={<span>{dialCode}</span>} disabled={isFieldDisabled(itemsMap, "Phone")} />
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Country/Region") && (
          <Form.Item
            label={tc("auth:countryCode")}
            name="countryCode"
            rules={withPattern([{ validator: validateCountryCode }], "Country/Region")}
          >
            <CountryCodeSelect
              ariaLabel={tc("auth:countryCode")}
              style={{ width: "100%" }}
              disabled={isFieldDisabled(itemsMap, "Country/Region")}
            />
          </Form.Item>
        )}
        <Form.Item label={tc("region", "Region")} name="region">
          <Input />
        </Form.Item>
        {isFieldVisible(itemsMap, "Location") && (
          <Form.Item
            label={t("account.location")}
            name="location"
            rules={withPattern([], "Location")}
          >
            <Input disabled={isFieldDisabled(itemsMap, "Location")} />
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Gender") && (
          <Form.Item label={t("user.gender")} name="gender">
            <Select
              allowClear
              placeholder={t("account.selectGender")}
              disabled={isFieldDisabled(itemsMap, "Gender")}
            >
              <Select.Option value="male">{t("user.genderMale")}</Select.Option>
              <Select.Option value="female">{t("user.genderFemale")}</Select.Option>
              <Select.Option value="other">{t("user.genderOther")}</Select.Option>
            </Select>
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Birthday") && (
          <Form.Item label={t("user.birthday")} name="birthday" rules={withPattern([], "Birthday")}>
            <Input placeholder="YYYY-MM-DD" disabled={isFieldDisabled(itemsMap, "Birthday")} />
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Bio") && (
          <Form.Item label={t("user.bio")} name="bio" rules={withPattern([], "Bio")}>
            <Input.TextArea rows={3} disabled={isFieldDisabled(itemsMap, "Bio")} />
          </Form.Item>
        )}
        {isFieldVisible(itemsMap, "Language") && (
          <Form.Item label={tc("language")} name="language">
            <Select
              allowClear
              placeholder={t("account.selectLanguage")}
              disabled={isFieldDisabled(itemsMap, "Language")}
            >
              <Select.Option value="en">English</Select.Option>
              <Select.Option value="zh">{tc("lang.zhCN", "Chinese")}</Select.Option>
              <Select.Option value="ja">{tc("lang.ja", "Japanese")}</Select.Option>
              <Select.Option value="ko">한국어</Select.Option>
            </Select>
          </Form.Item>
        )}
      </FormSection>
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={updateProfileMutation.isPending}>
          {t("account.saveChanges")}
        </Button>
      </Form.Item>
    </Form>
  );
}

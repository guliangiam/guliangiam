/**
 * Organization 详情页 · 「账户与导航」分区
 *
 * 两个导航树（管理端 / 用户端）共用同一份 `navPreferencesJson` 表单字段，
 * 键名与后端 `extractNavItems` 契约一致（`navItems` / `userNavItems`），
 * 分组顺序写入共享的 `navGroupOrder` 键；故抽出模块级 `NavPreferenceTree`
 * 复用读写逻辑（而不是在同一组件里复制两份 JSON.parse 样板）。
 */
import { Form, Input, Select, Switch } from "antd";
import type { FormInstance } from "antd";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { NavTreeEditor } from "@/shared/ui/NavTreeEditor";
import AccountItemTable from "@/shared/ui/AccountItemTable";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { toLanguageOptions } from "@/shared/constants/locale-options";
import { StructuredJsonField } from "../StructuredJsonEditor";
import type { OrganizationFormValues } from "./types";

type NavPreferenceTreeProps = {
  form: FormInstance<OrganizationFormValues>;
  /** 与后端 extractNavItems 读取的键名严格一致。 */
  preferenceKey: "navItems" | "userNavItems";
  fallbackChecked: string[];
  label: string;
};

function NavPreferenceTree({
  form,
  preferenceKey,
  fallbackChecked,
  label,
}: NavPreferenceTreeProps) {
  // useWatch 保证异步加载的表单值（setFieldsValue）能驱动树勾选态刷新。
  const jsonValue = Form.useWatch("navPreferencesJson", form) as string | undefined;

  const parsePreferences = (value: unknown): Record<string, unknown> => {
    try {
      return JSON.parse((value as string) || "{}") as Record<string, unknown>;
    } catch {
      return {};
    }
  };

  const writePreference = (patch: Record<string, unknown>) => {
    const current = parsePreferences(form.getFieldValue("navPreferencesJson"));
    // 管理端树改写规范键时清理历史键，避免两份数据并存造成歧义。
    if (preferenceKey === "navItems") delete current["adminNavItems"];
    form.setFieldValue("navPreferencesJson", JSON.stringify({ ...current, ...patch }, null, 2));
  };

  const raw = parsePreferences(jsonValue);
  // 历史数据兼容：旧版把管理端菜单写在 adminNavItems 键，读取时回退展示。
  const legacyAdmin =
    preferenceKey === "navItems" && raw["navItems"] === undefined
      ? raw["adminNavItems"]
      : undefined;
  const checkedKeys = ((raw[preferenceKey] ?? legacyAdmin ?? fallbackChecked) as string[]) ?? [];
  const groupOrder = (raw["navGroupOrder"] ?? []) as string[];

  return (
    <Form.Item label={label}>
      <NavTreeEditor
        checkedKeys={checkedKeys}
        onCheck={(checked) => writePreference({ [preferenceKey]: checked })}
        groupOrder={groupOrder}
        onGroupOrderChange={(groupKeys) => writePreference({ navGroupOrder: groupKeys })}
      />
    </Form.Item>
  );
}

type NavigationTabProps = {
  form: FormInstance<OrganizationFormValues>;
};

export function NavigationTab({ form }: NavigationTabProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");

  const languageOptions = useMemo(() => toLanguageOptions(t), [t]);

  return (
    <>
      {/* 注册隐藏字段：useWatch 只对注册字段生效，导航树的勾选态需随异步加载的表单值刷新。 */}
      <Form.Item name="navPreferencesJson" hidden>
        <Input />
      </Form.Item>
      <Form.Item label={tr("organization.languages")} name="languages">
        <Select
          mode="multiple"
          allowClear
          showSearch
          optionFilterProp="label"
          options={languageOptions}
          placeholder={tr("settings.languagesPlaceholder")}
        />
      </Form.Item>
      <Form.Item
        label={tr("organization.countryCodes")}
        name="countryCodes"
        extra={tr("organization.countryCodesHint")}
      >
        {/* 默认带入常用六国（CN/US/SG/GB/DE/IN），其余国家通过搜索添加。 */}
        <CountryCodeSelect
          mode="multiple"
          allowClear
          placeholder={tr("settings.countryCodesPlaceholder")}
        />
      </Form.Item>
      <Form.Item label={tr("organization.userTypes")} name="userTypes">
        <Select mode="tags" tokenSeparators={[","]} />
      </Form.Item>
      <Form.Item label={tr("organization.tags")} name="tags">
        <Select mode="tags" tokenSeparators={[","]} />
      </Form.Item>
      <Form.Item label={tr("organization.accountMenuStyle")} name="accountMenu">
        <Select
          allowClear
          options={[
            { label: tr("organization.menuStyleHorizontal"), value: "Horizontal" },
            { label: tr("organization.menuStyleVertical"), value: "Vertical" },
          ]}
        />
      </Form.Item>
      <Form.Item label={tr("organization.accountItems")} name="accountItems" valuePropName="value">
        <AccountItemTable />
      </Form.Item>
      <NavPreferenceTree
        form={form}
        preferenceKey="navItems"
        fallbackChecked={["all"]}
        label={tr("organization.adminNavItems", "Admin Navigation")}
      />
      <NavPreferenceTree
        form={form}
        preferenceKey="userNavItems"
        fallbackChecked={[]}
        label={tr("organization.userNavItems", "User Navigation")}
      />
      <Form.Item
        label={tr("organization.featureFlagToggle")}
        name="featureFlag"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <StructuredJsonField
        label={tr("organization.featureFlagsExtraJson")}
        name="featureFlagsExtraJson"
        rows={4}
      />
    </>
  );
}

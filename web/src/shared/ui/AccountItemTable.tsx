import { AutoComplete, Button, Input, Select, Space, Switch, Table, Tooltip } from "antd";
import { DeleteOutlined, DownOutlined, PlusOutlined, UpOutlined } from "@ant-design/icons";
import { type FC, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { AccountItem } from "@/shared/api/iam/organization";

type Props = {
  value?: AccountItem[];
  onChange?: (value: AccountItem[]) => void;
};

const ACCOUNT_ITEM_NAMES = [
  { name: "Organization", label: "Organization" },
  { name: "ID", label: "ID" },
  { name: "Name", label: "Name" },
  { name: "Display name", label: "Display name" },
  { name: "First name", label: "First name" },
  { name: "Last name", label: "Last name" },
  { name: "Avatar", label: "Avatar" },
  { name: "User type", label: "User type" },
  { name: "Password", label: "Password" },
  { name: "Email", label: "Email" },
  { name: "Phone", label: "Phone" },
  { name: "Country code", label: "Country code" },
  { name: "Country/Region", label: "Country/Region" },
  { name: "Location", label: "Location" },
  { name: "Address", label: "Address" },
  { name: "Addresses", label: "Addresses" },
  { name: "Affiliation", label: "Affiliation" },
  { name: "Title", label: "Title" },
  { name: "ID card type", label: "ID card type" },
  { name: "ID card", label: "ID card" },
  { name: "ID card info", label: "ID card info" },
  { name: "Real name", label: "Real name" },
  { name: "ID verification", label: "ID verification" },
  { name: "Homepage", label: "Homepage" },
  { name: "Bio", label: "Bio" },
  { name: "Tag", label: "Tag" },
  { name: "Language", label: "Language" },
  { name: "Gender", label: "Gender" },
  { name: "Birthday", label: "Birthday" },
  { name: "Education", label: "Education" },
  { name: "Balance", label: "Balance" },
  { name: "Balance currency", label: "Balance currency" },
  { name: "Balance credit", label: "Balance credit" },
  { name: "Cart", label: "Cart" },
  { name: "Transactions", label: "Transactions" },
  { name: "Score", label: "Score" },
  { name: "Karma", label: "Karma" },
  { name: "Ranking", label: "Ranking" },
  { name: "Signup application", label: "Signup application" },
  { name: "Register type", label: "Register type" },
  { name: "Register source", label: "Register source" },
  { name: "API key", label: "API key" },
  { name: "Groups", label: "Groups" },
  { name: "Roles", label: "Roles" },
  { name: "Permissions", label: "Permissions" },
  { name: "3rd-party logins", label: "3rd-party logins" },
  { name: "Properties", label: "Properties" },
  { name: "Is online", label: "Is online" },
  { name: "Is admin", label: "Is admin" },
  { name: "Is forbidden", label: "Is forbidden" },
  { name: "Is deleted", label: "Is deleted" },
  { name: "Need update password", label: "Need update password" },
  { name: "IP whitelist", label: "IP whitelist" },
  { name: "Multi-factor authentication", label: "Multi-factor authentication" },
  { name: "WebAuthn credentials", label: "WebAuthn credentials" },
  { name: "Last change password time", label: "Last change password time" },
  { name: "Managed accounts", label: "Managed accounts" },
  { name: "Face ID", label: "Face ID" },
  { name: "MFA accounts", label: "MFA accounts" },
  { name: "MFA items", label: "MFA items" },
] as const;

const REGEX_ENABLED_NAMES = new Set([
  "Display name",
  "Password",
  "Email",
  "Phone",
  "Location",
  "Title",
  "Homepage",
  "Bio",
  "Gender",
  "Birthday",
  "Education",
  "ID card",
  "ID card type",
]);

const VIEW_RULE_VALUES = ["Public", "Self", "Admin"] as const;

const MODIFY_RULE_SELF_VALUES = ["Self", "Admin", "Immutable"] as const;

const MODIFY_RULE_ADMIN_ONLY_VALUES = ["Admin", "Immutable"] as const;

const AccountItemTable: FC<Props> = ({ value = [], onChange = () => {} }) => {
  const { t: tr } = useTranslation("resource");
  const usedNames = useMemo(() => new Set(value.map((item) => item.name)), [value]);
  const viewRuleOptions = useMemo(
    () =>
      VIEW_RULE_VALUES.map((v) => ({
        value: v,
        label: tr(`organization.accountItemTable.viewRule.${v}`),
      })),
    [tr],
  );
  const modifyRuleSelfOptions = useMemo(
    () =>
      MODIFY_RULE_SELF_VALUES.map((v) => ({
        value: v,
        label: tr(`organization.accountItemTable.modifyRule.${v}`),
      })),
    [tr],
  );
  const modifyRuleAdminOnlyOptions = useMemo(
    () =>
      MODIFY_RULE_ADMIN_ONLY_VALUES.map((v) => ({
        value: v,
        label: tr(`organization.accountItemTable.modifyRule.${v}`),
      })),
    [tr],
  );

  const availableNameOptions = useMemo(
    () =>
      ACCOUNT_ITEM_NAMES.map((item) => ({
        value: item.name,
        label: item.label,
        disabled: usedNames.has(item.name),
      })),
    [usedNames],
  );

  const updateField = useCallback(
    <K extends keyof AccountItem>(index: number, key: K, val: AccountItem[K]) => {
      const next = [...value];
      next[index] = { ...next[index], [key]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const handleAdd = useCallback(() => {
    const defaultName = ACCOUNT_ITEM_NAMES.find((item) => !usedNames.has(item.name))?.name ?? "";
    onChange([
      ...value,
      {
        name: defaultName,
        visible: true,
        viewRule: "Public",
        modifyRule: "Self",
        tab: "",
        regex: "",
      },
    ]);
  }, [value, onChange, usedNames]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  const handleMoveUp = useCallback(
    (index: number) => {
      if (index === 0) return;
      const next = [...value];
      [next[index - 1], next[index]] = [next[index], next[index - 1]];
      onChange(next);
    },
    [value, onChange],
  );

  const handleMoveDown = useCallback(
    (index: number) => {
      if (index >= value.length - 1) return;
      const next = [...value];
      [next[index], next[index + 1]] = [next[index + 1], next[index]];
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: tr("organization.accountItemTable.columns.name"),
      dataIndex: "name",
      key: "name",
      width: 220,
      render: (_: string, __: AccountItem, index: number) => (
        // AutoComplete 同时支持「从标准字段选择」与「直接输入自定义字段名」。
        // 后端 name 是自由字符串，不需要枚举约束。
        <AutoComplete
          style={{ width: "100%" }}
          value={value[index]?.name}
          options={availableNameOptions.map((option) => ({
            ...option,
            disabled: option.disabled && option.value !== value[index]?.name,
          }))}
          onChange={(val) => updateField(index, "name", val)}
          filterOption={(input, option) =>
            String(option?.label ?? "")
              .toLowerCase()
              .includes(input.toLowerCase())
          }
          placeholder={tr("organization.accountItemTable.namePlaceholder")}
          size="small"
        />
      ),
    },
    {
      title: tr("organization.accountItemTable.columns.visible"),
      dataIndex: "visible",
      key: "visible",
      width: 90,
      render: (_: boolean, __: AccountItem, index: number) => (
        <Switch
          checked={value[index]?.visible}
          onChange={(checked) => updateField(index, "visible", checked)}
          size="small"
        />
      ),
    },
    {
      title: tr("organization.accountItemTable.columns.viewRule"),
      dataIndex: "viewRule",
      key: "viewRule",
      width: 140,
      render: (_: string, record: AccountItem, index: number) => {
        if (!record.visible) return null;
        return (
          <Select
            style={{ width: "100%" }}
            value={value[index]?.viewRule}
            options={viewRuleOptions}
            onChange={(val) => updateField(index, "viewRule", val)}
            size="small"
          />
        );
      },
    },
    {
      title: tr("organization.accountItemTable.columns.modifyRule"),
      dataIndex: "modifyRule",
      key: "modifyRule",
      width: 140,
      render: (_: string, record: AccountItem, index: number) => {
        if (!record.visible) return null;
        const isAdminOnly = record.viewRule === "Admin" || record.name === "Is admin";
        return (
          <Select
            style={{ width: "100%" }}
            value={value[index]?.modifyRule}
            options={isAdminOnly ? modifyRuleAdminOnlyOptions : modifyRuleSelfOptions}
            onChange={(val) => updateField(index, "modifyRule", val)}
            size="small"
          />
        );
      },
    },
    {
      title: tr("organization.accountItemTable.columns.tab"),
      dataIndex: "tab",
      key: "tab",
      width: 130,
      render: (_: string, __: AccountItem, index: number) => (
        <Input
          value={value[index]?.tab}
          onChange={(e) => updateField(index, "tab", e.target.value)}
          placeholder={tr("organization.accountItemTable.tabPlaceholder")}
          size="small"
        />
      ),
    },
    {
      title: tr("organization.accountItemTable.columns.regex"),
      dataIndex: "regex",
      key: "regex",
      width: 180,
      render: (_: string, record: AccountItem, index: number) => {
        if (!REGEX_ENABLED_NAMES.has(record.name)) return null;
        return (
          <Input
            value={value[index]?.regex}
            onChange={(e) => updateField(index, "regex", e.target.value)}
            placeholder={tr("organization.accountItemTable.regexPlaceholder")}
            size="small"
          />
        );
      },
    },
    {
      title: tr("organization.accountItemTable.columns.action"),
      key: "action",
      width: 120,
      render: (_: unknown, __: AccountItem, index: number) => (
        <Space size={4}>
          <Tooltip title={tr("organization.accountItemTable.moveUp")}>
            <Button
              icon={<UpOutlined />}
              size="small"
              disabled={index === 0}
              onClick={() => handleMoveUp(index)}
            />
          </Tooltip>
          <Tooltip title={tr("organization.accountItemTable.moveDown")}>
            <Button
              icon={<DownOutlined />}
              size="small"
              disabled={index === value.length - 1}
              onClick={() => handleMoveDown(index)}
            />
          </Tooltip>
          <Tooltip title={tr("organization.accountItemTable.delete")}>
            <Button
              icon={<DeleteOutlined />}
              size="small"
              danger
              onClick={() => handleRemove(index)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <Table
      rowKey={(record) => record.name}
      columns={columns}
      dataSource={value}
      size="small"
      bordered
      pagination={false}
      scroll={{ x: "max-content" }}
      title={() => (
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span>{tr("organization.accountItemTable.title")}</span>
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleAdd}>
            {tr("organization.accountItemTable.add")}
          </Button>
        </div>
      )}
    />
  );
};

export default AccountItemTable;

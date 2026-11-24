import { Button, Input, Select, Space, Table, Tooltip } from "antd";
import { DeleteOutlined, DownOutlined, PlusOutlined, UpOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

import { useTranslation } from "react-i18next";
export type SigninMethodValue = {
  name: string;
  displayName: string;
  rule: string;
};

const SIGNIN_METHOD_OPTIONS = [
  { name: "Password", displayName: "Password" },
  { name: "Verification code", displayName: "Verification code" },
  { name: "WebAuthn", displayName: "WebAuthn" },
  { name: "LDAP", displayName: "LDAP" },
  { name: "Face ID", displayName: "Face ID" },
  { name: "Device login", displayName: "Device login" },
  { name: "WeChat", displayName: "WeChat" },
  { name: "DingTalk", displayName: "DingTalk" },
  { name: "Lark", displayName: "Lark" },
  { name: "CAS", displayName: "CAS" },
];

function getRuleOptions(methodName: string): Array<{ label: string; value: string }> {
  switch (methodName) {
    case "Password":
      return [
        { label: "All", value: "All" },
        { label: "Non-LDAP", value: "Non-LDAP" },
        { label: "Hide password", value: "Hide password" },
      ];
    case "Verification code":
      return [
        { label: "All", value: "All" },
        { label: "Email only", value: "Email only" },
        { label: "Phone only", value: "Phone only" },
      ];
    case "WeChat":
    case "Device login":
      return [
        { label: "Tab", value: "Tab" },
        { label: "Login page", value: "Login page" },
      ];
    default:
      return [];
  }
}

// oxlint-disable-next-line react/only-export-components -- getDefaultRule 也被 AuthenticationTab 引用，无法在本文件内合并进组件
export function getDefaultRule(methodName: string): string {
  switch (methodName) {
    case "Password":
    case "Verification code":
      return "All";
    case "WeChat":
    case "Device login":
      return "Tab";
    default:
      return "None";
  }
}

type Props = {
  value: SigninMethodValue[];
  onChange: (value: SigninMethodValue[]) => void;
  readOnly?: boolean;
};

const SigninMethodTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t } = useTranslation("common");
  const handleAdd = useCallback(() => {
    onChange([...value, { name: "", displayName: "", rule: "None" }]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  const handleChange = useCallback(
    (index: number, field: keyof SigninMethodValue, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const handleMethodChange = useCallback(
    (index: number, methodName: string) => {
      const next = [...value];
      next[index] = {
        ...next[index],
        name: methodName,
        displayName: methodName,
        rule: getDefaultRule(methodName),
      };
      onChange(next);
    },
    [value, onChange],
  );

  const handleSwap = useCallback(
    (i: number, j: number) => {
      if (j < 0 || j >= value.length) return;
      const next = [...value];
      [next[i], next[j]] = [next[j], next[i]];
      onChange(next);
    },
    [value, onChange],
  );

  // Get available methods that haven't been selected yet
  const usedNames = new Set(value.map((v) => v.name));
  const availableMethods = SIGNIN_METHOD_OPTIONS.filter((m) => !usedNames.has(m.name));

  const columns = [
    {
      title: "Name",
      dataIndex: "name",
      width: "30%",
      render: (_: string, __: SigninMethodValue, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Select
            value={value[index]?.name || undefined}
            onChange={(v) => handleMethodChange(index, v)}
            placeholder={t("selectSigninMethod")}
            size="small"
            style={{ width: "100%" }}
            showSearch
            options={SIGNIN_METHOD_OPTIONS.map((m) => ({
              label: m.displayName,
              value: m.name,
              disabled: usedNames.has(m.name) && value[index]?.name !== m.name,
            }))}
          />
        ),
    },
    {
      title: "Display Name",
      dataIndex: "displayName",
      width: "30%",
      render: (_: string, __: SigninMethodValue, index: number) =>
        readOnly ? (
          value[index]?.displayName || "-"
        ) : (
          <Input
            value={value[index]?.displayName}
            onChange={(e) => handleChange(index, "displayName", e.target.value)}
            placeholder="Display name"
            size="small"
          />
        ),
    },
    {
      title: "Rule",
      dataIndex: "rule",
      width: "25%",
      render: (_: string, __: SigninMethodValue, index: number) => {
        const options = getRuleOptions(value[index]?.name ?? "");
        if (options.length === 0 || readOnly) {
          return readOnly ? (value[index]?.rule === "None" ? "-" : value[index]?.rule) : null;
        }
        return (
          <Select
            value={value[index]?.rule || undefined}
            onChange={(v) => handleChange(index, "rule", v)}
            options={options}
            size="small"
            style={{ width: "100%" }}
          />
        );
      },
    },
    {
      title: "",
      width: "15%",
      render: (_: unknown, __: SigninMethodValue, index: number) =>
        readOnly ? null : (
          <Space size={4}>
            <Tooltip title="Move up">
              <Button
                size="small"
                icon={<UpOutlined />}
                disabled={index === 0}
                onClick={() => handleSwap(index, index - 1)}
              />
            </Tooltip>
            <Tooltip title="Move down">
              <Button
                size="small"
                icon={<DownOutlined />}
                disabled={index === value.length - 1}
                onClick={() => handleSwap(index, index + 1)}
              />
            </Tooltip>
            <Tooltip title="Delete">
              <Button
                size="small"
                danger
                icon={<DeleteOutlined />}
                disabled={value.length <= 1}
                onClick={() => handleRemove(index)}
              />
            </Tooltip>
          </Space>
        ),
    },
  ];

  return (
    <div>
      <Table
        columns={columns}
        dataSource={value.map((item, i) => ({ ...item, key: i }))}
        pagination={false}
        size="small"
        bordered
      />
      {!readOnly && (
        <Space style={{ marginTop: 8 }}>
          <Button
            type="dashed"
            onClick={handleAdd}
            icon={<PlusOutlined />}
            size="small"
            disabled={availableMethods.length === 0}
          >
            Add Signin Method
          </Button>
        </Space>
      )}
    </div>
  );
};

export default SigninMethodTable;
export { SigninMethodTable };

import { Button, Input, Select, Space, Switch, Table, Typography } from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { type FC, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  getSignupItemDefaultCss,
  getSignupItemRuleOptions,
  getSignupItemType,
  SIGNUP_ITEM_NAMES,
} from "@/shared/lib/signin-default-css";

export type SignupItemValue = {
  name: string;
  visible: boolean;
  required: boolean;
  prompted: boolean;
  rule?: string;
  regex?: string;
  options?: string[];
  label?: string;
  placeholder?: string;
  type?: string;
  customCss?: string;
};

const ITEM_TYPE_OPTIONS = [
  { value: "text", label: "Text" },
  { value: "password", label: "Password" },
  { value: "email", label: "Email" },
  { value: "phone", label: "Phone" },
  { value: "select", label: "Select" },
  { value: "textarea", label: "Textarea" },
  { value: "checkbox", label: "Checkbox" },
  { value: "hidden", label: "Hidden" },
];

type Props = {
  value: SignupItemValue[];
  onChange: (value: SignupItemValue[]) => void;
  readOnly?: boolean;
};

const SignupItemsEditor: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t: tr } = useTranslation("resource");

  const handleAdd = useCallback(() => {
    onChange([
      ...value,
      {
        name: "",
        visible: true,
        required: false,
        prompted: false,
        rule: "",
        label: "",
        placeholder: "",
        type: "text",
      },
    ]);
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
    (index: number, field: keyof SignupItemValue, val: string | boolean | string[]) => {
      const next = [...value];
      const updated = { ...next[index], [field]: val };
      // 选择注册项名称时，若 customCss 为空则自动填入默认值，并同步推荐 type。
      if (field === "name") {
        const name = String(val ?? "");
        if (!updated.customCss) {
          const defaultCss = getSignupItemDefaultCss(name);
          if (defaultCss) updated.customCss = defaultCss;
        }
        if (!updated.type) updated.type = getSignupItemType(name);
      }
      next[index] = updated;
      onChange(next);
    },
    [value, onChange],
  );

  const handleMove = useCallback(
    (index: number, delta: -1 | 1) => {
      const target = index + delta;
      if (target < 0 || target >= value.length) return;
      const next = [...value];
      [next[index], next[target]] = [next[target], next[index]];
      onChange(next);
    },
    [value, onChange],
  );

  const usedNames = useMemo(() => new Set(value.map((item) => item.name).filter(Boolean)), [value]);
  const validation = useMemo(() => {
    const seen = new Set<string>();
    let duplicate = "";
    let missing = false;
    for (const item of value) {
      const name = (item.name ?? "").trim();
      if (!name) {
        missing = true;
        continue;
      }
      if (seen.has(name)) duplicate = duplicate || name;
      seen.add(name);
    }
    return { missing, duplicate };
  }, [value]);

  const yes = tr("application.signupItems.yes");
  const no = tr("application.signupItems.no");

  const columns = [
    ...(readOnly
      ? []
      : [
          {
            title: tr("application.signupItems.order"),
            width: 64,
            render: (_: unknown, __: SignupItemValue, index: number) => (
              <Space size={0}>
                <Button
                  type="text"
                  size="small"
                  aria-label={tr("application.signupItems.moveUp")}
                  disabled={index === 0}
                  icon={<ArrowUpOutlined />}
                  onClick={() => handleMove(index, -1)}
                />
                <Button
                  type="text"
                  size="small"
                  aria-label={tr("application.signupItems.moveDown")}
                  disabled={index === value.length - 1}
                  icon={<ArrowDownOutlined />}
                  onClick={() => handleMove(index, 1)}
                />
              </Space>
            ),
          },
        ]),
    {
      title: tr("application.signupItems.name"),
      dataIndex: "name",
      width: 150,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.name
        ) : (
          <Select
            value={value[index]?.name}
            onChange={(val) => handleChange(index, "name", val)}
            placeholder={tr("application.signupItems.placeholder.name")}
            status={value[index]?.name ? undefined : "error"}
            size="small"
            style={{ width: "100%" }}
            showSearch
            options={SIGNUP_ITEM_NAMES.filter(
              (n) => n === value[index]?.name || !usedNames.has(n),
            ).map((n) => ({ label: n, value: n }))}
          />
        ),
    },
    {
      title: tr("application.signupItems.label"),
      dataIndex: "label",
      width: 120,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.label || "-"
        ) : (
          <Input
            value={value[index]?.label}
            onChange={(e) => handleChange(index, "label", e.target.value)}
            placeholder={tr("application.signupItems.placeholder.label")}
            size="small"
          />
        ),
    },
    {
      title: tr("application.signupItems.visible"),
      dataIndex: "visible",
      width: 70,
      render: (_: boolean, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.visible ? (
            yes
          ) : (
            no
          )
        ) : (
          <Switch
            size="small"
            checked={value[index]?.visible}
            onChange={(val) => handleChange(index, "visible", val)}
          />
        ),
    },
    {
      title: tr("application.signupItems.required"),
      dataIndex: "required",
      width: 70,
      render: (_: boolean, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.required ? (
            yes
          ) : (
            no
          )
        ) : (
          <Switch
            size="small"
            checked={value[index]?.required}
            onChange={(val) => handleChange(index, "required", val)}
          />
        ),
    },
    {
      title: tr("application.signupItems.prompted"),
      dataIndex: "prompted",
      width: 70,
      render: (_: boolean, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.prompted ? (
            yes
          ) : (
            no
          )
        ) : (
          <Switch
            size="small"
            checked={value[index]?.prompted}
            onChange={(val) => handleChange(index, "prompted", val)}
          />
        ),
    },
    {
      title: tr("application.signupItems.rule"),
      dataIndex: "rule",
      width: 130,
      render: (_: string, __: SignupItemValue, index: number) => {
        const item = value[index];
        const options = getSignupItemRuleOptions(item?.name ?? "");
        return readOnly ? (
          item?.rule || "-"
        ) : (
          <Select
            value={item?.rule || undefined}
            onChange={(val) => handleChange(index, "rule", val)}
            options={options}
            size="small"
            style={{ width: "100%" }}
            placeholder="None"
            allowClear
          />
        );
      },
    },
    {
      title: tr("application.signupItems.type", "Type"),
      dataIndex: "type",
      width: 100,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.type || "-"
        ) : (
          <Select
            value={value[index]?.type || "text"}
            onChange={(val) => handleChange(index, "type", val)}
            options={ITEM_TYPE_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Options",
      dataIndex: "options",
      width: 140,
      render: (_: string[] | undefined, __: SignupItemValue, index: number) => {
        const item = value[index];
        if (item?.type !== "select") return <span style={{ color: "rgba(0,0,0,0.25)" }}>-</span>;
        return readOnly ? (
          (item?.options ?? []).join(", ") || "-"
        ) : (
          <Select
            mode="tags"
            value={item?.options ?? []}
            onChange={(val) => handleChange(index, "options", val)}
            placeholder="Add options"
            size="small"
            style={{ width: "100%" }}
            tokenSeparators={[","]}
          />
        );
      },
    },
    {
      title: "Regex",
      dataIndex: "regex",
      width: 120,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.regex || "-"
        ) : (
          <Input
            value={value[index]?.regex}
            onChange={(e) => handleChange(index, "regex", e.target.value)}
            placeholder="regex"
            size="small"
          />
        ),
    },
    {
      title: tr("application.signupItems.placeholder.title"),
      dataIndex: "placeholder",
      width: 120,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.placeholder || "-"
        ) : (
          <Input
            value={value[index]?.placeholder}
            onChange={(e) => handleChange(index, "placeholder", e.target.value)}
            placeholder={tr("application.signupItems.placeholder.placeholder")}
            size="small"
          />
        ),
    },
    {
      title: tr("application.signupItems.customCss"),
      dataIndex: "customCss",
      width: 200,
      render: (_: string, __: SignupItemValue, index: number) =>
        readOnly ? (
          value[index]?.customCss || "-"
        ) : (
          <Input.TextArea
            value={value[index]?.customCss}
            onChange={(e) => handleChange(index, "customCss", e.target.value)}
            placeholder={tr("application.signupItems.placeholder.customCss")}
            size="small"
            autoSize={{ minRows: 1, maxRows: 2 }}
          />
        ),
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 40,
            render: (_: unknown, __: SignupItemValue, index: number) => (
              <Button
                type="text"
                danger
                size="small"
                aria-label={tr("application.signupItems.remove", {
                  name: value[index]?.name || index + 1,
                })}
                icon={<DeleteOutlined />}
                onClick={() => handleRemove(index)}
              />
            ),
          },
        ]),
  ];

  return (
    <Space orientation="vertical" style={{ width: "100%" }}>
      <Table
        dataSource={value.map((item, i) => ({ ...item, _idx: i }))}
        columns={columns}
        pagination={false}
        size="small"
        rowKey="_idx"
        scroll={{ x: "max-content" }}
        locale={{
          emptyText: readOnly
            ? tr("application.signupItems.empty")
            : tr("application.signupItems.emptyEditable"),
        }}
      />
      {!readOnly && (validation.missing || validation.duplicate) && (
        <div role="alert" aria-live="polite">
          {validation.missing && (
            <Typography.Text type="danger">
              {tr("application.signupItems.validation.missing")}
            </Typography.Text>
          )}
          {validation.duplicate && (
            <Typography.Text type="danger">
              {tr("application.signupItems.validation.duplicate", {
                name: validation.duplicate,
              })}
            </Typography.Text>
          )}
        </div>
      )}
      {!readOnly && (
        <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
          {tr("application.signupItems.add")}
        </Button>
      )}
    </Space>
  );
};

export default SignupItemsEditor;
export { SignupItemsEditor };

import { useCallback, useMemo } from "react";
import { Button, Input, Select, Space, Switch, Table } from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import {
  SIGNIN_ITEM_NAMES,
  getSigninItemDefaultCss,
  getSigninItemRuleOptions,
} from "@/shared/lib/signin-default-css";
import { isCustomSigninItem } from "@/shared/lib/auth-signin-items";

type SigninItem = {
  name: string;
  label?: string;
  placeholder?: string;
  type?: string;
  visible?: boolean;
  required?: boolean;
  rule?: string;
  isCustom?: boolean;
  customCss?: string;
};

type SigninItemsEditorProps = {
  value?: SigninItem[];
  onChange?: (items: SigninItem[]) => void;
};

export default function SigninItemsEditor({ value = [], onChange }: SigninItemsEditorProps) {
  const { t } = useTranslation();
  const { t: tc } = useTranslation("common");

  const usedNames = useMemo(() => new Set(value.map((item) => item.name).filter(Boolean)), [value]);

  const handleAdd = useCallback(() => {
    onChange?.([...value, { name: "", label: "", type: "text", visible: true, required: false }]);
  }, [value, onChange]);

  const handleAddCustom = useCallback(() => {
    const idx = value.filter((item) => isCustomSigninItem(item)).length + 1;
    onChange?.([
      ...value,
      { name: `Text ${idx}`, isCustom: true, visible: true, label: "", customCss: "" },
    ]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange?.(next);
    },
    [value, onChange],
  );

  const handleMoveUp = useCallback(
    (index: number) => {
      if (index === 0) return;
      const next = [...value];
      [next[index - 1], next[index]] = [next[index], next[index - 1]];
      onChange?.(next);
    },
    [value, onChange],
  );

  const handleMoveDown = useCallback(
    (index: number) => {
      if (index === value.length - 1) return;
      const next = [...value];
      [next[index], next[index + 1]] = [next[index + 1], next[index]];
      onChange?.(next);
    },
    [value, onChange],
  );

  const handleFieldChange = useCallback(
    (index: number, field: keyof SigninItem, fieldValue: unknown) => {
      const next = [...value];
      const updated = { ...next[index], [field]: fieldValue };
      // 选择登录项名称时，若 customCss 为空则自动填入默认值（对齐真实登录页 DOM 结构）。
      if (field === "name" && !updated.customCss) {
        const defaultCss = getSigninItemDefaultCss(String(fieldValue ?? ""));
        if (defaultCss) updated.customCss = defaultCss;
      }
      next[index] = updated;
      onChange?.(next);
    },
    [value, onChange],
  );

  const nameOptions = useMemo(
    () => SIGNIN_ITEM_NAMES.filter((n) => !usedNames.has(n)).map((n) => ({ label: n, value: n })),
    [usedNames],
  );

  const columns = [
    {
      title: t("common:name", "Name"),
      dataIndex: "name",
      width: 160,
      fixed: "left" as const,
      render: (_: unknown, __: unknown, index: number) => {
        const item = value[index];
        const custom = isCustomSigninItem(item);
        return custom ? (
          <Input value={item?.name} size="small" disabled />
        ) : (
          <Select
            value={item?.name || undefined}
            onChange={(v) => handleFieldChange(index, "name", v)}
            placeholder="Select signin item"
            size="small"
            style={{ width: "100%" }}
            showSearch
            options={
              item?.name ? [{ label: item.name, value: item.name }, ...nameOptions] : nameOptions
            }
          />
        );
      },
    },
    {
      title: "Visible",
      dataIndex: "visible",
      width: 80,
      render: (_: unknown, __: unknown, index: number) => (
        <Switch
          size="small"
          checked={value[index]?.visible ?? true}
          onChange={(v) => handleFieldChange(index, "visible", v)}
        />
      ),
    },
    {
      title: tc("label", "Label"),
      dataIndex: "label",
      width: 130,
      render: (_: unknown, __: unknown, index: number) => (
        <Input
          value={value[index]?.label}
          onChange={(e) => handleFieldChange(index, "label", e.target.value)}
          placeholder="Display Label"
          size="small"
        />
      ),
    },
    {
      title: "Placeholder",
      dataIndex: "placeholder",
      width: 130,
      render: (_: unknown, __: unknown, index: number) => (
        <Input
          value={value[index]?.placeholder}
          onChange={(e) => handleFieldChange(index, "placeholder", e.target.value)}
          placeholder="placeholder"
          size="small"
        />
      ),
    },
    {
      title: "Rule",
      dataIndex: "rule",
      width: 150,
      render: (_: unknown, __: unknown, index: number) => {
        const item = value[index];
        const options = getSigninItemRuleOptions(item?.name ?? "");
        if (options.length === 0) return <span style={{ color: "rgba(0,0,0,0.25)" }}>-</span>;
        return (
          <Select
            value={item?.rule || undefined}
            onChange={(v) => handleFieldChange(index, "rule", v)}
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
      title: t("common:customCss", "Custom CSS"),
      dataIndex: "customCss",
      width: 220,
      render: (_: unknown, __: unknown, index: number) => (
        <Input.TextArea
          value={value[index]?.customCss}
          onChange={(e) => handleFieldChange(index, "customCss", e.target.value)}
          placeholder={
            isCustomSigninItem(value[index])
              ? "HTML content for this item"
              : "CSS selector for this field"
          }
          size="small"
          autoSize={{ minRows: 1, maxRows: 2 }}
        />
      ),
    },
    {
      title: t("common:actions", "Actions"),
      width: 120,
      fixed: "right" as const,
      render: (_: unknown, __: unknown, index: number) => (
        <Space size={4}>
          <Button
            icon={<ArrowUpOutlined />}
            size="small"
            disabled={index === 0}
            onClick={() => handleMoveUp(index)}
          />
          <Button
            icon={<ArrowDownOutlined />}
            size="small"
            disabled={index === value.length - 1}
            onClick={() => handleMoveDown(index)}
          />
          <Button
            icon={<DeleteOutlined />}
            size="small"
            danger
            onClick={() => handleRemove(index)}
          />
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Table
        dataSource={value.map((item, idx) => ({ ...item, key: idx }))}
        columns={columns}
        pagination={false}
        size="small"
        bordered
        scroll={{ x: "max-content" }}
      />
      <Space style={{ marginTop: 8, width: "100%" }}>
        <Button type="dashed" icon={<PlusOutlined />} onClick={handleAdd} size="small">
          {tc("addItem", "Add Item")}
        </Button>
        <Button type="dashed" onClick={handleAddCustom} size="small">
          + Custom item
        </Button>
      </Space>
    </div>
  );
}
